package sync

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"explorer-server/database"
	"explorer-server/database/operations"
)

// Limits drawn from the wire spec.
const (
	// MaxTokensPerRequest is the server-enforced cap on token_ids length.
	MaxTokensPerRequest = 50

	// DefaultPageSize matches the server default; safe across runs.
	DefaultPageSize = 100

	// MaxPageSize is the server-enforced ceiling.
	MaxPageSize = 1000
)

// Config drives a sync run.
type Config struct {
	// Network selects the bucket in fullnodes.json (mainnet / testnet).
	Network string

	// PeersURL is the registry URL; empty → DefaultFullnodePeersURL.
	PeersURL string

	// PageSize must stay constant for the whole run so total_pages remains
	// stable. Zero → DefaultPageSize. Clamped to (0, MaxPageSize].
	PageSize int

	// LocalBridgePortStart and LocalBridgePortCount delimit the TCP port pool
	// the PeerManager allocates from. One pool entry is held per concurrent
	// PeerConn; the service uses exactly one connection per run, so a pool
	// of 4 is generous.
	LocalBridgePortStart int
	LocalBridgePortCount int

	// HTTPTimeout is the per-request ceiling on a single SendJSONRequest.
	// Page fetches over libp2p are typically fast (<1s) but we leave room
	// for the first request of a run (swarm connect + forward + page fetch).
	HTTPTimeout time.Duration

	// TokenLimit caps how many token IDs a single RunOnce will sync. Zero
	// means no limit. Useful in production to throttle initial bootstrap.
	TokenLimit int
}

func (c Config) effectivePageSize() int {
	if c.PageSize <= 0 {
		return DefaultPageSize
	}
	if c.PageSize > MaxPageSize {
		return MaxPageSize
	}
	return c.PageSize
}

// Service orchestrates one sync run end-to-end.
type Service struct {
	cfg         Config
	peerSource  *PeerSource
	peerManager *PeerManager
}

// NewService builds a Service. The IPFSForwarder is the live transport
// (production: NewShellAdapter("localhost:5001")). Pass a fake in tests.
func NewService(cfg Config, ipfs IPFSForwarder) *Service {
	if cfg.LocalBridgePortStart <= 0 {
		cfg.LocalBridgePortStart = 7200
	}
	if cfg.LocalBridgePortCount <= 0 {
		cfg.LocalBridgePortCount = 4
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 110 * time.Second
	}
	pm := NewPeerManager(ipfs, cfg.LocalBridgePortStart, cfg.LocalBridgePortCount, cfg.HTTPTimeout)
	ps := NewPeerSource(cfg.PeersURL, cfg.Network)
	return &Service{cfg: cfg, peerSource: ps, peerManager: pm}
}

// RunResult is a structured summary of one sync run. Returned by RunOnce
// and logged by the scheduler / one-shot CLI handler.
type RunResult struct {
	PeerID            string
	TokensScanned     int
	BatchesProcessed  int
	BatchesFailed     int
	PagesFetched      int
	DivergentTokens   []string
	// ChainBrokenTokens are tokens whose response failed structural
	// validation (previous_transaction_id mismatch or position gap). They
	// were flagged with Tokens.needs_sync = true and will be revisited on
	// the next scheduled run. Already-inserted rows are NOT rolled back —
	// the partial chain remains in SyncTokenChain for forensic comparison.
	ChainBrokenTokens []string
	Duration          time.Duration
	StartedAt         time.Time
	FinishedAt        time.Time
}

// RunOnce executes one sync cycle:
//
//  1. Resolve the pinned fullnode peer for this run.
//  2. Load the explorer's token list, chunk into batches of 50.
//  3. Open one libp2p connection to the fullnode and reuse it.
//  4. For each batch: build known_positions, paginate page 1..total_pages,
//     re-fetch any gaps once, persist entries, purge+replace divergent tokens.
//  5. Close the connection.
//
// A peer-dial or HTTP failure aborts the current run — we do not fall over
// to a different peer mid-run because different fullnodes can have slightly
// divergent state and mixing pages from two peers would corrupt the mirror.
// The next scheduled run picks a fresh peer.
func (s *Service) RunOnce(ctx context.Context) RunResult {
	res := RunResult{StartedAt: time.Now()}
	defer func() {
		res.FinishedAt = time.Now()
		res.Duration = res.FinishedAt.Sub(res.StartedAt)
	}()

	peerID, err := s.peerSource.PickActivePeer(ctx)
	if err != nil {
		log.Printf("[Sync] pick peer: %v — aborting run", err)
		return res
	}
	res.PeerID = peerID
	log.Printf("[Sync] run pinned to peer %s", peerID)

	tokens, err := operations.ListAllTokenIDs(database.ReadDB)
	if err != nil {
		log.Printf("[Sync] list tokens: %v — aborting run", err)
		return res
	}
	if s.cfg.TokenLimit > 0 && len(tokens) > s.cfg.TokenLimit {
		log.Printf("[Sync] token limit %d applied; %d of %d tokens this run",
			s.cfg.TokenLimit, s.cfg.TokenLimit, len(tokens))
		tokens = tokens[:s.cfg.TokenLimit]
	}
	res.TokensScanned = len(tokens)
	if len(tokens) == 0 {
		log.Printf("[Sync] no tokens to sync — finishing run")
		return res
	}

	conn, err := s.peerManager.OpenPeerConn(ctx, peerID, SyncAppName)
	if err != nil {
		log.Printf("[Sync] open peer conn: %v — aborting run", err)
		return res
	}
	defer conn.Close()

	batches := chunkTokenIDs(tokens, MaxTokensPerRequest)
	log.Printf("[Sync] %d batch(es) of up to %d tokens (page_size=%d)",
		len(batches), MaxTokensPerRequest, s.cfg.effectivePageSize())

	divergentSet := make(map[string]struct{})
	brokenSet := make(map[string]struct{})
	for i, batch := range batches {
		if ctx.Err() != nil {
			log.Printf("[Sync] context cancelled — aborting at batch %d/%d", i+1, len(batches))
			break
		}
		pageHits, divergent, broken, err := s.runBatch(ctx, conn, batch, i+1, len(batches))
		if err != nil {
			res.BatchesFailed++
			log.Printf("[Sync] batch %d/%d failed: %v", i+1, len(batches), err)
			// One batch failing doesn't poison the run — move on.
			continue
		}
		res.BatchesProcessed++
		res.PagesFetched += pageHits
		for _, tok := range divergent {
			divergentSet[tok] = struct{}{}
		}
		for _, tok := range broken {
			brokenSet[tok] = struct{}{}
		}
	}

	for tok := range divergentSet {
		res.DivergentTokens = append(res.DivergentTokens, tok)
	}
	for tok := range brokenSet {
		res.ChainBrokenTokens = append(res.ChainBrokenTokens, tok)
	}
	return res
}

// runBatch handles one batch of up to 50 tokens through to completion:
//
//  1. Capture known_positions from local SyncTokenChain (sent verbatim on
//     every page request — must NOT change mid-batch, the server's
//     total_pages depends on it).
//  2. Fetch page 1 (carries total_pages + total_items), then pages 2..N.
//  3. Buffer all entries in memory as pages arrive. Don't write to the DB
//     yet — wire arrival order doesn't necessarily match chain order
//     (gap-audit retries can arrive out of sequence).
//  4. Gap-audit: re-fetch any missing pages once.
//  5. After all fetches, applyBuffered sorts each token's accumulated
//     entries by position, runs the structural validator against the
//     frozen known head (or "" if new), and inserts via
//     InsertSyncChainEntry. A break flags Tokens.needs_sync and stops
//     inserts for that token; already-inserted rows for that token (from
//     before the break in this same batch) stay.
//
// Returns (pages fetched, divergent token IDs, chain-broken token IDs, err).
func (s *Service) runBatch(
	ctx context.Context,
	conn *PeerConn,
	batch []string,
	batchIdx, batchTotal int,
) (int, []string, []string, error) {
	frozenKnown, err := operations.GetKnownPositions(database.ReadDB, batch)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("known positions: %w", err)
	}

	pageSize := s.cfg.effectivePageSize()
	gotPages := make(map[int]struct{})
	perToken := make(map[string][]ChainEntry)
	divergent := make(map[string]struct{})

	// Page 1.
	resp, err := s.fetchPage(ctx, conn, batch, frozenKnown, 1, pageSize)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("page 1: %w", err)
	}
	if resp.Result.TotalItems == 0 {
		log.Printf("[Sync] batch %d/%d empty (total_items=0)", batchIdx, batchTotal)
		return 1, nil, nil, nil
	}
	s.collectPage(&resp.Result, perToken, divergent)
	gotPages[1] = struct{}{}
	totalPages := resp.Result.TotalPages
	log.Printf("[Sync] batch %d/%d page 1/%d collected (items=%d)",
		batchIdx, batchTotal, totalPages, resp.Result.TotalItems)

	// Pages 2..total_pages.
	for n := 2; n <= totalPages; n++ {
		if ctx.Err() != nil {
			break
		}
		resp, err := s.fetchPage(ctx, conn, batch, frozenKnown, n, pageSize)
		if err != nil {
			log.Printf("[Sync] batch %d/%d page %d/%d failed: %v (will retry in gap audit)",
				batchIdx, batchTotal, n, totalPages, err)
			continue
		}
		s.collectPage(&resp.Result, perToken, divergent)
		gotPages[n] = struct{}{}
	}

	// Gap audit: re-fetch each missing page once. Still-missing pages
	// defer to the next scheduled run; we accept that some chains will be
	// incomplete and may trigger the validator on the partial data.
	missing := missingPages(totalPages, gotPages)
	for _, n := range missing {
		if ctx.Err() != nil {
			break
		}
		log.Printf("[Sync] batch %d/%d gap re-fetch page %d", batchIdx, batchTotal, n)
		resp, err := s.fetchPage(ctx, conn, batch, frozenKnown, n, pageSize)
		if err != nil {
			log.Printf("[Sync] batch %d/%d page %d re-fetch failed: %v (deferring to next run)",
				batchIdx, batchTotal, n, err)
			continue
		}
		s.collectPage(&resp.Result, perToken, divergent)
		gotPages[n] = struct{}{}
	}

	// Apply: validate + insert + flag.
	flagged := s.applyBuffered(ctx, perToken, divergent, frozenKnown)

	var divergentList []string
	for tok := range divergent {
		divergentList = append(divergentList, tok)
	}
	return len(gotPages), divergentList, collectFlagged(flagged), nil
}

// collectPage appends a page's entries into the per-token buffer and
// records any divergent tokens. No DB writes yet — applyBuffered runs at
// the end of the batch with everything sorted.
func (s *Service) collectPage(
	page *SyncTxnInfoChainResult,
	perToken map[string][]ChainEntry,
	divergent map[string]struct{},
) {
	for _, tok := range page.DivergentTokens {
		divergent[tok] = struct{}{}
	}
	for tokenID, entries := range page.Data {
		perToken[tokenID] = append(perToken[tokenID], entries...)
	}
}

// collectFlagged returns the keys of the flagged set as a sorted-undefined
// slice, suitable for surfacing in RunResult.ChainBrokenTokens.
func collectFlagged(flagged map[string]struct{}) []string {
	if len(flagged) == 0 {
		return nil
	}
	out := make([]string, 0, len(flagged))
	for tok := range flagged {
		out = append(out, tok)
	}
	return out
}

// fetchPage issues one page request and returns the decoded envelope.
// Surfaces non-2xx and Status:false explicitly so the caller's logs are
// useful when something does go wrong.
func (s *Service) fetchPage(
	ctx context.Context,
	conn *PeerConn,
	tokenIDs []string,
	known map[string]operations.KnownPosition,
	pageNumber, pageSize int,
) (*SyncTxnInfoChainResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	req := SyncTxnInfoChainRequest{
		TokenIDs:       tokenIDs,
		KnownPositions: toWireKnown(known),
		PageNumber:     pageNumber,
		PageSize:       pageSize,
	}
	var resp SyncTxnInfoChainResponse
	err := conn.SendJSONRequest("POST", "/rubix/v1/fullnode/sync-txn-info-chain", nil, req, &resp, false)
	if err != nil {
		return nil, err
	}
	if !resp.Status {
		return nil, fmt.Errorf("fullnode status=false: %s", resp.Message)
	}
	if resp.Result.Data == nil {
		resp.Result.Data = map[string][]ChainEntry{}
	}
	return &resp, nil
}

// applyBuffered runs at the end of a batch, once all page fetches (including
// gap-audit retries) have completed. For each token it:
//
//  1. If the token was in any page's divergent_tokens list, purges all
//     local rows for that token (the server's about-to-be-applied entries
//     replace the prior local view).
//  2. Sorts the accumulated entries by position. Server is supposed to
//     emit position-ascending per-token, but gap-audit retries can land
//     pages out of wire order — sorting here makes the validator
//     independent of arrival order.
//  3. Walks the sorted entries in order, validating chain linkage against
//     the running head:
//       - if the token was divergent, or wasn't in frozenKnown (new token),
//         the first entry must have previous_transaction_id == "" (genesis).
//         Its position can be anything the chain starts at — we don't
//         hardcode 0 vs 1.
//       - otherwise, the first entry must continue from frozenKnown: its
//         position must equal head.position + 1, and its previous_transaction_id
//         must equal head.transaction_id.
//       - subsequent entries: same shape, with head advancing to the last
//         successfully-validated entry's (position, id).
//  4. On any mismatch, logs the break, sets Tokens.needs_sync = true,
//     marks the token in `flagged`, and stops processing further entries
//     for that token. Already-inserted entries (those before the break)
//     stay in SyncTokenChain — explicitly per the design instruction, so
//     the partial chain is queryable for forensics and the next cycle has
//     a head to validate continuations against.
//
// Other tokens are unaffected by one token's break.
func (s *Service) applyBuffered(
	ctx context.Context,
	perToken map[string][]ChainEntry,
	divergent map[string]struct{},
	frozenKnown map[string]operations.KnownPosition,
) map[string]struct{} {
	flagged := make(map[string]struct{})

	// 1. Purge divergent tokens up front. After purge, the validator
	//    treats them like new tokens (no head → expect genesis prev).
	for tok := range divergent {
		if err := operations.PurgeSyncChainForToken(database.WriteDB, tok); err != nil {
			log.Printf("[Sync] purge divergent token %s: %v", tok, err)
		}
	}

	for tokenID, entries := range perToken {
		if ctx.Err() != nil {
			return flagged
		}
		if len(entries) == 0 {
			continue
		}

		// 2. Sort by position ascending (defensive — server claims this,
		//    but gap-audit retries can interleave pages on the wire).
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].Position < entries[j].Position
		})

		// 3. Validator's starting head: post-purge for divergent tokens
		//    (treat as new), else from frozenKnown if the token had any
		//    prior local rows.
		var headTxnID string
		var headPos int64
		hasHead := false
		if _, div := divergent[tokenID]; !div {
			if k, ok := frozenKnown[tokenID]; ok {
				headTxnID = k.TransactionID
				headPos = k.Position
				hasHead = true
			}
		}

		for i := range entries {
			if ctx.Err() != nil {
				return flagged
			}
			e := &entries[i]

			var expectedPrev string
			var positionMustEqual int64
			positionConstrained := false
			if hasHead {
				expectedPrev = headTxnID
				positionMustEqual = headPos + 1
				positionConstrained = true
			}

			if positionConstrained && e.Position != positionMustEqual {
				log.Printf("[Sync] Token %s: position gap at %d (expected %d, prev_tx=%s) — marking needs_sync",
					tokenID, e.Position, positionMustEqual, e.PreviousTransactionID)
				s.flagToken(tokenID, flagged)
				break
			}
			if e.PreviousTransactionID != expectedPrev {
				log.Printf("[Sync] Token %s: chain break at position %d (expected prev=%q got %q) — marking needs_sync",
					tokenID, e.Position, expectedPrev, e.PreviousTransactionID)
				s.flagToken(tokenID, flagged)
				break
			}

			if err := operations.InsertSyncChainEntry(
				database.WriteDB, tokenID,
				e.Position, e.ID, e.PreviousTransactionID, e.Role, e.Info,
			); err != nil {
				log.Printf("[Sync] insert entry token=%s pos=%d: %v — marking needs_sync",
					tokenID, e.Position, err)
				s.flagToken(tokenID, flagged)
				break
			}

			headTxnID = e.ID
			headPos = e.Position
			hasHead = true
		}
	}
	return flagged
}

// flagToken marks tokenID as needs_sync in the Tokens table and adds it to
// the in-memory `flagged` set so later entries in this batch are skipped.
// Idempotent. DB-write errors are logged but not propagated: the in-memory
// set keeps this batch consistent even if the DB update transiently fails,
// and the next cycle will re-attempt the flag (and re-detect the break)
// anyway.
func (s *Service) flagToken(tokenID string, flagged map[string]struct{}) {
	if _, already := flagged[tokenID]; already {
		return
	}
	flagged[tokenID] = struct{}{}
	if err := operations.FlagTokenNeedsSync(database.WriteDB, tokenID); err != nil {
		log.Printf("[Sync] flag token %s needs_sync: %v", tokenID, err)
	}
}

// chunkTokenIDs splits ids into chunks of at most n. Dedupes and drops empty
// strings on the way (the spec requires both).
func chunkTokenIDs(ids []string, n int) [][]string {
	seen := make(map[string]struct{}, len(ids))
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil
	}
	if n <= 0 {
		n = MaxTokensPerRequest
	}
	var out [][]string
	for i := 0; i < len(clean); i += n {
		end := i + n
		if end > len(clean) {
			end = len(clean)
		}
		out = append(out, append([]string(nil), clean[i:end]...))
	}
	return out
}

// missingPages returns the page numbers in [1, total] not present in got,
// sorted ascending. Cheap given that total is bounded by total_pages, which
// is on the order of dozens for realistic batches.
func missingPages(total int, got map[int]struct{}) []int {
	var out []int
	for n := 1; n <= total; n++ {
		if _, ok := got[n]; !ok {
			out = append(out, n)
		}
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// toWireKnown converts the operations-layer KnownPosition map into the wire
// shape. Kept as a separate function so the operations package stays free of
// wire-format types.
func toWireKnown(in map[string]operations.KnownPosition) map[string]KnownPosition {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]KnownPosition, len(in))
	for k, v := range in {
		out[k] = KnownPosition{Position: v.Position, TransactionID: v.TransactionID}
	}
	return out
}

package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"explorer-server/database"
	"explorer-server/database/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- harness ----------

// setupSyncDB swaps in an in-memory sqlite for the global Read/Write DBs.
// Returns a cleanup hook to restore originals.
func setupSyncDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Composite-PK + DISTINCT ON in GetKnownPositions are Postgres-specific;
	// we substitute a simpler equivalent for sqlite by AutoMigrating the
	// models and using a different reader. The integration here exercises
	// the page loop and the InsertSyncChainEntry helper, which work fine
	// against sqlite.
	if err := db.AutoMigrate(
		&models.Token{},
		&models.SyncTransaction{},
		&models.SyncTokenChain{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	prevR, prevW := database.ReadDB, database.WriteDB
	database.ReadDB = db
	database.WriteDB = db
	return db, func() {
		database.ReadDB = prevR
		database.WriteDB = prevW
	}
}

// scriptedFullnode plays canned page responses keyed by page_number. Used
// to drive RunOnce through a deterministic sequence.
type scriptedFullnode struct {
	pages       map[int]SyncTxnInfoChainResult // keyed by page_number
	totalItems  int
	totalPages  int
	pageSize    int
	calls       int32
	seenReqs    []SyncTxnInfoChainRequest
	statusFalse map[int]string // page_number -> message
}

func (s *scriptedFullnode) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.calls, 1)
		var req SyncTxnInfoChainRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.seenReqs = append(s.seenReqs, req)
		p := req.PageNumber
		if p == 0 {
			p = 1
		}
		w.Header().Set("Content-Type", "application/json")
		if msg, ok := s.statusFalse[p]; ok {
			json.NewEncoder(w).Encode(SyncTxnInfoChainResponse{Status: false, Message: msg})
			return
		}
		result, ok := s.pages[p]
		if !ok {
			// Out-of-script page — return an empty 'no data' shape.
			result = SyncTxnInfoChainResult{
				Data:       map[string][]ChainEntry{},
				PageNumber: p, TotalPages: s.totalPages,
				PageSize: s.pageSize, TotalItems: s.totalItems,
			}
		}
		// Always fill page-size/total fields from the scripted top-level
		// numbers so callers' assertions are stable.
		if result.PageNumber == 0 {
			result.PageNumber = p
		}
		if result.TotalPages == 0 {
			result.TotalPages = s.totalPages
		}
		if result.PageSize == 0 {
			result.PageSize = s.pageSize
		}
		if result.TotalItems == 0 {
			result.TotalItems = s.totalItems
		}
		json.NewEncoder(w).Encode(SyncTxnInfoChainResponse{
			Status: true, Message: "ok", Result: result,
		})
	}
}

// newTestService wires up a Service backed by a real PeerManager + fake
// forwarder + httptest fullnode + an in-memory peer-source that returns
// "peer1" without going to the network.
func newTestService(t *testing.T, script *scriptedFullnode) (*Service, func()) {
	t.Helper()
	srv := httptest.NewServer(script.handler())
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	fwd := &fakeForwarder{}
	pm := NewPeerManager(fwd, port, 1, 5*time.Second)

	// Replace peer source with a registry that returns one entry.
	ps := &PeerSource{}
	ps.URL = "" // not used — we override via the static-list registry below
	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"mainnet":[{"peer_id":"peer1","status":"Active"}],"testnet":[]}`))
	}))
	ps = NewPeerSource(regSrv.URL, "mainnet")

	svc := &Service{
		cfg: Config{
			Network:     "mainnet",
			PageSize:    100,
			HTTPTimeout: 5 * time.Second,
		},
		peerSource:  ps,
		peerManager: pm,
	}
	return svc, func() {
		srv.Close()
		regSrv.Close()
	}
}

func seedTokenRow(t *testing.T, db *gorm.DB, tokenID string) {
	t.Helper()
	if err := db.Create(&models.Token{TokenID: tokenID, DID: "did", TokenType: 1}).Error; err != nil {
		t.Fatalf("seed token %s: %v", tokenID, err)
	}
}

func chainEntry(id, prev string, pos int64, role int16) ChainEntry {
	return ChainEntry{
		ID:                    id,
		Role:                  role,
		Position:              pos,
		PreviousTransactionID: prev,
		Info:                  json.RawMessage(fmt.Sprintf(`{"initiator":"i-%s"}`, id)),
	}
}

func countChain(t *testing.T, db *gorm.DB, tokenID string) int64 {
	t.Helper()
	var n int64
	db.Model(&models.SyncTokenChain{}).Where("token_id = ?", tokenID).Count(&n)
	return n
}

// ---------- scenarios ----------

func TestRunOnce_SinglePage_PersistsAndExits(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	script := &scriptedFullnode{
		totalPages: 1, totalItems: 2, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokA": {
						chainEntry("A1", "", 1, 1),
						chainEntry("A2", "A1", 2, 2),
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())
	if res.PeerID != "peer1" {
		t.Errorf("peer id: want peer1 got %s", res.PeerID)
	}
	if res.TokensScanned != 1 || res.BatchesProcessed != 1 || res.PagesFetched != 1 {
		t.Errorf("summary: %+v", res)
	}
	if n := countChain(t, db, "tokA"); n != 2 {
		t.Errorf("expected 2 rows in SyncTokenChain, got %d", n)
	}
}

func TestRunOnce_MultiPage_FetchesAllPages(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	script := &scriptedFullnode{
		totalPages: 3, totalItems: 5, pageSize: 2,
		pages: map[int]SyncTxnInfoChainResult{
			1: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A1", "", 1, 1), chainEntry("A2", "A1", 2, 1)}}},
			2: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A3", "A2", 3, 1), chainEntry("A4", "A3", 4, 1)}}},
			3: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A5", "A4", 5, 1)}}},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())
	if res.PagesFetched != 3 {
		t.Errorf("expected 3 pages, got %d", res.PagesFetched)
	}
	if n := countChain(t, db, "tokA"); n != 5 {
		t.Errorf("expected 5 chain rows, got %d", n)
	}
}

func TestRunOnce_EmptyResult_ShortCircuits(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	script := &scriptedFullnode{totalPages: 0, totalItems: 0, pageSize: 100}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())
	if res.PagesFetched != 1 {
		t.Errorf("expected only page 1 fetched for empty result, got %d", res.PagesFetched)
	}
	if n := countChain(t, db, "tokA"); n != 0 {
		t.Errorf("expected no chain rows for empty result, got %d", n)
	}
}

func TestRunOnce_GapRefetch_RecoversMissingPage(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Script: page 2 returns status:false on first call, succeeds on second.
	pages := map[int]SyncTxnInfoChainResult{
		1: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A1", "", 1, 1)}}},
		2: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A2", "A1", 2, 1)}}},
		3: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A3", "A2", 3, 1)}}},
	}
	flaky := &flakyScript{pages: pages, totalPages: 3, totalItems: 3, pageSize: 1, failPage: 2, failTimes: 1}
	srv := httptest.NewServer(flaky.handler())
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	fwd := &fakeForwarder{}
	pm := NewPeerManager(fwd, port, 1, 5*time.Second)

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"mainnet":[{"peer_id":"peer1","status":"Active"}]}`))
	}))
	defer regSrv.Close()
	svc := &Service{
		cfg:         Config{Network: "mainnet", PageSize: 1, HTTPTimeout: 5 * time.Second},
		peerSource:  NewPeerSource(regSrv.URL, "mainnet"),
		peerManager: pm,
	}

	res := svc.RunOnce(context.Background())
	if res.PagesFetched != 3 {
		t.Errorf("expected 3 pages after gap recovery, got %d (calls=%d)",
			res.PagesFetched, atomic.LoadInt32(&flaky.calls))
	}
	if n := countChain(t, db, "tokA"); n != 3 {
		t.Errorf("expected 3 chain rows after recovery, got %d", n)
	}
}

// flakyScript: returns status:false for a configured page N times before
// finally returning the canned success.
type flakyScript struct {
	pages      map[int]SyncTxnInfoChainResult
	totalPages int
	totalItems int
	pageSize   int
	failPage   int
	failTimes  int32
	calls      int32

	mu       atomicCounter // pageNumber -> attempts so far
	attempts map[int]*int32
}

type atomicCounter struct{}

func (s *flakyScript) handler() http.HandlerFunc {
	if s.attempts == nil {
		s.attempts = make(map[int]*int32)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.calls, 1)
		var req SyncTxnInfoChainRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		p := req.PageNumber
		if p == 0 {
			p = 1
		}
		w.Header().Set("Content-Type", "application/json")
		if p == s.failPage {
			counter, ok := s.attempts[p]
			if !ok {
				var c int32
				s.attempts[p] = &c
				counter = &c
			}
			n := atomic.AddInt32(counter, 1)
			if n <= s.failTimes {
				json.NewEncoder(w).Encode(SyncTxnInfoChainResponse{Status: false, Message: "flaky"})
				return
			}
		}
		result := s.pages[p]
		result.PageNumber = p
		result.TotalPages = s.totalPages
		result.PageSize = s.pageSize
		result.TotalItems = s.totalItems
		if result.Data == nil {
			result.Data = map[string][]ChainEntry{}
		}
		json.NewEncoder(w).Encode(SyncTxnInfoChainResponse{Status: true, Message: "ok", Result: result})
	}
}

func TestRunOnce_DivergentToken_PurgesAndReplaces(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Seed prior local rows that the divergence purge should clear.
	for pos := int64(1); pos <= 3; pos++ {
		if err := db.Create(&models.SyncTokenChain{
			TokenID: "tokA", Position: pos, TransactionID: fmt.Sprintf("OLD-%d", pos),
			Role: 1, PreviousTransactionID: "",
		}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Single-page response with divergent_tokens = ["tokA"] and a fresh
	// chain from position 0.
	script := &scriptedFullnode{
		totalPages: 1, totalItems: 2, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				DivergentTokens: []string{"tokA"},
				Data: map[string][]ChainEntry{
					"tokA": {
						chainEntry("FRESH-0", "", 0, 1),
						chainEntry("FRESH-1", "FRESH-0", 1, 1),
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())
	if got := len(res.DivergentTokens); got != 1 || res.DivergentTokens[0] != "tokA" {
		t.Errorf("divergent tokens: %+v", res.DivergentTokens)
	}

	var ids []string
	db.Model(&models.SyncTokenChain{}).Where("token_id = ?", "tokA").
		Order("position ASC").Pluck("transaction_id", &ids)
	wantIDs := []string{"FRESH-0", "FRESH-1"}
	if len(ids) != len(wantIDs) {
		t.Fatalf("expected exactly %v after divergent replace, got %v", wantIDs, ids)
	}
	for i, w := range wantIDs {
		if ids[i] != w {
			t.Errorf("row[%d]: want %s got %s", i, w, ids[i])
		}
	}
}

func TestRunOnce_PeerDialFailure_AbortsRun(t *testing.T) {
	_, restore := setupSyncDB(t)
	defer restore()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"mainnet":[{"peer_id":"peer1","status":"Active"}]}`))
	}))
	defer regSrv.Close()
	fwd := &fakeForwarder{forwardErr: fmt.Errorf("dial refused")}
	pm := NewPeerManager(fwd, 19200, 1, time.Second)
	svc := &Service{
		cfg:         Config{Network: "mainnet", PageSize: 100, HTTPTimeout: time.Second},
		peerSource:  NewPeerSource(regSrv.URL, "mainnet"),
		peerManager: pm,
	}

	res := svc.RunOnce(context.Background())
	if res.BatchesProcessed != 0 {
		t.Errorf("expected zero batches on dial failure, got %d", res.BatchesProcessed)
	}
}

func TestRunOnce_NoTokens_SkipsConnect(t *testing.T) {
	_, restore := setupSyncDB(t)
	defer restore()
	// No tokens seeded in Tokens table.

	var connectCount int32
	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"mainnet":[{"peer_id":"peer1","status":"Active"}]}`))
	}))
	defer regSrv.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&connectCount, 1)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	fwd := &fakeForwarder{}
	pm := NewPeerManager(fwd, port, 1, time.Second)
	svc := &Service{
		cfg:         Config{Network: "mainnet", PageSize: 100, HTTPTimeout: time.Second},
		peerSource:  NewPeerSource(regSrv.URL, "mainnet"),
		peerManager: pm,
	}

	res := svc.RunOnce(context.Background())
	if res.TokensScanned != 0 {
		t.Errorf("expected 0 tokens, got %d", res.TokensScanned)
	}
	if got := atomic.LoadInt32(&fwd.forwardCount); got != 0 {
		t.Errorf("no tokens → should not have opened libp2p conn; forwardCount=%d", got)
	}
}

// ---------- pure helpers ----------

func TestChunkTokenIDs_DedupesAndDropsEmpty(t *testing.T) {
	in := []string{"a", "", "b", "a", "c", ""}
	got := chunkTokenIDs(in, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d (%v)", len(got), got)
	}
	flat := append(append([]string{}, got[0]...), got[1]...)
	want := []string{"a", "b", "c"}
	if len(flat) != len(want) {
		t.Errorf("flat %v want %v", flat, want)
	}
	for i := range want {
		if flat[i] != want[i] {
			t.Errorf("idx %d: want %s got %s", i, want[i], flat[i])
		}
	}
}

// ---------- structural validator ----------

func TestRunOnce_ChainBreakMidPage_FlagsAndKeepsValidEntries(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Entries 1..2 are valid. Entry 3 has a bad prev (should be "A2", got "WRONG").
	// Entry 4 would chain correctly *after* entry 3, but the validator stops
	// the moment it sees the break.
	script := &scriptedFullnode{
		totalPages: 1, totalItems: 4, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokA": {
						chainEntry("A1", "", 1, 1),
						chainEntry("A2", "A1", 2, 1),
						chainEntry("A3", "WRONG", 3, 1),
						chainEntry("A4", "A3", 4, 1),
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())

	// A1, A2 stored; A3, A4 not stored — entry-level forensics preserved.
	if n := countChain(t, db, "tokA"); n != 2 {
		t.Errorf("expected 2 chain rows (A1, A2 only), got %d", n)
	}

	// Tokens.NeedsSync flipped to true.
	var tok models.Token
	if err := db.Where("token_id = ?", "tokA").First(&tok).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if !tok.NeedsSync {
		t.Error("Tokens.needs_sync should be true after chain break")
	}

	// Run summary surfaces the flagged token.
	if len(res.ChainBrokenTokens) != 1 || res.ChainBrokenTokens[0] != "tokA" {
		t.Errorf("ChainBrokenTokens: want [tokA] got %v", res.ChainBrokenTokens)
	}
}

func TestRunOnce_PositionGap_FlagsToken(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// prev linkage is correct (A2.prev == A1), but A2 jumps from position 1
	// to position 3 — a position gap. Validator should catch it on the
	// position check, not the prev check.
	script := &scriptedFullnode{
		totalPages: 1, totalItems: 2, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokA": {
						chainEntry("A1", "", 1, 1),
						chainEntry("A2", "A1", 3, 1), // pos 3 follows pos 1 — gap
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())

	// Only A1 stored.
	if n := countChain(t, db, "tokA"); n != 1 {
		t.Errorf("expected 1 chain row (A1 only), got %d", n)
	}
	var tok models.Token
	db.Where("token_id = ?", "tokA").First(&tok)
	if !tok.NeedsSync {
		t.Error("Tokens.needs_sync should be true after position gap")
	}
	if len(res.ChainBrokenTokens) != 1 || res.ChainBrokenTokens[0] != "tokA" {
		t.Errorf("ChainBrokenTokens: want [tokA] got %v", res.ChainBrokenTokens)
	}
}

func TestRunOnce_OtherTokensUnaffectedByOneTokensBreak(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokBad")
	seedTokenRow(t, db, "tokGood")

	script := &scriptedFullnode{
		totalPages: 1, totalItems: 4, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokBad": {
						chainEntry("B1", "", 1, 1),
						chainEntry("B2", "WRONG", 2, 1), // break at entry 2
					},
					"tokGood": {
						chainEntry("G1", "", 1, 1),
						chainEntry("G2", "G1", 2, 1),
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())

	if countChain(t, db, "tokBad") != 1 {
		t.Errorf("tokBad: expected 1 row (B1 only), got %d", countChain(t, db, "tokBad"))
	}
	if countChain(t, db, "tokGood") != 2 {
		t.Errorf("tokGood: expected 2 rows, got %d", countChain(t, db, "tokGood"))
	}

	var bad, good models.Token
	db.Where("token_id = ?", "tokBad").First(&bad)
	db.Where("token_id = ?", "tokGood").First(&good)
	if !bad.NeedsSync {
		t.Error("tokBad should be flagged needs_sync")
	}
	if good.NeedsSync {
		t.Error("tokGood should NOT be flagged needs_sync")
	}
	if len(res.ChainBrokenTokens) != 1 || res.ChainBrokenTokens[0] != "tokBad" {
		t.Errorf("ChainBrokenTokens: want [tokBad] got %v", res.ChainBrokenTokens)
	}
}

func TestRunOnce_BreakAcrossPages_FlagsAndKeepsEarlierPageEntries(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Page 1 is clean. Page 2's first entry breaks the chain. Buffering
	// means all entries are sorted by position before validating, so the
	// break is caught after A1+A2 are validated and inserted.
	pages := map[int]SyncTxnInfoChainResult{
		1: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A1", "", 1, 1), chainEntry("A2", "A1", 2, 1)}}},
		2: {Data: map[string][]ChainEntry{"tokA": {chainEntry("A3", "WRONG", 3, 1), chainEntry("A4", "A3", 4, 1)}}},
	}
	script := &scriptedFullnode{totalPages: 2, totalItems: 4, pageSize: 2, pages: pages}
	svc, stop := newTestService(t, script)
	defer stop()

	res := svc.RunOnce(context.Background())

	if n := countChain(t, db, "tokA"); n != 2 {
		t.Errorf("expected 2 rows (A1, A2) after cross-page break, got %d", n)
	}
	var tok models.Token
	db.Where("token_id = ?", "tokA").First(&tok)
	if !tok.NeedsSync {
		t.Error("Tokens.needs_sync should be true after cross-page break")
	}
	if len(res.ChainBrokenTokens) != 1 || res.ChainBrokenTokens[0] != "tokA" {
		t.Errorf("ChainBrokenTokens: want [tokA] got %v", res.ChainBrokenTokens)
	}
}

func TestRunOnce_Resumption_ValidatesAgainstKnownPosition(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Pre-seed local mirror at position 5, transaction tx5. The server
	// should send entries from position 6 onward, with the first one's
	// prev == "tx5".
	for pos := int64(1); pos <= 5; pos++ {
		txn := fmt.Sprintf("tx%d", pos)
		prev := ""
		if pos > 1 {
			prev = fmt.Sprintf("tx%d", pos-1)
		}
		if err := db.Create(&models.SyncTokenChain{
			TokenID: "tokA", Position: pos, TransactionID: txn,
			PreviousTransactionID: prev, Role: 1,
		}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		// Also seed SyncTransactions so the GetKnownPositions reader doesn't
		// have to (it only reads SyncTokenChain anyway, but be tidy).
	}

	// Server response: a clean continuation from position 6.
	script := &scriptedFullnode{
		totalPages: 1, totalItems: 2, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokA": {
						chainEntry("tx6", "tx5", 6, 1),
						chainEntry("tx7", "tx6", 7, 1),
					},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	svc.RunOnce(context.Background())

	if n := countChain(t, db, "tokA"); n != 7 {
		t.Errorf("expected 7 rows total (5 seeded + 2 new), got %d", n)
	}
	var tok models.Token
	db.Where("token_id = ?", "tokA").First(&tok)
	if tok.NeedsSync {
		t.Error("Tokens.needs_sync should NOT be set after a clean resumption")
	}
}

func TestRunOnce_Resumption_BreakAgainstKnownPosition_Flags(t *testing.T) {
	db, restore := setupSyncDB(t)
	defer restore()
	seedTokenRow(t, db, "tokA")

	// Pre-seed at position 5 = tx5.
	for pos := int64(1); pos <= 5; pos++ {
		txn := fmt.Sprintf("tx%d", pos)
		prev := ""
		if pos > 1 {
			prev = fmt.Sprintf("tx%d", pos-1)
		}
		if err := db.Create(&models.SyncTokenChain{
			TokenID: "tokA", Position: pos, TransactionID: txn,
			PreviousTransactionID: prev, Role: 1,
		}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Server returns position 6 with prev="WRONG" — should fail validation.
	script := &scriptedFullnode{
		totalPages: 1, totalItems: 1, pageSize: 100,
		pages: map[int]SyncTxnInfoChainResult{
			1: {
				Data: map[string][]ChainEntry{
					"tokA": {chainEntry("tx6", "WRONG", 6, 1)},
				},
			},
		},
	}
	svc, stop := newTestService(t, script)
	defer stop()

	svc.RunOnce(context.Background())

	// No new rows inserted (the seeded 5 stay).
	if n := countChain(t, db, "tokA"); n != 5 {
		t.Errorf("expected 5 rows (just the seeded ones), got %d", n)
	}
	var tok models.Token
	db.Where("token_id = ?", "tokA").First(&tok)
	if !tok.NeedsSync {
		t.Error("Tokens.needs_sync should be true after resumption mismatch")
	}
}

func TestMissingPages(t *testing.T) {
	got := missingPages(5, map[int]struct{}{1: {}, 2: {}, 4: {}})
	want := []int{3, 5}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("want %v got %v", want, got)
	}
}

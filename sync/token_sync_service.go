package sync

import (
	"context"
	"errors"
	"explorer-server/database"
	"explorer-server/database/operations"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSyncInterval     = 5 * time.Minute
	defaultBatchDelay       = 1 * time.Minute
	defaultHTTPTimeout      = 110 * time.Second
	defaultBatchLimit       = 500
	defaultLocalBridgePort  = 7100
	defaultIPFSHost         = "localhost:5001"
)

// TokenSyncService periodically backfills token chains from a fullnode peer
// over a libp2p p2p-forward bridge.
type TokenSyncService struct {
	client     *TokenChainClient
	interval   time.Duration
	limit      int
	batchDelay time.Duration
}

// Config holds connection settings for token chain sync.
// Transport is libp2p (via local IPFS daemon); no proxy URL, no API key.
type Config struct {
	IPFSHost        string        // local IPFS daemon HTTP API (e.g. "127.0.0.1:5001")
	FullnodePeerID  string        // explicit peer ID; if empty, resolved from PeersURL+Network
	PeersURL        string        // registry URL (defaults to DefaultFullnodePeersURL)
	Network         string        // "mainnet" or "testnet" — used only when FullnodePeerID is empty
	LocalBridgePort int           // local TCP port for the p2p-forward bridge
	Interval        time.Duration
	HTTPTimeout     time.Duration
	BatchLimit      int
	BatchDelay      time.Duration
}

// ConfigFromEnv reads TOKEN_SYNC_* environment variables.
// testnet should reflect the -testnet CLI flag from main.go; it picks the
// network bucket in the fullnode peers registry when no explicit peer ID is set.
// Returns ok=false when sync is disabled OR no peer source is configured.
func ConfigFromEnv(testnet bool) (Config, bool) {
	enabled := true
	if v := os.Getenv("TOKEN_SYNC_ENABLED"); v != "" {
		enabled, _ = strconv.ParseBool(v)
	}
	if !enabled {
		return Config{}, false
	}

	peerID := strings.TrimSpace(os.Getenv("TOKEN_SYNC_FULLNODE_PEER_ID"))
	peersURL := strings.TrimSpace(os.Getenv("TOKEN_SYNC_FULLNODE_PEERS_URL"))
	if peersURL == "" {
		peersURL = DefaultFullnodePeersURL
	}

	// Need at least one peer source: explicit peer ID or a registry URL.
	if peerID == "" && peersURL == "" {
		return Config{}, false
	}

	network := "mainnet"
	if testnet {
		network = "testnet"
	}

	ipfsHost := os.Getenv("IPFS_HOST")
	if ipfsHost == "" {
		ipfsHost = defaultIPFSHost
	}

	port := defaultLocalBridgePort
	if v := os.Getenv("TOKEN_SYNC_LOCAL_BRIDGE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			port = n
		}
	}

	interval := defaultSyncInterval
	if v := os.Getenv("TOKEN_SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}

	timeout := defaultHTTPTimeout
	if v := os.Getenv("TOKEN_SYNC_HTTP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}

	limit := defaultBatchLimit
	if v := os.Getenv("TOKEN_SYNC_TOKEN_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	batchDelay := defaultBatchDelay
	if v := os.Getenv("TOKEN_SYNC_BATCH_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			batchDelay = d
		}
	}

	return Config{
		IPFSHost:        ipfsHost,
		FullnodePeerID:  peerID,
		PeersURL:        peersURL,
		Network:         network,
		LocalBridgePort: port,
		Interval:        interval,
		HTTPTimeout:     timeout,
		BatchLimit:      limit,
		BatchDelay:      batchDelay,
	}, true
}

// resolverFor returns the peer resolver implied by the config. If an explicit
// FullnodePeerID is set it wins; otherwise we use the registry URL + network.
func resolverFor(cfg Config) PeerResolver {
	if strings.TrimSpace(cfg.FullnodePeerID) != "" {
		return StaticPeerResolver{PeerID: cfg.FullnodePeerID}
	}
	return RegistryPeerResolver{
		URL:     cfg.PeersURL,
		Network: cfg.Network,
	}
}

// NewTokenSyncService builds a service using a real IPFS shell forwarder.
func NewTokenSyncService(cfg Config) *TokenSyncService {
	forwarder := NewShellForwarder(cfg.IPFSHost)
	return NewTokenSyncServiceWithForwarder(cfg, forwarder)
}

// NewTokenSyncServiceWithForwarder lets tests inject a fake StreamForwarder.
func NewTokenSyncServiceWithForwarder(cfg Config, forwarder StreamForwarder) *TokenSyncService {
	return &TokenSyncService{
		client:     NewTokenChainClient(forwarder, resolverFor(cfg), cfg.LocalBridgePort, cfg.HTTPTimeout),
		interval:   cfg.Interval,
		limit:      cfg.BatchLimit,
		batchDelay: cfg.BatchDelay,
	}
}

// NewTokenSyncServiceFromEnv returns nil when sync is disabled or no peer source
// is configured. testnet must reflect the -testnet CLI flag from main.go.
func NewTokenSyncServiceFromEnv(testnet bool) *TokenSyncService {
	cfg, ok := ConfigFromEnv(testnet)
	if !ok {
		return nil
	}
	return NewTokenSyncService(cfg)
}

// Run executes sync cycles until ctx is cancelled.
// (Kept for future periodic mode; main.go currently invokes RunOnce via -sync flag.)
func (s *TokenSyncService) Run(ctx context.Context) {
	log.Printf("[TokenSync] Starting periodic sync (interval %s)", s.interval)
	s.RunOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[TokenSync] Stopped periodic sync")
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce loads pending tokens, fetches chains from the fullnode, and ingests them.
// Sleeps batchDelay between batches (not after the last) to avoid hammering the bridge.
func (s *TokenSyncService) RunOnce(ctx context.Context) {
	tokenIDs, err := operations.GetTokenIDsNeedingSync(database.ReadDB, s.limit)
	if err != nil {
		log.Printf("[TokenSync] Failed to list tokens needing sync: %v", err)
		return
	}
	if len(tokenIDs) == 0 {
		log.Println("[TokenSync] No tokens flagged for sync — nothing to do")
		return
	}

	batches := operations.BatchTokenIDs(tokenIDs)
	log.Printf("[TokenSync] Syncing %d token(s) in %d batch(es) of max %d (batch delay %s)",
		len(tokenIDs), len(batches), operations.MaxTokenSyncBatchSize, s.batchDelay)

	var fullySynced, partial, failed int

	for i, batch := range batches {
		if ctx.Err() != nil {
			log.Println("[TokenSync] Context cancelled — aborting remaining batches")
			break
		}

		log.Printf("[TokenSync] Batch %d/%d: requesting %d token(s)", i+1, len(batches), len(batch))
		chains, err := s.client.FetchTokenChains(batch)
		if err != nil {
			log.Printf("[TokenSync] Batch %d/%d failed (%d token(s)): %v", i+1, len(batches), len(batch), err)
			continue
		}

		batchFull, batchPartial, batchFailed := 0, 0, 0
		for _, tokenID := range batch {
			if ctx.Err() != nil {
				break
			}
			chain := chains[tokenID]
			err := operations.IngestTokenChain(nil, tokenID, chain)
			switch {
			case err == nil:
				batchFull++
			case errors.Is(err, operations.ErrChainValidationFailed):
				batchPartial++
				log.Printf("[TokenSync] Token %s: partial sync (%v)", tokenID, err)
			default:
				batchFailed++
				log.Printf("[TokenSync] Ingest failed for token %s: %v", tokenID, err)
			}
		}
		fullySynced += batchFull
		partial += batchPartial
		failed += batchFailed
		log.Printf("[TokenSync] Batch %d/%d done: %d full, %d partial, %d failed",
			i+1, len(batches), batchFull, batchPartial, batchFailed)

		if i < len(batches)-1 && s.batchDelay > 0 {
			log.Printf("[TokenSync] Sleeping %s before next batch", s.batchDelay)
			select {
			case <-ctx.Done():
				log.Println("[TokenSync] Context cancelled during batch delay — aborting")
				return
			case <-time.After(s.batchDelay):
			}
		}
	}

	log.Printf("[TokenSync] Sync cycle complete: %d fully synced, %d partial (chain break — token stays flagged), %d failed",
		fullySynced, partial, failed)
}

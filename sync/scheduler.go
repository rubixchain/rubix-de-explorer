package sync

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Scheduling defaults and env knobs.
const (
	// DefaultInterval matches the spec: every 6 hours.
	DefaultInterval = 6 * time.Hour

	// envEnabled gates the periodic scheduler. Independent from the -sync
	// CLI flag, which only triggers the one-shot.
	envEnabled = "TOKEN_SYNC_SCHEDULER_ENABLED"

	// envInterval overrides the 6h default (Go duration string, e.g. "30m").
	envInterval = "TOKEN_SYNC_INTERVAL"

	// envPeersURL overrides DefaultFullnodePeersURL.
	envPeersURL = "TOKEN_SYNC_FULLNODE_PEERS_URL"

	// envPageSize overrides DefaultPageSize. Must stay constant across an
	// entire run; we enforce that implicitly by reading it once per
	// Service.
	envPageSize = "TOKEN_SYNC_PAGE_SIZE"

	// envTokenLimit caps how many token IDs a single run processes.
	envTokenLimit = "TOKEN_SYNC_TOKEN_LIMIT"

	// envHTTPTimeout overrides the per-request timeout.
	envHTTPTimeout = "TOKEN_SYNC_HTTP_TIMEOUT"

	// envPortStart / envPortCount delimit the libp2p bridge port pool.
	envPortStart = "TOKEN_SYNC_LOCAL_BRIDGE_PORT_START"
	envPortCount = "TOKEN_SYNC_LOCAL_BRIDGE_PORT_COUNT"

	// envOneShotDelay overrides the 10s delay between startup and the
	// -sync one-shot. Lets tests run it immediately.
	envOneShotDelay = "TOKEN_SYNC_ONESHOT_DELAY"
)

// ConfigFromEnv reads explorer env vars into a sync Config. The testnet
// arg threads the -testnet CLI flag through to the peer-source bucket.
func ConfigFromEnv(testnet bool) Config {
	cfg := Config{
		PeersURL:    DefaultFullnodePeersURL,
		PageSize:    DefaultPageSize,
		HTTPTimeout: 110 * time.Second,
	}
	if testnet {
		cfg.Network = "testnet"
	} else {
		cfg.Network = "mainnet"
	}
	if v := strings.TrimSpace(os.Getenv(envPeersURL)); v != "" {
		cfg.PeersURL = v
	}
	if v := strings.TrimSpace(os.Getenv(envPageSize)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.PageSize = n
		}
	}
	if v := strings.TrimSpace(os.Getenv(envTokenLimit)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TokenLimit = n
		}
	}
	if v := strings.TrimSpace(os.Getenv(envHTTPTimeout)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.HTTPTimeout = d
		}
	}
	if v := strings.TrimSpace(os.Getenv(envPortStart)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LocalBridgePortStart = n
		}
	}
	if v := strings.TrimSpace(os.Getenv(envPortCount)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LocalBridgePortCount = n
		}
	}
	return cfg
}

// SchedulerEnabledFromEnv reports whether TOKEN_SYNC_SCHEDULER_ENABLED is set
// to a truthy value. Defaults to false — operators opt into the recurring
// timer explicitly.
func SchedulerEnabledFromEnv() bool {
	v := strings.TrimSpace(os.Getenv(envEnabled))
	if v == "" {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

// IntervalFromEnv returns the periodic-sync interval; defaults to 6h.
func IntervalFromEnv() time.Duration {
	if v := strings.TrimSpace(os.Getenv(envInterval)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultInterval
}

// OneShotDelayFromEnv returns the delay between startup and the -sync
// one-shot. Defaults to 10s so we don't race the IPFS daemon startup.
func OneShotDelayFromEnv() time.Duration {
	if v := strings.TrimSpace(os.Getenv(envOneShotDelay)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return 10 * time.Second
}

// Scheduler wraps a Service with the overlap guard and the periodic ticker.
// One Scheduler per process; the overlap guard ensures a tick that fires
// while a previous run is still in-flight silently skips instead of running
// concurrently. Mid-run drops will be caught by the next scheduled tick.
type Scheduler struct {
	svc      *Service
	interval time.Duration
	running  atomic.Bool
}

// NewScheduler builds a Scheduler around the given Service. Pass the
// service factory (NewService) the env-derived Config and IPFS forwarder.
func NewScheduler(svc *Service, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Scheduler{svc: svc, interval: interval}
}

// RunOneShot triggers a single sync run if no other run is currently active.
// Returns true if the run was started (and completed) by this call; false if
// the overlap guard rejected the attempt.
//
// Blocks until the run finishes. Caller picks whether to invoke from the
// main goroutine or a background goroutine.
func (s *Scheduler) RunOneShot(ctx context.Context, reason string) bool {
	if !s.running.CompareAndSwap(false, true) {
		log.Printf("[Sync] %s requested but another run is in progress — skipping", reason)
		return false
	}
	defer s.running.Store(false)

	log.Printf("[Sync] %s starting", reason)
	res := s.svc.RunOnce(ctx)
	logRunSummary(reason, res)
	return true
}

// Start runs the recurring scheduler until ctx is cancelled. Each tick calls
// RunOneShot; ticks that arrive while a run is still active are dropped by
// the overlap guard.
//
// Start does NOT do an initial run on entry — the -sync one-shot is a
// separate path. If you want a startup-and-then-periodic schedule, call
// RunOneShot followed by Start.
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[Sync] scheduler starting (interval=%s)", s.interval)
	t := time.NewTicker(s.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Sync] scheduler stopped: %v", ctx.Err())
			return
		case <-t.C:
			s.RunOneShot(ctx, "scheduled run")
		}
	}
}

func logRunSummary(reason string, r RunResult) {
	log.Printf("[Sync] %s finished: peer=%s tokens_scanned=%d batches_processed=%d batches_failed=%d pages_fetched=%d divergent_tokens=%d chain_broken_tokens=%d duration=%s",
		reason,
		r.PeerID,
		r.TokensScanned,
		r.BatchesProcessed,
		r.BatchesFailed,
		r.PagesFetched,
		len(r.DivergentTokens),
		len(r.ChainBrokenTokens),
		r.Duration.Round(time.Millisecond),
	)
	if len(r.DivergentTokens) > 0 {
		log.Printf("[Sync] %s divergent token IDs: %v", reason, r.DivergentTokens)
	}
	if len(r.ChainBrokenTokens) > 0 {
		log.Printf("[Sync] %s chain-broken token IDs (flagged needs_sync): %v", reason, r.ChainBrokenTokens)
	}
}

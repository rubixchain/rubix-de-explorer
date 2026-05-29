package sync

import (
	"context"
	"encoding/json"
	"errors"
	"explorer-server/model"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// ---------- test scaffolding ----------

// fakeForwarder records the calls a TokenChainClient would make to the IPFS
// shell, without actually setting up a libp2p bridge. Tests pair this with a
// real httptest.Server on the same loopback port — the HTTP request lands on
// the httptest server directly, bypassing libp2p entirely.
type fakeForwarder struct {
	forwardCount int32
	closeCount   int32
	forwardErr   error
}

func (f *fakeForwarder) Forward(_ context.Context, _ /*protocol*/, _ /*listenAddr*/, _ /*targetAddr*/ string) error {
	atomic.AddInt32(&f.forwardCount, 1)
	return f.forwardErr
}

func (f *fakeForwarder) Close(_ context.Context, _ /*listenAddr*/ string) error {
	atomic.AddInt32(&f.closeCount, 1)
	return nil
}

// startFake spins up an httptest server and returns a TokenChainClient pointed
// at it. The fakeForwarder no-ops, so the client's HTTP POST goes straight to
// the httptest handler on the loopback port. The resolver is static — no
// network call to GitHub from unit tests.
func startFake(t *testing.T, handler http.HandlerFunc) (*TokenChainClient, *fakeForwarder, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	u, err := url.Parse(srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("parse httptest URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		srv.Close()
		t.Fatalf("parse port: %v", err)
	}
	fwd := &fakeForwarder{}
	client := NewTokenChainClient(fwd, StaticPeerResolver{PeerID: "12D3KooWTestPeerID"}, port, 5*time.Second)
	return client, fwd, srv.Close
}

func successPayload(tokenID string, txns []model.SyncedTxn) []byte {
	b, _ := json.Marshal(model.SyncTokenChainResponse{
		Status:  true,
		Message: "ok",
		Result:  map[string][]model.SyncedTxn{tokenID: txns},
	})
	return b
}

// ---------- bridge / forwarder behaviour ----------

func TestFetchTokenChains_BridgeSetupOncePerProcess(t *testing.T) {
	client, fwd, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(successPayload("tok", []model.SyncedTxn{}))
	})
	defer stop()

	// Three calls — Forward should be invoked exactly once (lazy + cached).
	for i := 0; i < 3; i++ {
		if _, err := client.FetchTokenChains([]string{"tok"}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&fwd.forwardCount); got != 1 {
		t.Errorf("Forward should run once, got %d calls", got)
	}
	// Close runs once too (stale-forward cleanup before Forward).
	if got := atomic.LoadInt32(&fwd.closeCount); got != 1 {
		t.Errorf("Close should run once during setup, got %d calls", got)
	}
}

func TestFetchTokenChains_BridgeSetupFailureSurfaced(t *testing.T) {
	// httptest server exists but Forward returns an error — should fail fast.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	fwd := &fakeForwarder{forwardErr: errors.New("peer unreachable")}
	client := NewTokenChainClient(fwd, StaticPeerResolver{PeerID: "12D3KooWTestPeerID"}, port, 2*time.Second)

	_, err := client.FetchTokenChains([]string{"tok"})
	if err == nil {
		t.Fatal("expected bridge setup error")
	}
	if !contains(err.Error(), "p2p forward") || !contains(err.Error(), "peer unreachable") {
		t.Errorf("error should mention bridge setup failure, got: %v", err)
	}
}

func TestFetchTokenChains_EmptyPeerIDError(t *testing.T) {
	fwd := &fakeForwarder{}
	// StaticPeerResolver with empty PeerID surfaces the error via ResolvePeerID.
	client := NewTokenChainClient(fwd, StaticPeerResolver{PeerID: ""}, 7100, 2*time.Second)
	_, err := client.FetchTokenChains([]string{"tok"})
	if err == nil {
		t.Fatal("expected error when peer ID is empty")
	}
}

func TestFetchTokenChains_EmptySlice(t *testing.T) {
	fwd := &fakeForwarder{}
	client := NewTokenChainClient(fwd, StaticPeerResolver{PeerID: "12D3KooW"}, 7100, 2*time.Second)
	result, err := client.FetchTokenChains([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}
	// Must NOT have touched the bridge for an empty input.
	if atomic.LoadInt32(&fwd.forwardCount) != 0 {
		t.Error("empty input should not trigger bridge setup")
	}
}

// ---------- response shape ----------

func TestFetchTokenChains_HappyPath_NewShape(t *testing.T) {
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != syncTokenChainPath {
			t.Errorf("expected POST to %s, got %s", syncTokenChainPath, r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// libp2p transport — no API key header should be sent.
		if r.Header.Get("X-API-KEY") != "" {
			t.Errorf("X-API-KEY must not be sent over libp2p bridge")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(successPayload("tok1", []model.SyncedTxn{
			{ID: "A", Role: 1, PreviousTransactionID: "", Info: &model.TransactionInfo{Initiator: "did1"}},
			{ID: "B", Role: 2, PreviousTransactionID: "A", Info: &model.TransactionInfo{Initiator: "did2"}},
		}))
	})
	defer stop()

	result, err := client.FetchTokenChains([]string{"tok1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	chain, ok := result["tok1"]
	if !ok || len(chain) != 2 {
		t.Fatalf("expected 2 entries, got %v", result)
	}
	if chain[0].ID != "A" || chain[0].Role != 1 || chain[0].PreviousTransactionID != "" {
		t.Errorf("entry 0 mismatch: %+v", chain[0])
	}
	if chain[1].ID != "B" || chain[1].PreviousTransactionID != "A" {
		t.Errorf("entry 1 mismatch: %+v", chain[1])
	}
	if chain[1].Info == nil || chain[1].Info.Initiator != "did2" {
		t.Errorf("entry 1 Info not decoded: %+v", chain[1].Info)
	}
}

// ---------- error matrix (libp2p-relevant subset) ----------

func TestFetchTokenChains_502_RetriesAndFails(t *testing.T) {
	var hits int32
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	})
	defer stop()

	_, err := client.FetchTokenChains([]string{"t"})
	if err == nil {
		t.Fatal("expected error on persistent 502")
	}
	if n := atomic.LoadInt32(&hits); n != int32(maxAttemptsUnreachable) {
		t.Errorf("expected %d attempts for 502, got %d", maxAttemptsUnreachable, n)
	}
}

func TestFetchTokenChains_502_RecoverOnRetry(t *testing.T) {
	var hits int32
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(successPayload("tok", []model.SyncedTxn{}))
	})
	defer stop()

	result, err := client.FetchTokenChains([]string{"tok"})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestFetchTokenChains_500_RetriesOnce(t *testing.T) {
	var hits int32
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer stop()

	_, err := client.FetchTokenChains([]string{"t"})
	if err == nil {
		t.Fatal("expected error on persistent 500")
	}
	if n := atomic.LoadInt32(&hits); n != int32(maxAttempts500) {
		t.Errorf("500 should retry once (%d attempts), got %d", maxAttempts500, n)
	}
}

func TestFetchTokenChains_FullnodeError_PermanentNoRetry(t *testing.T) {
	var hits int32
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SyncTokenChainResponse{
			Status: false, Message: "max 50 token IDs per request",
		})
	})
	defer stop()

	_, err := client.FetchTokenChains([]string{"t"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.As(err, new(*permanentClientError)) {
		t.Errorf("expected permanentClientError, got %T: %v", err, err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("permanent fullnode error should not retry; got %d hits", n)
	}
}

func TestFetchTokenChains_FullnodeError_TransientRetriesOnce(t *testing.T) {
	var hits int32
	client, _, stop := startFake(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.SyncTokenChainResponse{
			Status: false, Message: "transient fullnode hiccup",
		})
	})
	defer stop()

	_, err := client.FetchTokenChains([]string{"t"})
	if err == nil {
		t.Fatal("expected error")
	}
	if n := atomic.LoadInt32(&hits); n != int32(maxAttemptsFullnode) {
		t.Errorf("transient fullnode error should retry once (%d attempts), got %d", maxAttemptsFullnode, n)
	}
}

// ---------- helpers ----------

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Ensure the protocol/path constants are referenced so they don't drift silently.
func TestConstants_AreExported(t *testing.T) {
	if syncTokenChainPath != "/rubix/v1/fullnode/sync-token-chain" {
		t.Errorf("syncTokenChainPath drifted: %q", syncTokenChainPath)
	}
	if syncProtocolID != "/x/RubixFullnodeSyncAPI/1.0" {
		t.Errorf("syncProtocolID drifted: %q", syncProtocolID)
	}
	// silence unused-import in some build configurations
	_ = fmt.Sprintf
}

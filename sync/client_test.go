package sync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeForwarder swaps out the IPFS daemon. It records swarm-connect /
// forward / close calls without actually setting up a tunnel; the test then
// uses an httptest.Server bound to the same loopback port the PeerManager
// would have asked for, so the HTTP request lands on the real test handler.
type fakeForwarder struct {
	mu              sync.Mutex
	connectCount    int32
	forwardCount    int32
	closeCount      int32
	lastProtocol    string
	lastListenAddr  string
	forwardErr      error
	connectErr      error
}

func (f *fakeForwarder) SwarmConnect(_ context.Context, _ ...string) error {
	atomic.AddInt32(&f.connectCount, 1)
	return f.connectErr
}

func (f *fakeForwarder) Forward(_ context.Context, protocol, listenAddr, _ string) error {
	atomic.AddInt32(&f.forwardCount, 1)
	f.mu.Lock()
	f.lastProtocol = protocol
	f.lastListenAddr = listenAddr
	f.mu.Unlock()
	return f.forwardErr
}

func (f *fakeForwarder) Close(_ context.Context, _ string) error {
	atomic.AddInt32(&f.closeCount, 1)
	return nil
}

func startTestServer(t *testing.T, h http.HandlerFunc) (*PeerManager, *fakeForwarder, int, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	fwd := &fakeForwarder{}
	pm := NewPeerManager(fwd, port, 1, 5*time.Second)
	return pm, fwd, port, srv.Close
}

// ---------- PeerManager / OpenPeerConn semantics ----------

func TestOpenPeerConn_HappyPath_CallsSwarmThenForwardThenForwardsRequest(t *testing.T) {
	var hits int32
	pm, fwd, _, stop := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":true,"message":"ok","result":{"data":{},"page_number":1,"total_pages":0,"page_size":100,"total_items":0}}`))
	})
	defer stop()

	conn, err := pm.OpenPeerConn(context.Background(), "12D3KooWTestPeer", SyncAppName)
	if err != nil {
		t.Fatalf("OpenPeerConn: %v", err)
	}
	defer conn.Close()

	if got := atomic.LoadInt32(&fwd.connectCount); got != 1 {
		t.Errorf("SwarmConnect should run once, got %d", got)
	}
	if got := atomic.LoadInt32(&fwd.forwardCount); got != 1 {
		t.Errorf("Forward should run once, got %d", got)
	}
	// Stale-forward teardown happens too.
	if got := atomic.LoadInt32(&fwd.closeCount); got < 1 {
		t.Errorf("Close should run at least once for stale-cleanup, got %d", got)
	}

	wantProtocol := "/x/12D3KooWTestPeerRubixCore/1.0"
	if fwd.lastProtocol != wantProtocol {
		t.Errorf("protocol mismatch: want %q got %q", wantProtocol, fwd.lastProtocol)
	}

	var resp SyncTxnInfoChainResponse
	if err := conn.SendJSONRequest("POST", "/rubix/v1/fullnode/sync-txn-info-chain", nil, map[string]any{"token_ids": []string{"x"}}, &resp, false); err != nil {
		t.Fatalf("SendJSONRequest: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected Status=true")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 HTTP hit, got %d", got)
	}
}

func TestOpenPeerConn_EmptyPeerID_Rejects(t *testing.T) {
	pm := NewPeerManager(&fakeForwarder{}, 19000, 1, time.Second)
	if _, err := pm.OpenPeerConn(context.Background(), "", SyncAppName); err == nil {
		t.Fatal("expected error for empty peer ID")
	}
}

func TestOpenPeerConn_ForwardFailure_PortReleased(t *testing.T) {
	fwd := &fakeForwarder{forwardErr: errors.New("forward refused")}
	pm := NewPeerManager(fwd, 19010, 1, time.Second)
	if _, err := pm.OpenPeerConn(context.Background(), "peer1", SyncAppName); err == nil {
		t.Fatal("expected forward error to surface")
	}
	// A second call should succeed (port wasn't permanently leased).
	fwd.forwardErr = nil
	conn, err := pm.OpenPeerConn(context.Background(), "peer1", SyncAppName)
	if err != nil {
		t.Fatalf("second OpenPeerConn after recovery: %v", err)
	}
	defer conn.Close()
}

func TestPeerConn_Close_Idempotent(t *testing.T) {
	pm, _, _, stop := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer stop()
	conn, err := pm.OpenPeerConn(context.Background(), "peer1", SyncAppName)
	if err != nil {
		t.Fatalf("OpenPeerConn: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close should be no-op, got: %v", err)
	}
}

func TestPeerConn_SendJSONRequest_NonOK_ReturnsError(t *testing.T) {
	pm, _, _, stop := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer stop()
	conn, err := pm.OpenPeerConn(context.Background(), "peer1", SyncAppName)
	if err != nil {
		t.Fatalf("OpenPeerConn: %v", err)
	}
	defer conn.Close()
	var out any
	if err := conn.SendJSONRequest("POST", "/x", nil, struct{}{}, &out, false); err == nil {
		t.Fatal("expected non-2xx error")
	}
}

func TestPeerConn_SendJSONRequest_AfterClose_Rejects(t *testing.T) {
	pm, _, _, stop := startTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer stop()
	conn, _ := pm.OpenPeerConn(context.Background(), "peer1", SyncAppName)
	conn.Close()
	if err := conn.SendJSONRequest("POST", "/x", nil, nil, nil, false); err == nil {
		t.Fatal("expected error sending on closed conn")
	}
}

// ---------- port pool ----------

func TestPeerManager_PortPool_Exhaustion(t *testing.T) {
	fwd := &fakeForwarder{}
	pm := NewPeerManager(fwd, 19100, 2, time.Second)
	c1, err := pm.OpenPeerConn(context.Background(), "p1", SyncAppName)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	c2, err := pm.OpenPeerConn(context.Background(), "p2", SyncAppName)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if _, err := pm.OpenPeerConn(context.Background(), "p3", SyncAppName); err == nil {
		t.Fatal("expected exhaustion error on third acquire")
	}
	c1.Close()
	// One slot freed — third acquire should now succeed.
	c3, err := pm.OpenPeerConn(context.Background(), "p3", SyncAppName)
	if err != nil {
		t.Fatalf("third acquire after release: %v", err)
	}
	c2.Close()
	c3.Close()
}

// ---------- wire decode of ChainEntry.ParseInfo ----------

func TestChainEntry_ParseInfo_Cases(t *testing.T) {
	type tcase struct {
		name    string
		raw     string
		wantNil bool
		wantErr bool
	}
	tests := []tcase{
		{"empty", ``, true, false},
		{"null", `null`, true, false},
		{"valid", `{"initiator":"did1"}`, false, false},
		// Shape mismatch is an error path; ParseInfo returns (nil, err) so
		// callers can drop the entry without dereferencing a partial value.
		{"shape mismatch", `[1,2,3]`, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ChainEntry{Info: json.RawMessage(tt.raw)}
			info, err := e.ParseInfo()
			if (err != nil) != tt.wantErr {
				t.Errorf("err: want %v got %v", tt.wantErr, err)
			}
			if (info == nil) != tt.wantNil {
				t.Errorf("nil-info: want %v got %v", tt.wantNil, info == nil)
			}
		})
	}
}

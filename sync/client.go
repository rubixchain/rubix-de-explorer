package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	sync2 "sync"
	"sync/atomic"
	"time"

	ipfsapi "github.com/ipfs/go-ipfs-api"
)

// In-process PeerManager equivalent for the Rubix libp2p protocol.
//
// The Rubix reference implementation (rubixgoplatform/core/ipfsport/peer.go)
// exposes pm.OpenPeerConn(peerID, "", appName) which:
//   1. Issues `ipfs swarm connect /p2p/<peerID>` (direct or via circuit).
//   2. Issues `ipfs p2p forward /x/<appName>/1.0 /ip4/127.0.0.1/tcp/<port>
//      /p2p/<peerID>` to allocate a local TCP socket bridged into a libp2p
//      stream on the remote peer.
//   3. Wraps the socket in an HTTP client (ensweb.NewClient).
//
// This file reimplements that surface in-process against go-ipfs-api so the
// explorer doesn't have to pull in the full rubixgoplatform dep tree.
// Behaviour on the wire is identical: HTTP-over-libp2p, addressed by peer ID,
// with the same protocol-ID convention `/x/<appName>/1.0`.

// SyncAppName is the protocol app name we open against the fullnode.
//
// The fullnode side registers `/x/<peerID>RubixCore/1.0` per-peer (the
// fullnode's own peer ID is interpolated into the protocol path). We build
// the protocol ID with the *remote* peer ID we're dialing.
const SyncAppName = "RubixCore"

// IPFSForwarder is the slice of go-ipfs-api the client needs. Defining it as
// an interface lets tests inject a fake without spinning up a real daemon.
type IPFSForwarder interface {
	SwarmConnect(ctx context.Context, addrs ...string) error
	Forward(ctx context.Context, protocol, listenAddr, targetAddr string) error
	Close(ctx context.Context, listenAddr string) error
}

// shellAdapter wraps a *ipfsapi.Shell to satisfy IPFSForwarder. Production
// path; tests use the in-memory fakes in client_test.go.
type shellAdapter struct {
	shell *ipfsapi.Shell
}

// NewShellAdapter returns an IPFSForwarder backed by a local IPFS daemon at
// ipfsHost (e.g. "127.0.0.1:5001"). The daemon must have
// Experimental.Libp2pStreamMounting = true; the explorer's IPFSManager
// already sets that.
func NewShellAdapter(ipfsHost string) IPFSForwarder {
	return &shellAdapter{shell: ipfsapi.NewShell(ipfsHost)}
}

func (s *shellAdapter) SwarmConnect(ctx context.Context, addrs ...string) error {
	return s.shell.SwarmConnect(ctx, addrs...)
}

func (s *shellAdapter) Forward(ctx context.Context, protocol, listenAddr, targetAddr string) error {
	resp, err := s.shell.Request("p2p/forward", protocol, listenAddr, targetAddr).Send(ctx)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func (s *shellAdapter) Close(ctx context.Context, listenAddr string) error {
	resp, err := s.shell.Request("p2p/close").Option("listen-address", listenAddr).Send(ctx)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		// "no rules to close" just means the listener wasn't set up.
		if strings.Contains(strings.ToLower(resp.Error.Error()), "no rules") {
			return nil
		}
		return resp.Error
	}
	return nil
}

// PeerManager mints PeerConn handles. Each PeerConn corresponds to one
// (peerID, appName) tunnel and one local TCP port from the pool. The manager
// is process-wide so concurrent sync runs (if we ever add them) reuse the
// same port range and don't collide.
type PeerManager struct {
	ipfs       IPFSForwarder
	startPort  int
	maxPorts   int
	httpClient *http.Client

	mu    sync2.Mutex
	inUse map[int]struct{}
}

// NewPeerManager builds a manager that allocates TCP ports in
// [startPort, startPort+maxPorts) and reaches the fullnode via the supplied
// IPFSForwarder. httpTimeout caps any single HTTP request issued through
// SendJSONRequest.
func NewPeerManager(ipfs IPFSForwarder, startPort, maxPorts int, httpTimeout time.Duration) *PeerManager {
	if maxPorts <= 0 {
		maxPorts = 32
	}
	return &PeerManager{
		ipfs:       ipfs,
		startPort:  startPort,
		maxPorts:   maxPorts,
		httpClient: &http.Client{Timeout: httpTimeout},
		inUse:      make(map[int]struct{}),
	}
}

// acquirePort reserves a port within the manager's pool. Released via
// releasePort when the PeerConn is closed.
func (pm *PeerManager) acquirePort() (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i := 0; i < pm.maxPorts; i++ {
		port := pm.startPort + i
		if _, busy := pm.inUse[port]; !busy {
			pm.inUse[port] = struct{}{}
			return port, nil
		}
	}
	return 0, fmt.Errorf("peer manager port pool exhausted (start=%d max=%d)", pm.startPort, pm.maxPorts)
}

func (pm *PeerManager) releasePort(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.inUse, port)
}

// OpenPeerConn establishes a libp2p tunnel to peerID under protocol
// /x/<appName>/1.0 (the protocol convention Rubix uses). The returned
// PeerConn carries the local socket and the manager handle needed to tear
// the forward down on Close.
//
// Mirrors rubixgoplatform/core/ipfsport/peer.go::PeerManager.OpenPeerConn.
// The second arg there is a transport-level address override that Rubix
// always passes empty; we drop it from the signature for clarity.
func (pm *PeerManager) OpenPeerConn(ctx context.Context, peerID, appName string) (*PeerConn, error) {
	if strings.TrimSpace(peerID) == "" {
		return nil, fmt.Errorf("empty peer ID")
	}
	if strings.TrimSpace(appName) == "" {
		appName = SyncAppName
	}

	port, err := pm.acquirePort()
	if err != nil {
		return nil, err
	}

	listenAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	target := "/p2p/" + peerID
	protocol := fmt.Sprintf("/x/%s%s/1.0", peerID, appName)

	// 1. Best-effort swarm-connect. The Rubix peer.go does this via direct +
	//    circuit fallback; we hand the same multiaddr to go-ipfs-api's
	//    swarm/connect endpoint and accept either success or a permission/
	//    already-connected style failure.
	connectCtx, connectCancel := context.WithTimeout(ctx, 20*time.Second)
	defer connectCancel()
	if err := pm.ipfs.SwarmConnect(connectCtx, target); err != nil {
		// Don't hard-fail here: an already-known peer often returns an
		// idempotent error string from the IPFS daemon. The real test is
		// whether Forward succeeds.
		log.Printf("[Sync] swarm connect to %s warned: %v (continuing)", peerID, err)
	}

	// 2. Tear down any stale forward on this listen address (e.g. leftover
	//    from a prior process incarnation that crashed without closing).
	teardownCtx, teardownCancel := context.WithTimeout(ctx, 5*time.Second)
	_ = pm.ipfs.Close(teardownCtx, listenAddr)
	teardownCancel()

	// 3. Create the forward.
	forwardCtx, forwardCancel := context.WithTimeout(ctx, 20*time.Second)
	defer forwardCancel()
	if err := pm.ipfs.Forward(forwardCtx, protocol, listenAddr, target); err != nil {
		pm.releasePort(port)
		return nil, fmt.Errorf("p2p forward %s -> %s on %s: %w", listenAddr, target, protocol, err)
	}

	return &PeerConn{
		pm:         pm,
		peerID:     peerID,
		appName:    appName,
		protocol:   protocol,
		listenAddr: listenAddr,
		baseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		port:       port,
		closeFunc:  pm.releasePort,
	}, nil
}

// PeerConn is one libp2p tunnel to a specific fullnode peer. Mirrors the
// Rubix peer.go Peer struct surface that callers care about
// (SendJSONRequest + Close).
type PeerConn struct {
	pm         *PeerManager
	peerID     string
	appName    string
	protocol   string
	listenAddr string
	baseURL    string
	port       int
	closed     atomic.Bool
	closeFunc  func(int)
}

// PeerID returns the fullnode peer ID this connection is bound to.
func (p *PeerConn) PeerID() string { return p.peerID }

// SendJSONRequest issues an HTTP request over the libp2p tunnel. The wire
// protocol underneath is HTTP-over-libp2p; the local TCP socket is just the
// IPFS daemon's bridge.
//
// `query` mirrors the Rubix peer.go signature: pass nil if no query string.
// `body` is JSON-encoded if non-nil; pass nil for GET-shaped requests.
// `out` is the destination for the decoded JSON response body; pass nil to
// discard.
// `gzipped` toggles Accept-Encoding: gzip; left simple here since the
// fullnode sync endpoint isn't currently gzipping responses.
func (p *PeerConn) SendJSONRequest(method, path string, query map[string]string, body any, out any, gzipped bool) error {
	if p.closed.Load() {
		return fmt.Errorf("peer connection to %s is closed", p.peerID)
	}

	url := p.baseURL + path
	if len(query) > 0 {
		first := true
		var b strings.Builder
		b.WriteString(url)
		for k, v := range query {
			if first {
				b.WriteString("?")
				first = false
			} else {
				b.WriteString("&")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
		}
		url = b.String()
	}

	var reqBody *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	httpReq, err := http.NewRequest(strings.ToUpper(method), url, reqBody)
	if err != nil {
		return fmt.Errorf("build http request: %w", err)
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if gzipped {
		httpReq.Header.Set("Accept-Encoding", "gzip")
	}

	resp, err := p.pm.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send to %s (peer=%s): %w", path, p.peerID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> HTTP %d (peer=%s)", method, path, resp.StatusCode, p.peerID)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response from %s: %w", path, err)
	}
	return nil
}

// Close tears down the p2p forward and releases the local port back to the
// PeerManager pool. Idempotent: safe to call from a defer.
func (p *PeerConn) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := p.pm.ipfs.Close(ctx, p.listenAddr)
	p.closeFunc(p.port)
	if err != nil {
		log.Printf("[Sync] close forward for peer=%s: %v", p.peerID, err)
		return err
	}
	return nil
}

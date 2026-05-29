package sync

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"explorer-server/model"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	ipfsapi "github.com/ipfs/go-ipfs-api"
)

const (
	// HTTP path the fullnode serves at — same as the proxy days, but now reached
	// via the libp2p p2p-forward bridge instead of a public proxy.
	syncTokenChainPath = "/rubix/v1/fullnode/sync-token-chain"

	// Libp2p protocol ID the fullnode registers (core/ipfsport/listener.go in
	// rubixgoplatform branch vaishnav/fullnode-sync-API-for-explorer).
	syncProtocolID = "/x/RubixFullnodeSyncAPI/1.0"

	// Retry budgets (total attempts = initial + retries).
	maxAttemptsUnreachable = 4 // fullnode unreachable / network / 502
	maxAttempts500         = 2 // proxy/fullnode 500
	maxAttemptsFullnode    = 2 // 200 + status:false transient
)

// Fullnode-side messages that indicate a request-side bug — NEVER retry.
var permanentFullnodeMessages = []string{
	"max 50 token ids per request",
	"token_ids cannot be empty",
}

// StreamForwarder abstracts the IPFS shell's p2p forwarding so tests can fake it.
// The production implementation wraps *ipfsapi.Shell.
type StreamForwarder interface {
	// Forward sets up a local TCP listener that bridges to the target peer's
	// libp2p stream protocol. listenAddr and targetAddr are multiaddrs:
	//   listenAddr: /ip4/127.0.0.1/tcp/<port>
	//   targetAddr: /p2p/<peerID>
	Forward(ctx context.Context, protocol, listenAddr, targetAddr string) error

	// Close tears down any p2p forward matching listenAddr.
	Close(ctx context.Context, listenAddr string) error
}

// shellForwarder wraps go-ipfs-api Shell to satisfy StreamForwarder.
type shellForwarder struct {
	shell *ipfsapi.Shell
}

// NewShellForwarder constructs a StreamForwarder backed by the local IPFS daemon
// reachable at ipfsHost (e.g. "127.0.0.1:5001"). The daemon must have
// Experimental.Libp2pStreamMounting=true — the explorer's IPFSManager sets this.
func NewShellForwarder(ipfsHost string) StreamForwarder {
	return &shellForwarder{shell: ipfsapi.NewShell(ipfsHost)}
}

func (f *shellForwarder) Forward(ctx context.Context, protocol, listenAddr, targetAddr string) error {
	resp, err := f.shell.Request("p2p/forward", protocol, listenAddr, targetAddr).Send(ctx)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func (f *shellForwarder) Close(ctx context.Context, listenAddr string) error {
	resp, err := f.shell.Request("p2p/close").Option("listen-address", listenAddr).Send(ctx)
	if err != nil {
		return err
	}
	defer resp.Close()
	if resp.Error != nil {
		// "no rules to close" is fine — the listener wasn't set up.
		if strings.Contains(strings.ToLower(resp.Error.Error()), "no rules") {
			return nil
		}
		return resp.Error
	}
	return nil
}

// TokenChainClient fetches token chains from a fullnode peer over a libp2p bridge.
// It uses IPFS's p2p-forward to expose the fullnode's HTTP handler on a local TCP
// port, then makes a normal HTTP request to that port.
type TokenChainClient struct {
	ipfs       StreamForwarder
	resolver   PeerResolver
	listenAddr string // /ip4/127.0.0.1/tcp/<port>
	bridgeURL  string // http://127.0.0.1:<port>
	httpClient *http.Client
	timeout    time.Duration

	mu             sync.Mutex
	bridgeReady    bool
	fullnodePeerID string // resolved lazily on first ensureBridge
}

// NewTokenChainClient builds a client that bridges via the given IPFS shell,
// resolves the fullnode peer ID via the supplied resolver, and serves the
// local end of the bridge on 127.0.0.1:localPort.
func NewTokenChainClient(ipfs StreamForwarder, resolver PeerResolver, localPort int, timeout time.Duration) *TokenChainClient {
	return &TokenChainClient{
		ipfs:       ipfs,
		resolver:   resolver,
		listenAddr: fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", localPort),
		bridgeURL:  fmt.Sprintf("http://127.0.0.1:%d", localPort),
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
	}
}

// ensureBridge lazily resolves the fullnode peer ID and sets up the p2p forward.
// Idempotent within a process — the first call closes any stale forward on the
// listen address, then creates a fresh one. Subsequent calls are no-ops.
func (c *TokenChainClient) ensureBridge() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bridgeReady {
		return nil
	}
	if c.resolver == nil {
		return fmt.Errorf("no peer resolver configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	peerID, err := c.resolver.ResolvePeerID(ctx)
	if err != nil {
		return fmt.Errorf("resolve fullnode peer ID: %w", err)
	}
	c.fullnodePeerID = peerID
	log.Printf("[TokenSync] Resolved fullnode peer ID: %s", peerID)

	// Best-effort: tear down any stale forward on this listen address (e.g. from
	// a previous run of this process). We don't care if there was nothing to close.
	_ = c.ipfs.Close(ctx, c.listenAddr)

	target := "/p2p/" + peerID
	if err := c.ipfs.Forward(ctx, syncProtocolID, c.listenAddr, target); err != nil {
		return fmt.Errorf("p2p forward %s -> %s on %s: %w", c.listenAddr, target, syncProtocolID, err)
	}
	log.Printf("[TokenSync] Bridge ready: %s -> %s on protocol %s", c.listenAddr, target, syncProtocolID)
	c.bridgeReady = true
	return nil
}

// FetchTokenChains POSTs token_ids over the libp2p bridge and returns parsed chains.
func (c *TokenChainClient) FetchTokenChains(tokenIDs []string) (map[string][]model.SyncedTxn, error) {
	if len(tokenIDs) == 0 {
		return nil, nil
	}

	if err := c.ensureBridge(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(model.SyncTokenChainRequest{TokenIDs: tokenIDs})
	if err != nil {
		return nil, err
	}

	url := c.bridgeURL + syncTokenChainPath
	maxAttempts := maxAttemptsUnreachable // start optimistic; shrinks per error type
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(1<<uint(attempt-2)) * time.Second
			log.Printf("[TokenSync] Retrying in %s (attempt %d/%d, batch=%d, last err: %v)",
				backoff, attempt, maxAttempts, len(tokenIDs), lastErr)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fullnode unreachable: %w", err)
			log.Printf("[TokenSync] Bridge/network error on attempt %d/%d (batch=%d, url=%s): %v",
				attempt, maxAttempts, len(tokenIDs), url, err)
			continue
		}

		parsed, err := c.handleResponse(resp, url, len(tokenIDs))
		resp.Body.Close()

		if err == nil {
			return parsed, nil
		}

		var retryable *retryableProxyError
		if errors.As(err, &retryable) {
			if retryable.maxAttempts < maxAttempts {
				maxAttempts = retryable.maxAttempts
			}
			if attempt < maxAttempts {
				lastErr = err
				continue
			}
		}
		return nil, err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("sync-token-chain failed after retries")
}

// retryableProxyError signals the caller may retry. maxAttempts caps the total tries.
// "Proxy" in the name is historic — there is no proxy any more, but the error type
// is shared between "fullnode/bridge unreachable" and "fullnode returned 5xx".
type retryableProxyError struct {
	status      int
	maxAttempts int
	msg         string
}

func (e *retryableProxyError) Error() string {
	if e.msg != "" {
		return fmt.Sprintf("fullnode returned %d: %s", e.status, e.msg)
	}
	return fmt.Sprintf("fullnode returned %d", e.status)
}

// permanentClientError signals a request-side bug that retrying cannot fix
// (e.g. batch too large, empty token_ids).
type permanentClientError struct {
	msg string
}

func (e *permanentClientError) Error() string {
	return fmt.Sprintf("permanent client error: %s", e.msg)
}

func (c *TokenChainClient) handleResponse(resp *http.Response, url string, batchSize int) (map[string][]model.SyncedTxn, error) {
	switch resp.StatusCode {
	case http.StatusInternalServerError:
		body, _ := readMaybeGzip(resp)
		log.Printf("[TokenSync] Fullnode internal error (HTTP 500, batch=%d): %s",
			batchSize, truncate(string(body), 256))
		return nil, &retryableProxyError{status: 500, maxAttempts: maxAttempts500, msg: truncate(string(body), 256)}

	case http.StatusBadGateway:
		log.Printf("[TokenSync] Fullnode unavailable via bridge (HTTP 502, batch=%d)", batchSize)
		return nil, &retryableProxyError{status: 502, maxAttempts: maxAttemptsUnreachable}
	}

	body, err := readMaybeGzip(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sync-token-chain HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var payload model.SyncTokenChainResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode sync response: %w", err)
	}

	if !payload.Status {
		msg := payload.Message
		if msg == "" {
			msg = "unknown error"
		}
		if isPermanentFullnodeError(msg) {
			log.Printf("[TokenSync] BUG: fullnode rejected request (batch=%d): %s — will not retry", batchSize, msg)
			return nil, &permanentClientError{msg: msg}
		}
		log.Printf("[TokenSync] Fullnode error (batch=%d): %s — retryable", batchSize, msg)
		return nil, &retryableProxyError{status: 200, maxAttempts: maxAttemptsFullnode, msg: msg}
	}

	if payload.Result == nil {
		return map[string][]model.SyncedTxn{}, nil
	}
	return payload.Result, nil
}

func isPermanentFullnodeError(msg string) bool {
	low := strings.ToLower(msg)
	for _, p := range permanentFullnodeMessages {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

func readMaybeGzip(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(reader)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

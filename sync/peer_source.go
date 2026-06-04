package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// DefaultFullnodePeersURL is the canonical registry of fullnode peer IDs
// published by the Rubix team. Operators can override via
// TOKEN_SYNC_FULLNODE_PEERS_URL (e.g. a private fork).
//
// The file carries only peer_id (no IPs, no multiaddrs) — that's the only
// identifier the libp2p stack needs.
const DefaultFullnodePeersURL = "https://raw.githubusercontent.com/rubixchain/assets/refs/heads/main/fullnodes.json"

type fullnodePeer struct {
	PeerID string `json:"peer_id"`
	Status string `json:"status"`
}

type fullnodesRegistry struct {
	Mainnet []fullnodePeer `json:"mainnet"`
	Testnet []fullnodePeer `json:"testnet"`
}

// PeerSource fetches the registry and picks one active fullnode peer ID for a
// sync run. The same peer must be reused across the run so total_pages stays
// stable — see PickActivePeer.
type PeerSource struct {
	URL     string
	Network string // "mainnet" or "testnet"
	Client  *http.Client
	Rand    *rand.Rand // optional; tests inject a deterministic source
}

// NewPeerSource builds a PeerSource with sensible defaults. URL defaults to
// DefaultFullnodePeersURL when empty; Network defaults to "mainnet".
func NewPeerSource(url, network string) *PeerSource {
	if url == "" {
		url = DefaultFullnodePeersURL
	}
	if network == "" {
		network = "mainnet"
	}
	return &PeerSource{
		URL:     url,
		Network: network,
		Client:  &http.Client{Timeout: 15 * time.Second},
		Rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PickActivePeer fetches the registry, filters to status="Active" entries for
// the configured network, and returns one peer ID picked at random.
// Caller pins to this peer for the entire sync run.
func (p *PeerSource) PickActivePeer(ctx context.Context) (string, error) {
	peers, err := p.listActivePeers(ctx)
	if err != nil {
		return "", err
	}
	if len(peers) == 0 {
		return "", fmt.Errorf("no active fullnode peers in %s", p.Network)
	}
	return peers[p.Rand.Intn(len(peers))], nil
}

func (p *PeerSource) listActivePeers(ctx context.Context) ([]string, error) {
	if p.URL == "" {
		return nil, fmt.Errorf("peer registry URL is empty")
	}
	network := strings.ToLower(strings.TrimSpace(p.Network))
	if network == "" {
		network = "mainnet"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build registry request: %w", err)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch registry %s: %w", p.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry %s returned HTTP %d", p.URL, resp.StatusCode)
	}

	var reg fullnodesRegistry
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, fmt.Errorf("decode registry: %w", err)
	}

	var src []fullnodePeer
	switch network {
	case "testnet":
		src = reg.Testnet
	case "mainnet":
		src = reg.Mainnet
	default:
		return nil, fmt.Errorf("unknown network %q (use mainnet or testnet)", network)
	}

	active := make([]string, 0, len(src))
	for _, peer := range src {
		if strings.EqualFold(strings.TrimSpace(peer.Status), "Active") && strings.TrimSpace(peer.PeerID) != "" {
			active = append(active, strings.TrimSpace(peer.PeerID))
		}
	}
	return active, nil
}

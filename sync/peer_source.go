package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultFullnodePeersURL is the canonical registry of fullnode peer IDs
// published by the Rubix team. Operators can override via
// TOKEN_SYNC_FULLNODE_PEERS_URL (e.g. a private fork).
const DefaultFullnodePeersURL = "https://raw.githubusercontent.com/rubixchain/assets/refs/heads/main/fullnodes.json"

// PeerResolver returns a fullnode peer ID to dial. Implementations:
//   - StaticPeerResolver: returns a pre-configured ID (env var)
//   - RegistryPeerResolver: fetches the registry JSON, picks first Active peer
//
// Resolution is intentionally lazy — the client doesn't call ResolvePeerID
// until the first sync request, so a -sync run that finds no flagged tokens
// never hits the network.
type PeerResolver interface {
	ResolvePeerID(ctx context.Context) (string, error)
}

// StaticPeerResolver returns a hard-coded peer ID. Used when an operator
// pins a specific fullnode via TOKEN_SYNC_FULLNODE_PEER_ID.
type StaticPeerResolver struct {
	PeerID string
}

func (s StaticPeerResolver) ResolvePeerID(_ context.Context) (string, error) {
	id := strings.TrimSpace(s.PeerID)
	if id == "" {
		return "", fmt.Errorf("static peer ID is empty")
	}
	return id, nil
}

// RegistryPeerResolver fetches a JSON registry of fullnode peer IDs grouped by
// network and returns the first Active peer for the configured network.
//
// Expected registry shape (matches https://raw.githubusercontent.com/rubixchain/assets/refs/heads/main/fullnodes.json):
//
//	{
//	  "mainnet": [{"peer_id": "12D3Koo...", "status": "Active"}, ...],
//	  "testnet": [{"peer_id": "12D3Koo...", "status": "Active"}, ...]
//	}
type RegistryPeerResolver struct {
	URL     string
	Network string // "mainnet" or "testnet"
	Client  *http.Client
}

type fullnodePeer struct {
	PeerID string `json:"peer_id"`
	Status string `json:"status"`
}

type fullnodesRegistry struct {
	Mainnet []fullnodePeer `json:"mainnet"`
	Testnet []fullnodePeer `json:"testnet"`
}

func (r RegistryPeerResolver) ResolvePeerID(ctx context.Context) (string, error) {
	if r.URL == "" {
		return "", fmt.Errorf("registry URL is empty")
	}
	network := strings.ToLower(strings.TrimSpace(r.Network))
	if network == "" {
		network = "mainnet"
	}

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return "", fmt.Errorf("build registry request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch registry %s: %w", r.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry %s returned HTTP %d", r.URL, resp.StatusCode)
	}

	var reg fullnodesRegistry
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return "", fmt.Errorf("decode registry: %w", err)
	}

	var peers []fullnodePeer
	switch network {
	case "testnet":
		peers = reg.Testnet
	case "mainnet":
		peers = reg.Mainnet
	default:
		return "", fmt.Errorf("unknown network %q (use mainnet or testnet)", network)
	}

	for _, p := range peers {
		if strings.EqualFold(strings.TrimSpace(p.Status), "Active") && strings.TrimSpace(p.PeerID) != "" {
			return strings.TrimSpace(p.PeerID), nil
		}
	}
	return "", fmt.Errorf("no active fullnode peers listed for network %s in registry %s", network, r.URL)
}

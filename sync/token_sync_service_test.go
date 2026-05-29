package sync

import (
	"testing"
	"time"
)

func setEnvs(t *testing.T, envs map[string]string) {
	t.Helper()
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func TestConfigFromEnv_DefaultsToRegistry(t *testing.T) {
	// No explicit peer ID and no explicit registry URL — should default to
	// DefaultFullnodePeersURL and network=mainnet.
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":   "",
		"TOKEN_SYNC_FULLNODE_PEERS_URL": "",
		"TOKEN_SYNC_ENABLED":            "true",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success with defaults (registry URL is built-in)")
	}
	if cfg.FullnodePeerID != "" {
		t.Errorf("expected empty FullnodePeerID, got %q", cfg.FullnodePeerID)
	}
	if cfg.PeersURL != DefaultFullnodePeersURL {
		t.Errorf("expected DefaultFullnodePeersURL, got %q", cfg.PeersURL)
	}
	if cfg.Network != "mainnet" {
		t.Errorf("expected mainnet, got %q", cfg.Network)
	}
}

func TestConfigFromEnv_TestnetFlagSwitchesNetwork(t *testing.T) {
	setEnvs(t, map[string]string{"TOKEN_SYNC_FULLNODE_PEER_ID": ""})
	cfg, ok := ConfigFromEnv(true)
	if !ok {
		t.Fatal("expected success")
	}
	if cfg.Network != "testnet" {
		t.Errorf("testnet flag should set Network=testnet, got %q", cfg.Network)
	}
}

func TestConfigFromEnv_ExplicitPeerIDOverridesRegistry(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":   "12D3KooWPinned",
		"TOKEN_SYNC_FULLNODE_PEERS_URL": "https://example.com/peers.json",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success")
	}
	if cfg.FullnodePeerID != "12D3KooWPinned" {
		t.Errorf("expected pinned peer ID, got %q", cfg.FullnodePeerID)
	}
	// resolverFor should choose StaticPeerResolver when FullnodePeerID is set.
	r := resolverFor(cfg)
	if _, ok := r.(StaticPeerResolver); !ok {
		t.Errorf("expected StaticPeerResolver when peer ID is pinned, got %T", r)
	}
}

func TestConfigFromEnv_NoPeerIDUsesRegistryResolver(t *testing.T) {
	setEnvs(t, map[string]string{"TOKEN_SYNC_FULLNODE_PEER_ID": ""})
	cfg, ok := ConfigFromEnv(true)
	if !ok {
		t.Fatal("expected success")
	}
	r := resolverFor(cfg)
	reg, ok := r.(RegistryPeerResolver)
	if !ok {
		t.Fatalf("expected RegistryPeerResolver, got %T", r)
	}
	if reg.Network != "testnet" || reg.URL != DefaultFullnodePeersURL {
		t.Errorf("registry resolver wrong: %+v", reg)
	}
}

func TestConfigFromEnv_CustomRegistryURL(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":   "",
		"TOKEN_SYNC_FULLNODE_PEERS_URL": "https://my.org/peers.json",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success")
	}
	if cfg.PeersURL != "https://my.org/peers.json" {
		t.Errorf("expected custom URL, got %q", cfg.PeersURL)
	}
}

func TestConfigFromEnv_Disabled(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID": "12D3KooWAbc",
		"TOKEN_SYNC_ENABLED":          "false",
	})
	if _, ok := ConfigFromEnv(false); ok {
		t.Error("expected false when TOKEN_SYNC_ENABLED=false")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":  "12D3KooWAbc",
		"TOKEN_SYNC_LOCAL_BRIDGE_PORT": "",
		"TOKEN_SYNC_INTERVAL":          "",
		"TOKEN_SYNC_HTTP_TIMEOUT":      "",
		"TOKEN_SYNC_TOKEN_LIMIT":       "",
		"TOKEN_SYNC_BATCH_DELAY":       "",
		"TOKEN_SYNC_ENABLED":           "",
		"IPFS_HOST":                    "",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success with only TOKEN_SYNC_FULLNODE_PEER_ID set")
	}
	if cfg.LocalBridgePort != defaultLocalBridgePort {
		t.Errorf("bridge port default: got %d", cfg.LocalBridgePort)
	}
	if cfg.IPFSHost != defaultIPFSHost {
		t.Errorf("ipfs host default: got %s", cfg.IPFSHost)
	}
	if cfg.Interval != defaultSyncInterval {
		t.Errorf("interval default: got %s", cfg.Interval)
	}
	if cfg.HTTPTimeout != defaultHTTPTimeout {
		t.Errorf("timeout default: got %s", cfg.HTTPTimeout)
	}
	if cfg.BatchLimit != defaultBatchLimit {
		t.Errorf("limit default: got %d", cfg.BatchLimit)
	}
	if cfg.BatchDelay != defaultBatchDelay {
		t.Errorf("batch delay default: got %s", cfg.BatchDelay)
	}
}

func TestConfigFromEnv_CustomValues(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":  "12D3KooWXyz",
		"TOKEN_SYNC_LOCAL_BRIDGE_PORT": "7777",
		"TOKEN_SYNC_INTERVAL":          "10m",
		"TOKEN_SYNC_HTTP_TIMEOUT":      "90s",
		"TOKEN_SYNC_TOKEN_LIMIT":       "100",
		"TOKEN_SYNC_BATCH_DELAY":       "30s",
		"IPFS_HOST":                    "127.0.0.1:5005",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success")
	}
	if cfg.LocalBridgePort != 7777 {
		t.Errorf("custom port: got %d", cfg.LocalBridgePort)
	}
	if cfg.IPFSHost != "127.0.0.1:5005" {
		t.Errorf("custom IPFS host: got %s", cfg.IPFSHost)
	}
	if cfg.Interval != 10*time.Minute || cfg.HTTPTimeout != 90*time.Second ||
		cfg.BatchLimit != 100 || cfg.BatchDelay != 30*time.Second {
		t.Errorf("custom values not applied: %+v", cfg)
	}
}

func TestConfigFromEnv_InvalidPortFallsBackToDefault(t *testing.T) {
	setEnvs(t, map[string]string{
		"TOKEN_SYNC_FULLNODE_PEER_ID":  "12D3KooW",
		"TOKEN_SYNC_LOCAL_BRIDGE_PORT": "not-a-number",
	})
	cfg, ok := ConfigFromEnv(false)
	if !ok {
		t.Fatal("expected success")
	}
	if cfg.LocalBridgePort != defaultLocalBridgePort {
		t.Errorf("invalid port should fall back to default, got %d", cfg.LocalBridgePort)
	}
}

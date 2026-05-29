package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStaticPeerResolver_ReturnsTrimmedPeerID(t *testing.T) {
	r := StaticPeerResolver{PeerID: "  12D3KooWStatic  "}
	got, err := r.ResolvePeerID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12D3KooWStatic" {
		t.Errorf("expected trimmed value, got %q", got)
	}
}

func TestStaticPeerResolver_EmptyRejects(t *testing.T) {
	r := StaticPeerResolver{PeerID: "   "}
	if _, err := r.ResolvePeerID(context.Background()); err == nil {
		t.Error("expected error for empty static peer ID")
	}
}

const sampleRegistry = `{
  "mainnet": [
    {"peer_id": "12D3KooWMainOne", "status": "Inactive"},
    {"peer_id": "12D3KooWMainTwo", "status": "Active"},
    {"peer_id": "12D3KooWMainThree", "status": "Active"}
  ],
  "testnet": [
    {"peer_id": "12D3KooWTestOne", "status": "Active"}
  ]
}`

func startRegistry(t *testing.T, body string, status int) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	return srv.URL, srv.Close
}

func TestRegistryPeerResolver_MainnetPicksFirstActive(t *testing.T) {
	url, stop := startRegistry(t, sampleRegistry, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "mainnet"}
	got, err := r.ResolvePeerID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12D3KooWMainTwo" {
		t.Errorf("expected first Active mainnet peer (Two), got %q", got)
	}
}

func TestRegistryPeerResolver_TestnetPicksOnlyActive(t *testing.T) {
	url, stop := startRegistry(t, sampleRegistry, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "testnet"}
	got, err := r.ResolvePeerID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12D3KooWTestOne" {
		t.Errorf("expected testnet peer, got %q", got)
	}
}

func TestRegistryPeerResolver_NetworkDefaultsToMainnet(t *testing.T) {
	url, stop := startRegistry(t, sampleRegistry, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: ""}
	got, err := r.ResolvePeerID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12D3KooWMainTwo" {
		t.Errorf("empty network should default to mainnet, got %q", got)
	}
}

func TestRegistryPeerResolver_CaseInsensitiveStatusAndNetwork(t *testing.T) {
	body := `{"mainnet":[{"peer_id":"12D3KooWMain","status":"active"}]}`
	url, stop := startRegistry(t, body, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "MAINNET"}
	got, err := r.ResolvePeerID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12D3KooWMain" {
		t.Errorf("got %q", got)
	}
}

func TestRegistryPeerResolver_NoActivePeersErrors(t *testing.T) {
	body := `{"testnet":[{"peer_id":"X","status":"Inactive"}]}`
	url, stop := startRegistry(t, body, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "testnet"}
	_, err := r.ResolvePeerID(context.Background())
	if err == nil {
		t.Fatal("expected error when no active peers")
	}
	if !strings.Contains(err.Error(), "no active") {
		t.Errorf("error should mention no active peers, got: %v", err)
	}
}

func TestRegistryPeerResolver_UnknownNetworkErrors(t *testing.T) {
	url, stop := startRegistry(t, sampleRegistry, http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "weirdnet"}
	_, err := r.ResolvePeerID(context.Background())
	if err == nil {
		t.Fatal("expected error for unknown network")
	}
	if !strings.Contains(err.Error(), "unknown network") {
		t.Errorf("error should call out unknown network, got: %v", err)
	}
}

func TestRegistryPeerResolver_HTTPErrorPropagated(t *testing.T) {
	url, stop := startRegistry(t, "not json", http.StatusServiceUnavailable)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "mainnet"}
	_, err := r.ResolvePeerID(context.Background())
	if err == nil {
		t.Fatal("expected error on 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestRegistryPeerResolver_MalformedJSONErrors(t *testing.T) {
	url, stop := startRegistry(t, "{not valid json", http.StatusOK)
	defer stop()

	r := RegistryPeerResolver{URL: url, Network: "mainnet"}
	_, err := r.ResolvePeerID(context.Background())
	if err == nil {
		t.Fatal("expected JSON decode error")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode, got: %v", err)
	}
}

func TestRegistryPeerResolver_EmptyURLErrors(t *testing.T) {
	r := RegistryPeerResolver{URL: "", Network: "mainnet"}
	_, err := r.ResolvePeerID(context.Background())
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestRegistryPeerResolver_RespectsContextDeadline(t *testing.T) {
	// Slow server that takes longer than the context allows.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := RegistryPeerResolver{URL: srv.URL, Network: "mainnet"}
	_, err := r.ResolvePeerID(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

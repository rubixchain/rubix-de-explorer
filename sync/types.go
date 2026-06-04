// Package sync implements the explorer's consumer for the Rubix fullnode's
// sync-txn-info-chain libp2p API.
//
// Wire format (POST /rubix/v1/fullnode/sync-txn-info-chain):
//
//	request:
//	  token_ids        []string                 (≤50, deduped, non-empty)
//	  known_positions  map[tokenID]{position, transaction_id}  (optional)
//	  page_number      int                      (1-indexed, omit/0 → 1)
//	  page_size        int                      (default 100, max 1000;
//	                                             MUST stay constant across
//	                                             a sync run for total_pages
//	                                             to remain stable)
//
//	response (BasicResponse-wrapped):
//	  status           bool
//	  message          string
//	  result:
//	    data            map[tokenID][]ChainEntry  (per-token, position asc)
//	    divergent_tokens []string                 (tokens whose claimed
//	                                                position+txID didn't
//	                                                match the fullnode — full
//	                                                chain sent from 0)
//	    page_number     int
//	    total_pages     int
//	    page_size       int
//	    total_items     int
//
// The connection itself travels over a libp2p p2p-forward tunnel keyed by the
// fullnode's peer ID; transport details live in client.go.
package sync

import (
	"encoding/json"

	"explorer-server/model"
)

// SyncTxnInfoChainRequest is the body sent to
// POST /rubix/v1/fullnode/sync-txn-info-chain.
type SyncTxnInfoChainRequest struct {
	TokenIDs       []string                  `json:"token_ids"`
	KnownPositions map[string]KnownPosition  `json:"known_positions,omitempty"`
	PageNumber     int                       `json:"page_number,omitempty"`
	PageSize       int                       `json:"page_size,omitempty"`
}

// KnownPosition tells the server "I already have token X up to position P,
// and the transaction at that position is T." The server uses this to:
//   - skip entries the explorer already has (efficiency)
//   - detect chain divergence by comparing its tx at position P against T
//
// Tokens omitted from the map are treated as "no local history" — the server
// returns the full chain from position 0.
type KnownPosition struct {
	Position      int64  `json:"position"`
	TransactionID string `json:"transaction_id"`
}

// ChainEntry is one row of a token's chain as returned by the fullnode.
// Position is the authoritative ordering on the fullnode side and forms the
// (token_id, position) primary key in the explorer's sync mirror.
type ChainEntry struct {
	ID                    string          `json:"id"`
	Role                  int16           `json:"role"`
	Position              int64           `json:"position"`
	PreviousTransactionID string          `json:"previous_transaction_id"`
	Info                  json.RawMessage `json:"info"`
}

// ParseInfo unmarshals the raw info bytes into *model.TransactionInfo.
// Returns nil, nil when info is JSON null or absent. Returns an error if the
// bytes don't fit the TransactionInfo shape — callers decide whether to drop
// the entry (per-entry tolerance) or treat the whole page as poisoned.
func (e *ChainEntry) ParseInfo() (*model.TransactionInfo, error) {
	if len(e.Info) == 0 {
		return nil, nil
	}
	trimmed := jsonTrim(e.Info)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var ti model.TransactionInfo
	if err := json.Unmarshal(e.Info, &ti); err != nil {
		return nil, err
	}
	return &ti, nil
}

// SyncTxnInfoChainResult is the body of result inside the BasicResponse envelope.
type SyncTxnInfoChainResult struct {
	Data            map[string][]ChainEntry `json:"data"`
	DivergentTokens []string                `json:"divergent_tokens"`
	PageNumber      int                     `json:"page_number"`
	TotalPages      int                     `json:"total_pages"`
	PageSize        int                     `json:"page_size"`
	TotalItems      int                     `json:"total_items"`
}

// SyncTxnInfoChainResponse is the full BasicResponse envelope.
type SyncTxnInfoChainResponse struct {
	Status  bool                   `json:"status"`
	Message string                 `json:"message"`
	Result  SyncTxnInfoChainResult `json:"result"`
}

// jsonTrim strips leading/trailing ASCII whitespace from a raw JSON message
// so the "is this null?" check doesn't trip over decoder padding.
func jsonTrim(b []byte) []byte {
	start := 0
	end := len(b)
	for start < end && isASCIIWhitespace(b[start]) {
		start++
	}
	for end > start && isASCIIWhitespace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isASCIIWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

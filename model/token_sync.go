package model

// SyncTokenChainRequest is the body for POST /rubix/v1/fullnode/sync-token-chain.
type SyncTokenChainRequest struct {
	TokenIDs []string `json:"token_ids"`
}

// SyncedTxn mirrors a single fullnode_tokenchain row enriched with TransactionInfo.
// PreviousTransactionID is sourced from the fullnode's tokenchain table (NOT Info),
// because unpledge entries reuse another transaction's Info and the unpledged token's
// previous-tx pointer isn't anywhere inside Info.
type SyncedTxn struct {
	ID                    string           `json:"id"`
	Role                  int16            `json:"role"`
	PreviousTransactionID string           `json:"previous_transaction_id"`
	Info                  *TransactionInfo `json:"info"`
}

// SyncTokenChainResponse matches rubixgoplatform BasicResponse for sync-token-chain.
// Result is keyed by tokenID; the slice is chronological for that token.
type SyncTokenChainResponse struct {
	Status  bool                   `json:"status"`
	Message string                 `json:"message"`
	Result  map[string][]SyncedTxn `json:"result"`
}

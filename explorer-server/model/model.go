package model

import (
	"explorer-server/database/models"
	"time"
)

// TxnAnalytics represents transaction analytics
type TxnAnalytics struct {
	IntervalStart time.Time `json:"interval_start" db:"interval_start"`
	IntervalEnd   time.Time `json:"interval_end" db:"interval_end"`
	TxnCount      int64     `json:"txn_count" db:"txn_count"`
	TotalValue    float64   `json:"total_value" db:"total_value"`
	TokenType     string    `json:"token_type" db:"token_type"`
}

// DatabaseHealth represents the health status of the database
type DatabaseHealth struct {
	IsConnected bool   `json:"is_connected"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type Token struct {
	TokenId    string  `json:"token_id"`
	OwnerDID   string  `json:"owner_did"`
	TokenValue float64 `json:"token_value"`
}

type TokenResponse struct {
	Tokens []Token `json:"tokens"`
}

type HolderResponse struct {
	OwnerDID   string `json:"owner_did"`
	TokenCount float64  `json:"token_count"`
	// TotalTransactions int64 `json:"total_transactions"`
}

type HoldersResponse struct {
	HoldersResponse []HolderResponse `json:"holders_response"`
}

type TransactionResponse struct {
	TxnHash string    `json:"txn_hash"`
	TxnType string    `json:"txn_type"`
	Amount  float64   `json:"amount"`
	Epoch   time.Time `json:"txn_time"`
	SenderDID   string    `json:"sender_did"`
	ReceiverDID string    `json:"receiver_did"`
}

type TransactionsResponse struct {
	TransactionsResponse []TransactionResponse `json:"transactions_response"`
}

// type SearchResponse struct {
// 	ResponseType  string `json:"response_type"`

// 	TotalRBTCount           int64           `json:"total_rbt_count"`
// 	RBTs                    []RBT           `json:"rbts"`
// 	TotalFTCount           int64           `json:"total_ft_count"`
// 	FTs                    []FT           `json:"fts"`
// 	TotalNFTCount          int64          `json:"total_nft_count"`
// 	NFTs                   []NFT          `json:"nfts"`
// 	TotalSmartContractCount int64           `json:"total_smart_contract_count"`
// 	SmartContracts         []SmartContract `json:"smart_contracts"`
// 	TotalDIDCount          int64          `json:"total_did_count"`
// 	DIDs                   []DIDs         `json:"dids"`
// }

// -----Token explorer response
type RbtResponse struct {
	RBT    models.RBT   `json:"rbt"`
	Blocks Block `json:"block"`
}

type Block struct {
	TxnHash   string    `json:"txn_hash"`
	BlockType int64     `json:"block_type"`
	Epoch     time.Time `json:"epoch"`
}

type TokenChainResponse struct {
	Blocks []Block `json:"blocks"`
}

type DIDResponse struct {
	DID  models.DIDs   `json:"did"`
	RBTs []models.RBT `json:"rbts"`
	// Trasactions []TransactionResponse `json:"transactions"`
}

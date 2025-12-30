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
	OwnerDID   string  `json:"owner_did"`
	TokenCount float64 `json:"token_count"`
	// TotalTransactions int64 `json:"total_transactions"`
}

type HoldersResponse struct {
	HoldersResponse []HolderResponse `json:"holders_response"`
	Count           int64            `json:"count"`
}

type TransactionResponse struct {
	TxnHash     string  `json:"txn_hash"`
	TxnType     string  `json:"txn_type"`
	Amount      float64 `json:"amount"`
	Epoch       *int64  `json:"txn_time"`
	SenderDID   string  `json:"sender_did"`
	ReceiverDID string  `json:"receiver_did"`
}

type TransactionsResponse struct {
	TransactionsResponse []TransactionResponse `json:"transactions_response"`
	Count                int64                 `json:"count"`
}

type SCBlocksListResponse struct {
	SC_Blocks []models.SC_Block `json:"sc_blocks"`
	Count     int64             `json:"count"`
}

type BurntBlocksListResponse struct {
	BurntBlocks []models.BurntBlocks `json:"burntblocks"`
	Count       int64                `json:"count"`
}

type RBTListResponse struct {
	Tokens []Token `json:"tokens"`
	Count  int64   `json:"count"`
}

// -----Token explorer response
type RbtResponse struct {
	RBT    models.RBT `json:"rbt"`
	Blocks Block      `json:"block"`
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
	DID  models.DIDs  `json:"did"`
	RBTs []models.RBT `json:"rbts"`
	// Trasactions []TransactionResponse `json:"transactions"`
}

type FailedToSyncTokenDetailsInfo struct {
	TokenID   string `gorm:"column:token_id;primaryKey"` // `gorm:"column:token;primaryKey"`
	TokenType int    `gorm:"column:token_type"`
	Did       string `gorm:"column:did"`
	AssetType int    `gorm:"column:asset_type"`
}

// TokenDetails represents the specific token information received from the full node
type TokenDetails struct {
	TokenID    string  `json:"token_id"`
	TokenType  int     `json:"token_type"`
	TokenValue float64 `json:"token_value"`
}

// IncomingBlockInfo represents what Explorer receives from fullnode
type IncomingBlockInfo struct {
	BlockHash         string                 `json:"block_hash"`
	TransactionID     string                 `json:"transaction_id"`
	TxnType           string                 `json:"transaction_type"`
	AssetType         int                    `json:"asset_type"`
	FTName            string                 `json:"ft_name"`
	CreatorDID        string                 `json:"creator_did"`
	PublisherDID      string                 `json:"publisher_did"`
	ReceiverDID       string                 `json:"receiver_did"`
	TxnBlock          map[string]interface{} `json:"block_map"`
	LatestBlockHeight uint64                 `json:"block_height"`
	TransactionValue  float64                `json:"transaction_value"`
	TokenValue        float64                `json:"token_value"`
	TokenDetails      []TokenDetails         `json:"token_details"`
}

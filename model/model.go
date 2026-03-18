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
	AssetType   string  `json:"asset_type"`
	Amount      float64 `json:"amount"`
	Epoch       *int64  `json:"txn_time"`
	SenderDID   string  `json:"sender_did"`
	ReceiverDID string  `json:"receiver_did"`
}

type TransactionsResponse struct {
	TransactionsResponse []TransactionResponse `json:"transactions_response"`
	Count                int64                 `json:"count"`
}

type RBTListResponse struct {
	Tokens []models.RBT `json:"tokens"`
	Count  int64        `json:"count"`
}

type SCListResponse struct {
	SCs   []models.SC `json:"scs"`
	Count int64       `json:"count"`
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
	LatestBlockHeight uint64                 `json:"block_height"`
	TransactionValue  float64                `json:"transaction_value"`
	TokenValue        float64                `json:"token_value"`
	BlockMap          map[string]interface{} `json:"block_map"`
	TokenDetails      []TokenDetails         `json:"token_details"`
}

// PubSubTxnInfo matches the Fullnode's published struct exactly.
type PubSubTxnInfo struct {
	BlockHash         string  `json:"block_hash"`
	TransactionID     string  `json:"transaction_id"`
	BlockType         string  `json:"block_type"`
	AssetType         int     `json:"asset_type"`
	FTName            string  `json:"ft_name"`
	CreatorDID        string  `json:"creator_did"`
	PublisherDID      string  `json:"publisher_did"`
	ReceiverDID       string  `json:"receiver_did"`
	TxnBlock          []byte  `json:"block"`
	LatestBlockHeight uint64  `json:"block_height"`
	TransactionValue  float64 `json:"transaction_value"`
	TokenValue        float64 `json:"token_value"`
}

// --- Transaction DTOs (Received from PubSub/Network, matches Rubix node format) ---

type TransactionInfo struct {
	Initiator       string             `json:"initiator"`
	Owner           string             `json:"owner"`
	Epoch           int                `json:"epoch"`
	Network         string             `json:"network"`
	Tokens          *TransactionTokens `json:"tokens"`
	CommittedTokens []*TokenInfo       `json:"committedTokens"`
	Quorums         []*QuorumInfo      `json:"quorums"`
	Memo            string             `json:"memo"`
}

type TransactionTokens struct {
	RBT           []*TokenInfo `json:"rbt"`
	NFT           []*TokenInfo `json:"nft"`
	FT            []*TokenInfo `json:"ft"`
	SmartContract []*TokenInfo `json:"smartContract"`
}

type TokenInfo struct {
	TokenID               string `json:"tokenId"`
	PreviousTransactionID string `json:"previousTransactionID"`
	Data                  string `json:"data"`
}

type QuorumInfo struct {
	Did    string       `json:"did"`
	Tokens []*TokenInfo `json:"tokens"`
}

type QuorumSignature struct {
	Did       string `json:"did"`
	Signature string `json:"signature"`
}

type Signature struct {
	InitiatorSignature string            `json:"initiatorSignature"`
	Quorums            []QuorumSignature `json:"quorums"`
}

type Transactions struct {
	TransactionID   string           `json:"transaction_id"`
	TransactionInfo *TransactionInfo `json:"transaction_info"`
	Signatures      *Signature       `json:"signatures"`
}

type EventTransaction struct {
	Transaction *Transactions `json:"transaction"`
	Status      bool          `json:"status"`
	Message     string        `json:"message"`
}

type FTGroup struct {
	FTName      string  `json:"ftName"`
	NumberOfFts float64 `json:"numberOfFts"`
	CreatorDID  string  `json:"creatorDID"`
}

type FTGroupResponse struct {
	FTGroups []FTGroup `json:"ftGroups"`
	Count    int64     `json:"count"`
}

package models

import (
	"encoding/json"
	"time"
)

// ==========================================
//   Explorer DB Models (Aligned with Rubix Node Schema)
// ==========================================

// Transactions stores the full transaction as JSON (matches Rubix node's Transactions table)
type Transactions struct {
	ID        string          `json:"id" gorm:"primaryKey;column:id"`
	Info      json.RawMessage `json:"info" gorm:"column:info;type:jsonb"`
	Signature json.RawMessage `json:"signature" gorm:"column:signature;type:jsonb"`
	CreatedAt time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (Transactions) TableName() string { return "Transactions" }

// EventTransaction stores the PubSub event wrapper (status + message for failed consensus)
type EventTransaction struct {
	TransactionID string    `json:"transaction_id" gorm:"primaryKey;column:transaction_id"`
	Status        bool      `json:"status" gorm:"column:status"`
	Message       string    `json:"message" gorm:"column:message"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (EventTransaction) TableName() string { return "EventTransactions" }

// TransactionInfo stores flattened transaction info fields for easy frontend filtering
type TransactionInfo struct {
	TransactionID   string          `json:"transaction_id" gorm:"primaryKey;column:transaction_id"`
	Initiator       string          `json:"initiator" gorm:"column:initiator;index"`
	Owner           string          `json:"owner" gorm:"column:owner;index"`
	Epoch           int             `json:"epoch" gorm:"column:epoch;index"`
	Network         string          `json:"network" gorm:"column:network"`
	Tokens          json.RawMessage `json:"tokens" gorm:"column:tokens;type:jsonb"`
	CommittedTokens json.RawMessage `json:"committedTokens" gorm:"column:committed_tokens;type:jsonb"`
	Quorums         json.RawMessage `json:"quorums" gorm:"column:quorums;type:jsonb"`
	Memo            string          `json:"memo" gorm:"column:memo"`
	Status          bool            `json:"status" gorm:"column:status;index"` // Persistent status field
	Amount          float64         `json:"amount" gorm:"column:amount"`       // Sum of values in Quorum tokens
	CreatedAt       time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (TransactionInfo) TableName() string { return "TransactionInfo" }

// FailedTransactionInfo mirrors TransactionInfo but for transactions that failed consensus
type FailedTransactionInfo struct {
	TransactionID   string          `json:"transaction_id" gorm:"primaryKey;column:transaction_id"`
	Initiator       string          `json:"initiator" gorm:"column:initiator;index"`
	Owner           string          `json:"owner" gorm:"column:owner;index"`
	Epoch           int             `json:"epoch" gorm:"column:epoch;index"`
	Network         string          `json:"network" gorm:"column:network"`
	Tokens          json.RawMessage `json:"tokens" gorm:"column:tokens;type:jsonb"`
	CommittedTokens json.RawMessage `json:"committedTokens" gorm:"column:committed_tokens;type:jsonb"`
	Quorums         json.RawMessage `json:"quorums" gorm:"column:quorums;type:jsonb"`
	Memo            string          `json:"memo" gorm:"column:memo"`
	Status          bool            `json:"status" gorm:"column:status;index"`
	Amount          float64         `json:"amount" gorm:"column:amount"`
	CreatedAt       time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (FailedTransactionInfo) TableName() string { return "FailedTransactionInfo" }

// Token (Unified model for RBT, FT, NFT, SC — aligned with Rubix node's Token table)
type Token struct {
	TokenID        string    `json:"token_id" gorm:"primaryKey;column:token_id"`
	ParentTokenID  string    `json:"parent_token_id" gorm:"column:parent_token_id"`
	TokenValue     float64   `json:"token_value" gorm:"column:token_value"`
	TokenStatus    int16     `json:"token_status" gorm:"column:token_status"`
	DID            string    `json:"did" gorm:"column:did;index"`
	TransactionID  string    `json:"transaction_id" gorm:"column:transaction_id"`
	TokenStateHash string    `json:"token_state_hash" gorm:"column:token_state_hash"`
	TokenType      int16     `json:"token_type" gorm:"column:token_type;index"`
	LatestPosition int64     `json:"latest_position" gorm:"column:latest_position"`
	LatestRole     int16     `json:"latest_role" gorm:"column:latest_role"`
	Data           string    `json:"data" gorm:"column:data"`                           // Metadata for NFTs/Smart Contracts
	DeployerDID    string    `json:"deployer" gorm:"column:deployer_did;index"`         // Original deployer/creator
	NeedsSync      bool      `json:"needs_sync" gorm:"column:needs_sync;default:false"` // Track missing history
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (Token) TableName() string { return "Tokens" }

// TokenChain tracks token provenance (aligned with Rubix node's TokenChain table)
type TokenChain struct {
	ID                    uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TokenID               string    `json:"token_id" gorm:"column:token_id;index"`
	TransactionID         string    `json:"transaction_id" gorm:"column:transaction_id;index"`
	Role                  int16     `json:"role" gorm:"column:role"`
	PreviousTransactionID string    `json:"previous_transaction_id" gorm:"column:previous_transaction_id;index"`
	CreatedAt             time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (TokenChain) TableName() string { return "TokenChain" }

// TokenChainArray stores the sequence of TokenChain record IDs for each token
type TokenChainArray struct {
	TokenID   string          `json:"token_id" gorm:"primaryKey;column:token_id"`
	Index     json.RawMessage `json:"index" gorm:"column:index;type:jsonb"` // JSON array of TokenChain.ID in sequence for a token.
	CreatedAt time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (TokenChainArray) TableName() string { return "TokenChainArray" }

// ==========================================
//   Explorer-Specific Models (Not in Rubix Node)
// ==========================================

// Enables fast "Top Holders" and "DID balance" queries.
type DIDBalance struct {
	DID        string    `json:"did" gorm:"primaryKey;column:did"`
	AssetType  string    `json:"asset_type" gorm:"primaryKey;column:asset_type"`
	TokenName  string    `json:"token_name" gorm:"primaryKey;column:token_name"`   // FT Name (empty for RBT/NFT/SC)
	CreatorDID string    `json:"creator_did" gorm:"primaryKey;column:creator_did"` // FT creator (empty for RBT/NFT/SC)
	TokenValue float64   `json:"token_value" gorm:"column:token_value"`            // FT Value (empty for RBT/NFT/SC)
	Balance    float64   `json:"balance" gorm:"column:balance"`
	PeerID     string    `json:"peer_id" gorm:"column:peer_id"`   // From rubix_did topic
	DIDAlgo    int64     `json:"did_algo" gorm:"column:did_algo"` // From rubix_did topic
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (DIDBalance) TableName() string { return "DIDBalances" }

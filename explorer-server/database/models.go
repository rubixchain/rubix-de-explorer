package database

import (
	"time"
)

// Token represents a token in the system
type Token struct {
	TokenID      string    `json:"token_id" db:"token_id"`
	TokenType    string    `json:"token_type" db:"token_type"`
	CurrentOwner string    `json:"current_owner" db:"current_owner"`
	State        string    `json:"state" db:"state"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Block represents a block/transaction in the system
type Block struct {
	BlockID       string    `json:"block_id" db:"block_id"`
	BlockHash     string    `json:"block_hash" db:"block_hash"`
	PrevBlockHash *string   `json:"prev_block_hash" db:"prev_block_hash"`
	SenderDID     *string   `json:"sender_did" db:"sender_did"`
	ReceiverDID   *string   `json:"receiver_did" db:"receiver_did"`
	TxnType       *string   `json:"txn_type" db:"txn_type"`
	Amount        *float64  `json:"amount" db:"amount"`
	TxnTime       time.Time `json:"txn_time" db:"txn_time"`
	Epoch         *int64    `json:"epoch" db:"epoch"`
	TimeTakenMs   *int64    `json:"time_taken_ms" db:"time_taken_ms"`
}

// TokenChain represents the mapping between tokens and blocks
type TokenChain struct {
	TokenID     string `json:"token_id" db:"token_id"`
	BlockHeight int64  `json:"block_height" db:"block_height"`
	BlockID     string `json:"block_id" db:"block_id"`
}

// DID represents a Decentralized Identifier
type DID struct {
	DIDID         string     `json:"did_id" db:"did_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	LastActive    *time.Time `json:"last_active" db:"last_active"`
	TotalBalance  float64    `json:"total_balance" db:"total_balance"`
	PledgedAmount float64    `json:"pledged_amount" db:"pledged_amount"`
}

// Validator represents a validator in the network
type Validator struct {
	ValidatorID   string     `json:"validator_id" db:"validator_id"`
	DIDID         string     `json:"did_id" db:"did_id"`
	ActiveSince   time.Time  `json:"active_since" db:"active_since"`
	LastSeen      *time.Time `json:"last_seen" db:"last_seen"`
	TotalPledged  float64    `json:"total_pledged" db:"total_pledged"`
	UptimePercent float64    `json:"uptime_percent" db:"uptime_percent"`
}

// Pledge represents a token pledge
type Pledge struct {
	PledgeID    int       `json:"pledge_id" db:"pledge_id"`
	TokenID     string    `json:"token_id" db:"token_id"`
	ValidatorID string    `json:"validator_id" db:"validator_id"`
	DIDID       string    `json:"did_id" db:"did_id"`
	Amount      float64   `json:"amount" db:"amount"`
	PledgedAt   time.Time `json:"pledged_at" db:"pledged_at"`
}

// BlockToken represents the mapping between blocks and tokens
type BlockToken struct {
	BlockID string   `json:"block_id" db:"block_id"`
	TokenID string   `json:"token_id" db:"token_id"`
	Amount  *float64 `json:"amount" db:"amount"`
}

// TokenStats represents aggregate token statistics
type TokenStats struct {
	TokenType     string    `json:"token_type" db:"token_type"`
	TotalTokens   int64     `json:"total_tokens" db:"total_tokens"`
	ActiveTokens  int64     `json:"active_tokens" db:"active_tokens"`
	PledgedTokens int64     `json:"pledged_tokens" db:"pledged_tokens"`
	LastUpdated   time.Time `json:"last_updated" db:"last_updated"`
}

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

package models

import (
	"time"

	"gorm.io/datatypes"
)

type TransferBlocks struct {
	BlockHash          string         `json:"block_hash" gorm:"primaryKey"`
	PrevBlockID        *string        `json:"prev_block_id"`
	SenderDID          *string        `json:"sender_did"`
	ReceiverDID        *string        `json:"receiver_did"`
	TxnType            *string        `json:"txn_type"`
	Amount             *float64       `json:"amount"`
	Epoch              *int64         `json:"epoch"`
	Tokens             datatypes.JSON `json:"tokens" gorm:"type:jsonb"`               // store []string as JSONB
	ValidatorPledgeMap datatypes.JSON `json:"validator_pledge_map" gorm:"type:jsonb"` // store map[string][]string as JSONB
	TxnID              *string        `json:"txn_id"`
}

// Token represents a token in the system
type TokenType struct {
	TokenID     string    `json:"token_id" db:"token_id"`
	TokenType   string    `json:"token_type" db:"token_type"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
}

type AllBlocks struct {
	BlockHash string    `json:"block_hash" db:"block_hash"`
	BlockType int64     `json:"block_type" db:"block_type"`
	Epoch     time.Time `json:"epoch" db:"epoch"`
	TxnID     string    `json:"txn_id" db:"txn_id"`
}

// type TransferBlocks struct {
// 	BlockHash          string               `json:"block_hash" db:"block_hash"`
// 	PrevBlockID        *string              `json:"prev_block_id" db:"prev_block_id"`
// 	SenderDID          *string              `json:"sender_did" db:"sender_did"`
// 	ReceiverDID        *string              `json:"receiver_did" db:"receiver_did"`
// 	TxnType            *string              `json:"txn_type" db:"txn_type"`
// 	Amount             *float64             `json:"amount" db:"amount"`
// 	TxnTime            time.Time            `json:"txn_time" db:"txn_time"`
// 	Epoch              *int64               `json:"epoch" db:"epoch"`
// 	TimeTakenMs        *int64               `json:"time_taken_ms" db:"time_taken_ms"`
// 	Tokens             []string             `json:"tokens" db:"tokens"`
// 	ValidatorPledgeMap *map[string][]string `json:"validator_pledge_map" db:"validator_pledge_map"`
// 	TxnID              *string              `json:"txn_id" db:"txn_id"`
// }

// // Block represents a block/transaction in the system
// type PledgeBlocks struct {
// 	BlockHash          string               `json:"block_hash" db:"block_hash"`
// 	BlockType          *string              `json:"block_type" db:"block_type"`
// 	Amount             *float64             `json:"amount" db:"amount"`
// 	TxnTime            time.Time            `json:"txn_time" db:"txn_time"`
// 	Epoch              *int64               `json:"epoch" db:"epoch"`
// 	TimeTakenMs        *int64               `json:"time_taken_ms" db:"time_taken_ms"`
// 	Tokens             []string             `json:"tokens" db:"tokens"`
// 	ValidatorPledgeMap *map[string][]string `json:"validator_pledge_map" db:"validator_pledge_map"`
// 	TxnID              *string              `json:"txn_id" db:"txn_id"`
// }

// type GenesisBlock struct {
// 	BlockHash   string    `json:"block_hash" db:"block_hash"`
// 	DeployerDID *string   `json:"deployer_did" db:"deployer_did"`
// 	ParentID    *string   `json:"parent_id" db:"parent_id"`
// 	TxnType     *string   `json:"txn_type" db:"txn_type"`
// 	Amount      *float64  `json:"amount" db:"amount"`
// 	TxnTime     time.Time `json:"txn_time" db:"txn_time"`
// 	Epoch       *int64    `json:"epoch" db:"epoch"`
// 	TimeTakenMs *int64    `json:"time_taken_ms" db:"time_taken_ms"`
// 	Tokens      []string  `json:"tokens" db:"tokens"`
// }

// type SmartContractBlocks struct {
// 	BlockHash string `json:"block_hash" db:"block_hash"`
// }

// type BurntBlocks struct {
// 	BlockHash string `json:"block_hash" db:"block_hash"`
// }

// type RemainingBlocks struct {
// 	BlockHash string `json:"block_hash" db:"block_hash"`
// }

// smart contract table
type SmartContract struct {
	ContractID  string `json:"contract_id" db:"contract_id"`
	BlockHash   string `json:"block_hash" db:"block_hash"`
	DeployerDID string `json:"deployer_did" db:"deployer_did"`
	TxnId       string `json:"txn_id" db:"txn_id"`
}

type RBT struct {
	TokenID     string  `json:"rbt_id" db:"rbt_id"`
	TokenValue  float64 `json:"token_value" db:"token_value"`
	OwnerDID    string  `json:"owner_did" db:"owner_did"`
	BlockID     string  `json:"block_id" db:"block_id"`
	BlockHeight string  `json:"block_height" db:"block_height"`
}

type FT struct {
	FtID        string  `json:"ft_id" db:"ft_id"`
	TokenValue  float64 `json:"token_value" db:"token_value"`
	FTName      string  `json:"ft_name" db:"ft_name"`
	OwnerDID    string  `json:"owner_did" db:"owner_did"`
	CreatorDID  string  `json:"creator_did" db:"creator_did"`
	BlockHeight string  `json:"block_height" db:"block_height"`
	BlockID     string  `json:"block_id" db:"block_id"`
	Txn_ID      string  `json:"txn_id" db:"txn_id"`
}

type NFT struct {
	TokenID    string `json:"nft_id" db:"nft_id"`
	TokenValue string `json:"token_value" db:"token_value"`
	OwnerDID   string `json:"owner_did" db:"owner_did"`
	BlockHash  string `json:"block_hash" db:"block_hash"`
	Txn_ID     string `json:"txn_id" db:"txn_id"`
}

// DID represents a Decentralized Identifier
type DIDs struct {
	DID       string    `json:"did" db:"did"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	TotalRBTs float64   `json:"total_rbts" db:"total_rbts"`
	TotalFTs  float64   `json:"total_fts" db:"total_fts"`
	TotalNFTs int64     `json:"total_nfts" db:"total_nfts"`
	TotalSC   int64     `json:"total_sc" db:"total_sc"`
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

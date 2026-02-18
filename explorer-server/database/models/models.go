package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// Blocks

type AllBlocks struct {
	BlockHash string `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	TxnID     string `json:"txn_id" gorm:"column:txn_id"`
	BlockType string `json:"block_type" gorm:"column:block_type"`
}

func (AllBlocks) TableName() string { return "AllBlocks" }

type TransactionBlocks struct {
	BlockHash   string         `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	TxnID       *string        `json:"txn_id" gorm:"column:txn_id"`
	SenderDID   *string        `json:"sender_did" gorm:"column:sender_did"`
	ReceiverDID *string        `json:"receiver_did" gorm:"column:receiver_did"`
	AssetType   string         `json:"asset_type" gorm:"column:asset_type"`
	Amount      *float64       `json:"amount" gorm:"column:amount"`
	Epoch       *int64         `json:"epoch" gorm:"column:epoch"`
	Tokens      datatypes.JSON `json:"tokens" gorm:"column:tokens;type:jsonb"`
	Validators  datatypes.JSON `json:"validators" gorm:"column:validators;type:jsonb"`
}

func (TransactionBlocks) TableName() string { return "TransactionBlocks" }

type SCBlocks struct {
	BlockHash   string    `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	TokenID     string    `json:"token_id" gorm:"column:token_id"`
	ExecutorDID *string   `json:"executor_did" gorm:"column:executor_did"`
	DeployerDID string    `json:"deployer_did" gorm:"column:deployer_did"`
	BlockHeight int64     `json:"block_height" gorm:"column:block_height"`
	Epoch       time.Time `json:"epoch" gorm:"column:epoch"`
}

func (SCBlocks) TableName() string { return "SCBlocks" }

type BurntBlocks struct {
	BlockHash string         `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	Tokens    datatypes.JSON `json:"tokens" gorm:"column:tokens;type:jsonb"`
	OwnerDID  string         `json:"owner_did" gorm:"column:owner_did"`
	Epoch     *int64         `json:"epoch" gorm:"column:epoch"`
}

func (BurntBlocks) TableName() string { return "BurntBlocks" }

type MintBlocks struct {
	BlockHash  string         `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	TokenIDs   pq.StringArray `json:"token_ids" gorm:"type:text[];column:token_ids"`
	TokenType  string         `json:"token_type" gorm:"column:token_type"`
	TokenValue *float64       `json:"token_value" gorm:"column:token_value"`
	CreatorDID string         `json:"creator_did" gorm:"column:creator_did"`
	FTName     *string        `json:"ft_name" gorm:"column:ft_name"`
	Epoch      *int64         `json:"epoch" gorm:"column:epoch"`
}

func (MintBlocks) TableName() string { return "MintBlocks" }

// Tokens & DID
type AllTokens struct {
	TokenID   string `json:"token_id" gorm:"primaryKey;column:token_id"`
	TokenType string `json:"token_type" gorm:"column:token_type"`
}

func (AllTokens) TableName() string { return "AllTokens" }

type SC struct {
	TokenID     string `json:"token_id" gorm:"primaryKey;column:token_id"`
	BlockHash   string `json:"block_hash" gorm:"column:block_hash"`
	DeployerDID string `json:"deployer_did" gorm:"column:deployer_did"`
	ExecutorDID string `json:"executor_did" gorm:"column:executor_did"`
	BlockHeight uint64 `json:"block_height" gorm:"column:block_height"`
	TokenStatus int    `json:"token_status" gorm:"column:token_status"`
}

func (SC) TableName() string { return "SC" }

type RBT struct {
	TokenID     string  `json:"token_id" gorm:"primaryKey;column:token_id"`
	OwnerDID    string  `json:"owner_did" gorm:"column:owner_did"`
	BlockHash   string  `json:"block_hash" gorm:"column:block_hash"`
	BlockHeight string  `json:"block_height" gorm:"column:block_height"`
	TokenValue  float64 `json:"token_value" gorm:"column:token_value"`
	TokenStatus int     `json:"token_status" gorm:"column:token_status"`
}

func (RBT) TableName() string { return "RBT" }

type FT struct {
	TokenID     string  `json:"token_id" gorm:"primaryKey;column:token_id"`
	FTName      string  `json:"ft_name" gorm:"column:ft_name"`
	TokenValue  float64 `json:"token_value" gorm:"column:token_value"`
	OwnerDID    string  `json:"owner_did" gorm:"column:owner_did"`
	CreatorDID  string  `json:"creator_did" gorm:"column:creator_did"`
	BlockHeight uint64  `json:"block_height" gorm:"column:block_height"`
	TokenStatus int     `json:"token_status" gorm:"column:token_status"`
}

func (FT) TableName() string { return "FT" }

type NFT struct {
	TokenID     string `json:"token_id" gorm:"primaryKey;column:token_id"`
	TokenValue  string `json:"token_value" gorm:"column:token_value"`
	OwnerDID    string `json:"owner_did" gorm:"column:owner_did"`
	BlockHeight uint64 `json:"block_height" gorm:"column:block_height"`
	TokenStatus int    `json:"token_status" gorm:"column:token_status"`
}

func (NFT) TableName() string { return "NFT" }

type DIDs struct {
	DID       string  `json:"did" gorm:"primaryKey;column:did"`
	TotalRBTs float64 `json:"total_rbts" gorm:"column:total_rbts"`
	TotalFTs  float64 `json:"total_fts" gorm:"column:total_fts"`
	TotalNFTs int64   `json:"total_nfts" gorm:"column:total_nfts"`
	TotalSC   int64   `json:"total_sc" gorm:"column:total_sc"`
}

func (DIDs) TableName() string { return "DIDs" }

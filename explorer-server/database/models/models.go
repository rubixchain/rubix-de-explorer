package models

import (
	"time"

	"gorm.io/datatypes"
)

// ========================= TransferBlocks =========================
type TransferBlocks struct {
	BlockHash          string         `json:"block_hash" gorm:"primaryKey;column:block_hash"`
	PrevBlockID        *string        `json:"prev_block_id" gorm:"column:prev_block_id"`
	SenderDID          *string        `json:"sender_did" gorm:"column:sender_did"`
	ReceiverDID        *string        `json:"receiver_did" gorm:"column:receiver_did"`
	TxnType            *string        `json:"txn_type" gorm:"column:txn_type"`
	Amount             *float64       `json:"amount" gorm:"column:amount"`
	Epoch              *int64         `json:"epoch" gorm:"column:epoch"`
	Tokens             datatypes.JSON `json:"tokens" gorm:"column:tokens;type:jsonb"`
	ValidatorPledgeMap datatypes.JSON `json:"validator_pledge_map" gorm:"column:validator_pledge_map;type:jsonb"`
	TxnID              *string        `json:"txn_id" gorm:"column:txn_id"`
}

func (TransferBlocks) TableName() string { return "TransferBlocks" }

// ========================= TokenType =========================
type TokenType struct {
	TokenID     string    `json:"token_id" gorm:"column:token_id"`
	TokenType   string    `json:"token_type" gorm:"column:token_type"`
	LastUpdated time.Time `json:"last_updated" gorm:"column:last_updated"`
}

func (TokenType) TableName() string { return "TokenType" }

// ========================= AllBlocks =========================
type AllBlocks struct {
	BlockHash string    `json:"block_hash" gorm:"column:block_hash"`
	BlockType int64     `json:"block_type" gorm:"column:block_type"`
	Epoch     time.Time `json:"epoch" gorm:"column:epoch"`
	TxnID     string    `json:"txn_id" gorm:"column:txn_id"`
}

func (AllBlocks) TableName() string { return "AllBlocks" }

// ========================= SmartContract =========================
type SmartContract struct {
	ContractID  string `json:"contract_id" gorm:"column:contract_id"`
	BlockHash   string `json:"block_hash" gorm:"column:block_hash"`
	DeployerDID string `json:"deployer_did" gorm:"column:deployer_did"`
	TxnId       string `json:"txn_id" gorm:"column:txn_id"`
}

func (SmartContract) TableName() string { return "SmartContract" }

// ========================= RBT =========================
type RBT struct {
	TokenID     string `json:"rbt_id" gorm:"column:rbt_id"`
	OwnerDID    string `json:"owner_did" gorm:"column:owner_did"`
	BlockID     string `json:"block_id" gorm:"column:block_id"`
	BlockHeight string `json:"block_height" gorm:"column:block_height"`
}

func (RBT) TableName() string { return "RBT" }

// ========================= FT =========================
type FT struct {
	FtID        string  `json:"ft_id" gorm:"column:ft_id"`
	TokenValue  float64 `json:"token_value" gorm:"column:token_value"`
	FTName      string  `json:"ft_name" gorm:"column:ft_name"`
	OwnerDID    string  `json:"owner_did" gorm:"column:owner_did"`
	CreatorDID  string  `json:"creator_did" gorm:"column:creator_did"`
	BlockHeight string  `json:"block_height" gorm:"column:block_height"`
	BlockID     string  `json:"block_id" gorm:"column:block_id"`
	Txn_ID      string  `json:"txn_id" gorm:"column:txn_id"`
}

func (FT) TableName() string { return "FT" }

// ========================= NFT =========================
type NFT struct {
	TokenID    string `json:"nft_id" gorm:"column:nft_id"`
	TokenValue string `json:"token_value" gorm:"column:token_value"`
	OwnerDID   string `json:"owner_did" gorm:"column:owner_did"`
	BlockHash  string `json:"block_hash" gorm:"column:block_hash"`
	Txn_ID     string `json:"txn_id" gorm:"column:txn_id"`
}

func (NFT) TableName() string { return "NFT" }

// ========================= DIDs =========================
type DIDs struct {
	DID       string    `json:"did" gorm:"primaryKey;column:did"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	TotalRBTs float64   `json:"total_rbts" gorm:"column:total_rbts"`
	TotalFTs  float64   `json:"total_fts" gorm:"column:total_fts"`
	TotalNFTs int64     `json:"total_nfts" gorm:"column:total_nfts"`
	TotalSC   int64     `json:"total_sc" gorm:"column:total_sc"`
}

func (DIDs) TableName() string { return "DIDs" }

// ========================= TxnAnalytics =========================
type TxnAnalytics struct {
	IntervalStart time.Time `json:"interval_start" gorm:"column:interval_start"`
	IntervalEnd   time.Time `json:"interval_end" gorm:"column:interval_end"`
	TxnCount      int64     `json:"txn_count" gorm:"column:txn_count"`
	TotalValue    float64   `json:"total_value" gorm:"column:total_value"`
	TokenType     string    `json:"token_type" gorm:"column:token_type"`
}

func (TxnAnalytics) TableName() string { return "TxnAnalytics" }

// ========================= DatabaseHealth =========================
type DatabaseHealth struct {
	IsConnected bool   `json:"is_connected" gorm:"column:is_connected"`
	Status      string `json:"status" gorm:"column:status"`
	Message     string `json:"message" gorm:"column:message"`
}

func (DatabaseHealth) TableName() string { return "DatabaseHealth" }

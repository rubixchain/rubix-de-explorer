package models

// Transaction

type TransInfo struct {
	Initiator       *string            `json:"initiator" gorm:"column:initiator;index"`
	Owner           *string            `json:"owner" gorm:"column:owner;index"`
	Epoch           *int64             `json:"epoch" gorm:"column:epoch;index"`
	Network         string             `json:"network" gorm:"column:network"`
	Tokens          *TransactionTokens `json:"tokens" gorm:"column:tokens;type:jsonb"`
	CommittedTokens []*TokenInfo       `json:"committed_tokens" gorm:"column:committed_tokens;type:jsonb"`
	Quorums         []*QuorumInfo      `json:"quorums" gorm:"column:quorums;type:jsonb"`
	Memo            string             `json:"memo" gorm:"column:memo"`
}

func (TransInfo) TableName() string { return "TransInfo" }

type TransactionTokens struct {
	RBT           []*TokenInfo `json:"rbt"`
	NFT           []*TokenInfo `json:"nft"`
	FT            []*TokenInfo `json:"ft"`
	SmartContract []*TokenInfo `json:"smart_contract"`
}

type TokenInfo struct {
	TokenID               string `json:"token_id"`
	PreviousTransactionID string `json:"previous_transaction_id"`
	Data                  string `json:"data"`
}

type QuorumInfo struct {
	DID       string       `json:"did"`
	TokenInfo []*TokenInfo `json:"token_info"`
}

type QuorumSignatures struct {
	DID       string `json:"did"`
	Signature string `json:"signature"`
}

type Signatures struct {
	InitiatorSignature string              `json:"initiator_signature"`
	QuorumSignatures   []*QuorumSignatures `json:"quorum_signatures"`
}

type Transactions struct {
	TransactionID   string      `json:"transaction_id"`
	TransactionInfo *TransInfo  `json:"transaction_info"`
	Signatures      *Signatures `json:"signatures"`
}

type EventTransaction struct {
	Transaction *Transactions `json:"transaction"`
	Status      int           `json:"status"`
	Message     string        `json:"message"`
}

// Token

type RBT struct {
	TokenID     string  `json:"token_id" gorm:"primaryKey;column:token_id"`
	OwnerDID    string  `json:"owner_did" gorm:"column:owner_did;index"`
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
	OwnerDID    string  `json:"owner_did" gorm:"column:owner_did;index"`
	CreatorDID  string  `json:"creator_did" gorm:"column:creator_did"`
	BlockHeight uint64  `json:"block_height" gorm:"column:block_height"`
	TokenStatus int     `json:"token_status" gorm:"column:token_status"`
}

func (FT) TableName() string { return "FT" }

type NFT struct {
	TokenID     string `json:"token_id" gorm:"primaryKey;column:token_id"`
	TokenValue  string `json:"token_value" gorm:"column:token_value"`
	OwnerDID    string `json:"owner_did" gorm:"column:owner_did;index"`
	BlockHeight uint64 `json:"block_height" gorm:"column:block_height"`
	TokenStatus int    `json:"token_status" gorm:"column:token_status"`
}

func (NFT) TableName() string { return "NFT" }

type SC struct {
	TokenID     string `json:"token_id" gorm:"primaryKey;column:token_id"`
	BlockHash   string `json:"block_hash" gorm:"column:block_hash"`
	DeployerDID string `json:"deployer_did" gorm:"column:deployer_did;index"`
	ExecutorDID string `json:"executor_did" gorm:"column:executor_did"`
	BlockHeight uint64 `json:"block_height" gorm:"column:block_height"`
	TokenStatus int    `json:"token_status" gorm:"column:token_status"`
}

func (SC) TableName() string { return "SC" }

// Token chain
type TokenChain struct {
	ID                    uint64 `json:"id" gorm:"primaryKey;column:id"` //autoincrement
	TokenID               string `json:"token_id" gorm:"column:token_id"`
	TransactionID         string `json:"transaction_id" gorm:"column:transaction_id"`
	PreviousTransactionID string `json:"previous_transaction_id" gorm:"column:previous_transaction_id"`
	Role                  string `json:"role" gorm:"column:role"`
}

func (TokenChain) TableName() string { return "TokenChain" }

type TokenChainIndex struct {
	TokenID string   `json:"token_id" gorm:"primaryKey;column:token_id"`
	Index   []string `json:"index" gorm:"column:index;type:jsonb"`
}

func (TokenChainIndex) TableName() string { return "TokenChainIndex" }

// DID

type DIDs struct {
	DID       string  `json:"did" gorm:"primaryKey;column:did"`
	TotalRBTs float64 `json:"total_rbts" gorm:"column:total_rbts"`
	TotalFTs  float64 `json:"total_fts" gorm:"column:total_fts"`
	TotalNFTs int64   `json:"total_nfts" gorm:"column:total_nfts"`
	TotalSC   int64   `json:"total_sc" gorm:"column:total_sc"`
}

func (DIDs) TableName() string { return "DIDs" }

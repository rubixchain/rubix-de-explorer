package model

import (
	"time"
)

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

type TransferBlocks struct {
	BlockHash          string               `json:"block_hash" db:"block_hash"`
	PrevBlockID        *string              `json:"prev_block_id" db:"prev_block_id"`
	SenderDID          *string              `json:"sender_did" db:"sender_did"`
	ReceiverDID        *string              `json:"receiver_did" db:"receiver_did"`
	TxnType            *string              `json:"txn_type" db:"txn_type"`
	Amount             *float64             `json:"amount" db:"amount"`
	Epoch              *int64               `json:"epoch" db:"epoch"`
	Tokens             []string             `json:"tokens" db:"tokens"`
	ValidatorPledgeMap *map[string][]string `json:"validator_pledge_map" db:"validator_pledge_map"`
	TxnID              *string              `json:"txn_id" db:"txn_id"`
}

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
	ContractID     string     `json:"contract_id" db:"contract_id"`
	CreatorDID     string     `json:"creator_did" db:"creator_did"`
	DeployerDID    string     `json:"deployer_did" db:"deployer_did"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	LastExecutedAt *time.Time `json:"last_executed_at" db:"last_executed_at"`
}

type RBT struct {
	TokenID     string  `json:"rbt_id" db:"rbt_id"`
	TokenValue  float64 `json:"token_value" db:"token_value"`
	OwnerDID    string  `json:"owner_did" db:"owner_did"`
	BlockHash   string  `json:"block_id" db:"block_id"`
	BlockHeight string  `json:"block_height" db:"block_height"`
}

type FT struct {
	TokenID     string  `json:"ft_id" db:"ft_id"`
	TokenValue  float64 `json:"token_value" db:"token_value"`
	FTName      string  `json:"ft_name" db:"ft_name"`
	OwnerDID    string  `json:"owner_did" db:"owner_did"`
	CreatorDID  string  `json:"creator_did" db:"creator_did"`
	BlockHeight string  `json:"block_height" db:"block_height"`
	BlockID     string  `json:"block_id" db:"block_id"`
}

type NFT struct {
	TokenID    string `json:"nft_id" db:"nft_id"`
	CreatorDID string `json:"creator_did" db:"creator_did"`
	OwnerDID   string `json:"owner_did" db:"owner_did"`
	BlockID    string `json:"block_id" db:"block_id"`
}

// DID represents a Decentralized Identifier
type DID struct {
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
	RBT    RBT   `json:"rbt"`
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
	DID  DID   `json:"did"`
	RBTs []RBT `json:"rbts"`
	// Trasactions []TransactionResponse `json:"transactions"`
}

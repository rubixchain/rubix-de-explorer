package model

import (
	"explorer-server/database/models"
)

// --- Paginated Response Wrappers ---

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

func NewPaginated(data interface{}, total int64, page, limit int) PaginatedResponse {
	tp := int(total) / limit
	if int(total)%limit != 0 {
		tp++
	}
	return PaginatedResponse{Data: data, Total: total, Page: page, Limit: limit, TotalPages: tp}
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
	FTName     string  `json:"ftName"`
	Count      float64 `json:"count"`
	CreatorDID string  `json:"creatorDID"`
}

type FTSuggestion struct {
	FTName     string `json:"ft_name" gorm:"column:ft_name"`
	CreatorDID string `json:"creator_did" gorm:"column:creator_did"`
}

type RBTSuggestion struct {
	TokenID string `json:"token_id" gorm:"column:token_id"`
}

type RBTInfo struct {
	TokenID    string  `json:"token_id"`
	OwnerDID   string  `json:"owner_did"`
	TokenValue float64 `json:"token_value"`
}

type FTInfo struct {
	FTName      string  `json:"ft_name" gorm:"column:ft_name"`
	CreatorDID  string  `json:"creator_did" gorm:"column:creator_did"`
	FTValue     float64 `json:"ft_value" gorm:"column:ft_value"`
	TotalAmount int64   `json:"total_amount" gorm:"column:total_amount"`
	CreatedTime int64   `json:"created_time" gorm:"column:created_time"`
}

// DAGEdge represents a directed connection between two transactions in the DAG.
// From is the child (newer), To is the parent (older).
type DAGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// DAGResponse is returned by /api/dagtxn/{txnID}.
// Transactions contains all nodes (including the anchor), Edges contains all directed links.
// For infinite scroll: use the oldest transaction in the current response as the
// anchor for the next call to extend the graph further back.
type DAGResponse struct {
	Transactions []models.TransactionInfo `json:"transactions"`
	Edges        []DAGEdge                `json:"edges"`
}

type FTHolder struct {
	DID        string `json:"did" gorm:"column:did"`
	TokenCount int64  `json:"token_count" gorm:"column:token_count"`
}

type FTTopHoldersResponse struct {
	Holders    []FTHolder `json:"holders"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
}

type DIDInfo struct {
	PeerID    string `json:"peer_id"`
	DID       string `json:"did"`
	DIDAlgo   int64  `json:"did_algo"`
	Signature string `json:"signature"`
	Time      string `json:"time"`
}

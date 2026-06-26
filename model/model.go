package model

import (
	"encoding/json"
	"strings"
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
	TokenID               string  `json:"tokenId"`
	PreviousTransactionID string  `json:"previousTransactionID"`
	Data                  string  `json:"data"`
	TokenValue            float64 `json:"tokenValue"`
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

// Transactions DTO — matches Rubix Core's Transactions struct.
// Core uses `db:` tags (no `json:` tags), so Go defaults to uppercase field names.
// Info and Signature are json.RawMessage in the Core, parsed separately.
type Transactions struct {
	ID        string          `json:"id"`
	Info      json.RawMessage `json:"info"`
	Signature json.RawMessage `json:"signature"`
}

// trimTokenInfos strips surrounding whitespace from the identifier fields of a
// slice of TokenInfo. Upstream nodes occasionally emit whitespace-padded IDs.
func trimTokenInfos(ts []*TokenInfo) {
	for _, t := range ts {
		if t == nil {
			continue
		}
		t.TokenID = strings.TrimSpace(t.TokenID)
		t.PreviousTransactionID = strings.TrimSpace(t.PreviousTransactionID)
	}
}

// Normalize trims surrounding whitespace from every DID and token identifier in
// the transaction. Upstream payloads sometimes carry padded fields (e.g. a
// whitespace-prefixed owner DID); without this, the anchored format regexes in
// util reject otherwise-valid identities and the txn lands in
// FailedTransactionInfo. Called from ParseInfo so validation, the flattened
// TransactionInfo row, and asset processing all see clean values.
func (info *TransactionInfo) Normalize() {
	if info == nil {
		return
	}
	info.Initiator = strings.TrimSpace(info.Initiator)
	info.Owner = strings.TrimSpace(info.Owner)
	info.Network = strings.TrimSpace(info.Network)
	if info.Tokens != nil {
		trimTokenInfos(info.Tokens.RBT)
		trimTokenInfos(info.Tokens.NFT)
		trimTokenInfos(info.Tokens.FT)
		trimTokenInfos(info.Tokens.SmartContract)
	}
	trimTokenInfos(info.CommittedTokens)
	for _, q := range info.Quorums {
		if q == nil {
			continue
		}
		q.Did = strings.TrimSpace(q.Did)
		trimTokenInfos(q.Tokens)
	}
}

// Normalize trims surrounding whitespace from the quorum DIDs in a signature.
// Signature payloads themselves are left untouched.
func (s *Signature) Normalize() {
	if s == nil {
		return
	}
	for i := range s.Quorums {
		s.Quorums[i].Did = strings.TrimSpace(s.Quorums[i].Did)
	}
}

// ParseInfo deserializes the raw Info bytes into a structured TransactionInfo.
func (t *Transactions) ParseInfo() (*TransactionInfo, error) {
	if t.Info == nil {
		return nil, nil
	}
	var info TransactionInfo
	if err := json.Unmarshal(t.Info, &info); err != nil {
		return nil, err
	}
	info.Normalize()
	return &info, nil
}

// ParseSignature deserializes the raw Signature bytes into a structured Signature.
func (t *Transactions) ParseSignature() (*Signature, error) {
	if t.Signature == nil {
		return nil, nil
	}
	var sig Signature
	if err := json.Unmarshal(t.Signature, &sig); err != nil {
		return nil, err
	}
	sig.Normalize()
	return &sig, nil
}

type EventTransaction struct {
	Transaction   *Transactions `json:"transaction"`
	Status        bool          `json:"status"`
	Message       string        `json:"message"`
	TransactionID string        `json:"transaction_id"`
	AssetType     int           `json:"asset_type"`
}

type FTGroup struct {
	FTName     string  `json:"ftName" gorm:"column:ft_name"`
	Count      float64 `json:"count" gorm:"column:count"`
	CreatorDID string  `json:"creatorDID" gorm:"column:creator_did"`
	FTValue    float64 `json:"ftValue" gorm:"column:ft_value"`
}

type FTHolding struct {
	FTName     string  `json:"ft_name" gorm:"column:ft_name"`
	CreatorDID string  `json:"creator_did" gorm:"column:creator_did"`
	Count      float64 `json:"count" gorm:"column:count"`
}

type FTHolderInfo struct {
	DID          string      `json:"did"`
	TotalFTCount int64       `json:"total_ft_count"`
	Holdings     []FTHolding `json:"holdings"`
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

// DAGTxn represents a single transaction node: its ID and up to 20 parent IDs.
type DAGTxn struct {
	TransactionID          string   `json:"transaction_id"`
	PreviousTransactionIDs []string `json:"previous_transaction_ids"`
}

// DAGResponse is returned by /api/dagtxns and /api/dagtxn/{txnID}.
type DAGResponse struct {
	Transactions []DAGTxn `json:"transactions"`
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

// UnpledgeEvent is the PubSub payload published by quorum nodes after unpledging tokens.
type UnpledgeEvent struct {
	UnpledgeInfo          []UnpledgeTokenInfo `json:"unpledgeInfo"`
	PledgeTransactionID   string              `json:"pledgeTransactionID"`
	UnpledgeTransactionID string              `json:"unpledgeTransactionID"`
}

// UnpledgeTokenInfo describes a single token being unpledged.
type UnpledgeTokenInfo struct {
	TokenID string `json:"tokenId"`
}


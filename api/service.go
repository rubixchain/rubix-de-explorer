package api

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"strings"
)

// SearchResult is the unified structure for search API responses
type SearchResult struct {
	Type string      `json:"type"` // "DID", "Token", "Transaction"
	Data interface{} `json:"data"`
}

// -------------------------------------------------------------------
// 1. Search Logic
// -------------------------------------------------------------------

// GetSearchInfo routes search queries to the appropriate table based on format
func GetSearchInfo(query string) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// 1. DID Search (starts with bafybm)
	if strings.HasPrefix(query, "bafybm") {
		var balances []models.DIDBalance
		if err := database.ReadDB.Table("DIDBalances").Where("did = ?", query).Find(&balances).Error; err == nil && len(balances) > 0 {
			return &SearchResult{Type: "DID", Data: balances}, nil
		}
	}

	// 2. Token Search (starts with Qm or contains _)
	if strings.HasPrefix(query, "Qm") || strings.Contains(query, "_") {
		var token models.Token
		if err := database.ReadDB.Table("Tokens").Where("token_id = ?", query).First(&token).Error; err == nil {
			return &SearchResult{Type: "Token", Data: token}, nil
		}
	}

	// 3. Transaction Search (fallback)
	var txn models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", query).First(&txn).Error; err == nil {
		return &SearchResult{Type: "Transaction", Data: txn}, nil
	}

	return nil, fmt.Errorf("no data found for ID: %s", query)
}

// -------------------------------------------------------------------
// 2. Statistics (Counts)
// -------------------------------------------------------------------

func GetRBTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetNFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 3).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetTxnsCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Transactions{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetDIDCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.DIDBalance{}).Distinct("did").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// -------------------------------------------------------------------
// 3. Lists and Holders
// -------------------------------------------------------------------

func GetTransactionInfoList(limit, page int) ([]models.TransactionInfo, error) {
	var transactions []models.TransactionInfo
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	if err := database.ReadDB.Table("TransactionInfo").Order("epoch DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func GetDIDHoldersList(limit, page int) ([]models.DIDBalance, error) {
	var balances []models.DIDBalance
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	if err := database.ReadDB.Table("DIDBalances").Where("asset_type = ? AND balance > 0", "RBT").Order("balance DESC").Limit(limit).Offset(offset).Find(&balances).Error; err != nil {
		return nil, err
	}
	return balances, nil
}

func GetRBTList(limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 1).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func GetFTGroupList(limit, page int) ([]model.FTGroup, error) {
	var tokens []models.Token
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Find(&tokens).Error; err != nil {
		return nil, err
	}

	groupsMap := make(map[string]*model.FTGroup)
	for _, t := range tokens {
		parts := strings.Split(t.TokenID, "_")
		if len(parts) >= 3 {
			ftName := parts[0]
			creatorDID := parts[len(parts)-1]
			key := ftName + "_" + creatorDID
			if g, ok := groupsMap[key]; ok {
				g.Count++
			} else {
				groupsMap[key] = &model.FTGroup{FTName: ftName, Count: 1, CreatorDID: creatorDID}
			}
		}
	}

	var allGroups []model.FTGroup
	for _, g := range groupsMap {
		allGroups = append(allGroups, *g)
	}

	start := (page - 1) * limit
	if start > len(allGroups) {
		start = len(allGroups)
	}
	end := start + limit
	if end > len(allGroups) {
		end = len(allGroups)
	}
	return allGroups[start:end], nil
}

func GetFTListByFTName(ftName string, creatorDID string, limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit
	query := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Where("token_id LIKE ?", ftName+"_%")
	if creatorDID != "" {
		query = query.Where("token_id LIKE ?", "%_"+creatorDID)
	}
	if err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func GetSCList(limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// -------------------------------------------------------------------
// 4. Specific Info and History
// -------------------------------------------------------------------

func GetTransactionInfo(txnID string) (models.TransactionInfo, error) {
	var transaction models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", txnID).First(&transaction).Error; err != nil {
		return transaction, err
	}
	return transaction, nil
}

func GetTransactionInfoListByToken(tokenID string, page, limit int) ([]models.TransactionInfo, error) {
	ids, err := GetTransactionIDList(tokenID, page, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []models.TransactionInfo{}, nil
	}

	txnIDs := make([]string, len(ids))
	for i, entry := range ids {
		txnIDs[i] = entry.TransactionID
	}

	var transactions []models.TransactionInfo
	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id IN ?", txnIDs).Order("epoch DESC").Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func GetTransactionIDList(tokenID string, page, limit int) ([]models.TokenChain, error) {
	var tca models.TokenChainArray
	if err := database.ReadDB.Table("TokenChainArray").Where("token_id = ?", tokenID).First(&tca).Error; err != nil {
		return nil, err
	}

	var chain []uint64
	if err := json.Unmarshal(tca.Index, &chain); err != nil {
		return nil, err
	}

	total := len(chain)
	if total == 0 {
		return []models.TokenChain{}, nil
	}

	end := total - ((page - 1) * limit)
	start := total - (page * limit)
	if end <= 0 {
		return []models.TokenChain{}, nil
	}
	if start < 0 {
		start = 0
	}
	pagedIndices := chain[start:end]

	var history []models.TokenChain
	if err := database.ReadDB.Table("TokenChain").Where("id IN ?", pagedIndices).Order("id DESC").Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

func GetTokenInfo(tokenID string) (models.Token, error) {
	var token models.Token
	if err := database.ReadDB.Table("Tokens").Where("token_id = ?", tokenID).First(&token).Error; err != nil {
		return token, err
	}
	return token, nil
}

package api

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetTxnsCount returns total number of TransferBlocks records (cached 5s)
// func GetTxnsCount() (int64, error) {
// 	if cached, ok := responseCache.Get("txns_count"); ok {
// 		return cached.(int64), nil
// 	}
// 	var count int64
// 	if err := database.ReadDB.Model(&model.TransactionInfo{}).Count(&count).Error; err != nil {
// 		return 0, err
// 	}
// 	responseCache.Set("txns_count", count, 5*time.Second)
// 	return count, nil
// }

func GetTransactionInfoList(limit, page int) ([]models.TransactionInfo, error) {
	var transactions []models.TransactionInfo

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Fetch paginated transactions from TransactionInfo table
	if err := database.ReadDB.
		Table("TransactionInfo").
		Order("epoch DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error; err != nil {
		return nil, err
	}

	return transactions, nil
}

func GetTransactionInfo(txnID string) (models.TransactionInfo, error) {
	var transaction models.TransactionInfo

	if err := database.ReadDB.Table("TransactionInfo").Where("transaction_id = ?", txnID).First(&transaction).Error; err != nil {
		return transaction, err
	}

	return transaction, nil
}

func GetTokenInfo(tokenID string) (models.Token, error) {
	var token models.Token

	if err := database.ReadDB.Table("Tokens").Where("token_id = ?", tokenID).First(&token).Error; err != nil {
		return token, err
	}

	return token, nil
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

	// Pagination: from the end (most recent first)
	end := total - ((page - 1) * limit)
	start := total - (page * limit)

	if end <= 0 {
		return []models.TokenChain{}, nil
	}
	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}

	pagedIndices := chain[start:end]

	var history []models.TokenChain
	if err := database.ReadDB.Table("TokenChain").
		Where("id IN ?", pagedIndices).
		Order("id DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}

	return history, nil
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
	// Order by epoch DESC to ensure most recent transactions for that token are first
	if err := database.ReadDB.Table("TransactionInfo").
		Where("transaction_id IN ?", txnIDs).
		Order("epoch DESC").
		Find(&transactions).Error; err != nil {
		return nil, err
	}

	return transactions, nil
}

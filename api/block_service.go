package api

import (
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

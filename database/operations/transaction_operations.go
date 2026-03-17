package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"

	"gorm.io/gorm/clause"
)

// SaveTransaction saves a transaction to the database
func SaveTransaction(txn *models.Transactions) error {
	return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(txn).Error
}

// SaveEventTransaction saves the PubSub event wrapper (status + error message for failed consensus)
func SaveEventTransaction(txnID string, status bool, message string) error {
	event := &models.EventTransaction{
		TransactionID: txnID,
		Status:        status,
		Message:       message,
	}
	return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

// SaveTransactionDetails saves the flattened TransactionInfo fields for easy querying
func SaveTransactionDetails(txnID string, info *model.TransactionInfo) error {
	tokensJSON, _ := json.Marshal(info.Tokens)
	committedJSON, _ := json.Marshal(info.CommittedTokens)
	quorumsJSON, _ := json.Marshal(info.Quorums)

	details := &models.TransactionInfo{
		TransactionID:   txnID,
		Initiator:       info.Initiator,
		Owner:           info.Owner,
		Epoch:           info.Epoch,
		Network:         info.Network,
		Tokens:          tokensJSON,
		CommittedTokens: committedJSON,
		Quorums:         quorumsJSON,
		Memo:            info.Memo,
	}

	return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
}

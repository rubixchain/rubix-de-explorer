package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"

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

// SaveTransactionDetails saves the flattened TransactionInfo fields for easy querying.
// If status is false, it saves to FailedTransactionInfo table.
func SaveTransactionDetails(txnID string, info *model.TransactionInfo, status bool) error {
	tokensJSON, _ := json.Marshal(info.Tokens)
	committedJSON, _ := json.Marshal(info.CommittedTokens)
	quorumsJSON, _ := json.Marshal(info.Quorums)

	// Calculate total pledge amount from Quorums (RBT values only)
	var totalPledge float64
	for _, q := range info.Quorums {
		for _, t := range q.Tokens {
			val, _ := util.GetTokenValueFromTokenID(t.TokenID)
			totalPledge += val
		}
	}

	if status {
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
			Status:          true,
			Amount:          totalPledge,
		}
		return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
	} else {
		details := &models.FailedTransactionInfo{
			TransactionID:   txnID,
			Initiator:       info.Initiator,
			Owner:           info.Owner,
			Epoch:           info.Epoch,
			Network:         info.Network,
			Tokens:          tokensJSON,
			CommittedTokens: committedJSON,
			Quorums:         quorumsJSON,
			Memo:            info.Memo,
			Status:          false,
			Amount:          totalPledge,
		}
		return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
	}
}

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

	// Calculate total amount from transferred RBTs
	var txnAmount float64
	if info.Tokens != nil {
		for _, t := range info.Tokens.RBT {
			val, _ := util.GetTokenValueFromTokenID(t.TokenID)
			txnAmount += val
		}
	}

	// Calculate total pledge amount from Quorums
	var totalPledge float64
	for _, q := range info.Quorums {
		for _, t := range q.Tokens {
			val, _ := util.GetTokenValueFromTokenID(t.TokenID)
			totalPledge += val
		}
	}

	// For the Explorer display, the transaction Amount should reflect the
	// primary payload (Tokens) if it exists, otherwise fallback to pledge.
	// We'll combine them to represent the total RBT volume of this Txn.
	finalAmount := txnAmount + totalPledge

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
			Amount:          finalAmount,
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
			Amount:          finalAmount,
		}
		return database.WriteDB.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
	}
}

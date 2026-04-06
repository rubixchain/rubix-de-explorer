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

	var finalAmount float64

	// Amount Waterfall (derived from Rubix Core protocol):
	// 1. Quorums present       → Transfer / SC Deploy / Execute → sum of pledged tokens
	// 2. Tokens.RBT (no quorum) → RBT Mint or Split            → always 1 RBT
	// 3. CommittedTokens only   → FT Mint                      → sum of burned RBT values
	if len(info.Quorums) > 0 {
		// Transfer, SC Deploy, or Multi-Asset Transfer: Amount = Sum of Pledged Tokens
		for _, q := range info.Quorums {
			for _, t := range q.Tokens {
				val, _ := util.GetTokenValueFromTokenID(t.TokenID)
				finalAmount += val
			}
		}
	} else if info.Tokens != nil && len(info.Tokens.RBT) > 0 {
		// RBT Mint or Split: always exactly 1 RBT involved per transaction
		finalAmount = 1.0
	} else if len(info.CommittedTokens) > 0 {
		// FT Mint: Amount = sum of burned committed RBT values
		for _, ct := range info.CommittedTokens {
			val, _ := util.GetTokenValueFromTokenID(ct.TokenID)
			finalAmount += val
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

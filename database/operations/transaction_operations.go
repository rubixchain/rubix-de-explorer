package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveTransaction saves a transaction to the database
func SaveTransaction(db *gorm.DB, txn *models.Transactions) error {
	if db == nil {
		db = database.WriteDB
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(txn).Error
}

// SaveEventTransaction saves the PubSub event wrapper (status + error message for failed consensus)
func SaveEventTransaction(db *gorm.DB, txnID string, status bool, message string) error {
	if db == nil {
		db = database.WriteDB
	}
	event := &models.EventTransaction{
		TransactionID: txnID,
		Status:        status,
		Message:       message,
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

// SaveTransactionDetails saves the flattened TransactionInfo fields for easy querying.
// If status is false, it saves to FailedTransactionInfo table.
func SaveTransactionDetails(db *gorm.DB, txnID string, info *model.TransactionInfo, status bool, failureReason ...string) error {
	if db == nil {
		db = database.WriteDB
	}
	tokensJSON, _ := json.Marshal(info.Tokens)
	committedJSON, _ := json.Marshal(info.CommittedTokens)
	quorumsJSON, _ := json.Marshal(info.Quorums)

	// Enrich tokenValue fields before marshaling (Rubix Core sends tokenValue: 0)
	if info.Tokens != nil {
		for _, t := range info.Tokens.RBT {
			if t.TokenValue == 0 {
				t.TokenValue, _ = util.GetTokenValueFromTokenID(t.TokenID)
			}
		}
		tokensJSON, _ = json.Marshal(info.Tokens)
	}
	for _, ct := range info.CommittedTokens {
		if ct.TokenValue == 0 {
			ct.TokenValue, _ = util.GetTokenValueFromTokenID(ct.TokenID)
		}
	}
	committedJSON, _ = json.Marshal(info.CommittedTokens)

	var finalAmount float64

	// Amount Waterfall (derived from Rubix Core protocol):
	// 1. Quorums present       → Transfer / SC Deploy / Execute → sum of pledged tokens
	// 2. Tokens.RBT (no quorum) → RBT Mint or Split            → sum of token values
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
		// RBT Mint or Split: Sum of actual token values
		for _, t := range info.Tokens.RBT {
			val, _ := util.GetTokenValueFromTokenID(t.TokenID)
			finalAmount += val
		}
	} else if len(info.CommittedTokens) > 0 {
		// FT Mint: Amount = sum of burned committed RBT values
		for _, ct := range info.CommittedTokens {
			val, _ := util.GetTokenValueFromTokenID(ct.TokenID)
			finalAmount += val
		}
	}

	finalAmount = util.RoundToMaxDecimals(finalAmount)

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
		return db.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
	} else {
		reason := ""
		if len(failureReason) > 0 {
			reason = failureReason[0]
		}
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
			FailureReason:   reason,
		}
		return db.Clauses(clause.OnConflict{DoNothing: true}).Create(details).Error
	}
}

func SaveFailedTransactionReason(db *gorm.DB, txnID, reason string) error {
	if db == nil {
		db = database.WriteDB
	}
	details := &models.FailedTransactionInfo{
		TransactionID: txnID,
		Status:        false,
		FailureReason: reason,
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "transaction_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"failure_reason": reason,
			"status":         false,
		}),
	}).Create(details).Error
}

package operations

import (
	"encoding/json"
	"errors"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// MaxTokenSyncBatchSize matches the fullnode cap (server/transaction.go).
const MaxTokenSyncBatchSize = 50

// ErrChainValidationFailed signals the chain returned by the fullnode is internally
// inconsistent (a previous_transaction_id pointer didn't line up). Inserts up to the
// mismatch are committed; the token stays flagged so the next cycle retries.
var ErrChainValidationFailed = errors.New("chain validation failed")

// GetTokenIDsNeedingSync returns token IDs flagged for backfill (needs_sync or chain sync issue).
func GetTokenIDsNeedingSync(db *gorm.DB, limit int) ([]string, error) {
	if db == nil {
		db = database.ReadDB
	}
	if limit <= 0 {
		limit = MaxTokenSyncBatchSize
	}
	var ids []string
	err := db.Model(&models.Token{}).
		Where("needs_sync = ? OR token_status = ?", true, models.TokenStatus_ChainSyncIssue).
		Order("updated_at ASC").
		Limit(limit).
		Pluck("token_id", &ids).Error
	return ids, err
}

// BatchTokenIDs splits IDs into chunks of at most MaxTokenSyncBatchSize.
func BatchTokenIDs(tokenIDs []string) [][]string {
	if len(tokenIDs) == 0 {
		return nil
	}
	var batches [][]string
	for i := 0; i < len(tokenIDs); i += MaxTokenSyncBatchSize {
		end := i + MaxTokenSyncBatchSize
		if end > len(tokenIDs) {
			end = len(tokenIDs)
		}
		batch := make([]string, end-i)
		copy(batch, tokenIDs[i:end])
		batches = append(batches, batch)
	}
	return batches
}

// TransactionExists reports whether a transaction hash is already stored.
func TransactionExists(db *gorm.DB, txnID string) (bool, error) {
	if db == nil {
		db = database.ReadDB
	}
	if txnID == "" {
		return false, nil
	}
	var count int64
	err := db.Model(&models.Transactions{}).Where("id = ?", txnID).Count(&count).Error
	return count > 0, err
}

// IngestTokenChain backfills missing transactions for one token using the fullnode's
// chronological chain (genesis at index 0).
//
// Validation is purely STRUCTURAL — it verifies the chain itself is contiguous
// (chain[0].PreviousTransactionID == "" and chain[i].PreviousTransactionID == chain[i-1].ID).
// It does NOT compare against the explorer's existing state; dedupe handles reconciliation.
//
// Non-destruction guarantee: existing rows (already inserted by the PubSub path or a prior
// sync cycle) are NEVER touched. Dedupe'd entries are skipped entirely — no SaveTransaction,
// no SaveTransactionDetails, no SaveEventTransaction, no ProcessTransactionAssets.
//
// Outcomes:
//   - Empty chain → return nil. We have no evidence the explorer is up to date.
//   - Full validation OK → MarkTokenSyncedWithStatus re-derives token_status from the
//     last entry's role and clears needs_sync.
//   - Structural mismatch → inserts up to that point stay committed, MarkTokenSynced
//     is NOT called (token stays flagged), and ErrChainValidationFailed is returned.
func IngestTokenChain(db *gorm.DB, tokenID string, chain []model.SyncedTxn) error {
	if len(chain) == 0 {
		return nil
	}

	var (
		chainValid  = true
		mismatchIdx int
		mismatchExp string
		mismatchGot string
		inserted    int
	)

	run := func(tx *gorm.DB) error {
		for i := range chain {
			entry := chain[i]

			// 1. Structural validation FIRST — verify the chain is internally consistent.
			expectedPrev := ""
			if i > 0 {
				expectedPrev = chain[i-1].ID
			}
			if entry.PreviousTransactionID != expectedPrev {
				chainValid = false
				mismatchIdx = i
				mismatchExp = expectedPrev
				mismatchGot = entry.PreviousTransactionID
				log.Printf("[TokenSync] Chain validation failed for token %s at index %d: expected previous=%q, got %q",
					tokenID, i, expectedPrev, entry.PreviousTransactionID)
				break
			}

			// 2. Dedupe — skip ENTIRELY if already stored. Do not call any Save*
			//    or ProcessTransactionAssets; the existing row stays byte-identical.
			exists, err := TransactionExists(tx, entry.ID)
			if err != nil {
				return err
			}
			if exists {
				continue
			}

			// 3. Net-new insert path.
			if err := insertSyncedTxn(tx, tokenID, entry); err != nil {
				return err
			}
			inserted++
		}

		if chainValid {
			if inserted > 0 {
				log.Printf("[TokenSync] Token %s: ingested %d transaction(s) from chain of %d", tokenID, inserted, len(chain))
			}
			return MarkTokenSyncedWithStatus(tx, tokenID, chain[len(chain)-1].Role)
		}
		if inserted > 0 {
			log.Printf("[TokenSync] Token %s: ingested %d transaction(s) before chain break", tokenID, inserted)
		}
		return nil
	}

	var err error
	if db != nil {
		err = run(db)
	} else {
		err = database.WriteDB.Transaction(run)
	}
	if err != nil {
		return err
	}
	if !chainValid {
		return fmt.Errorf("token %s at chain index %d (expected previous=%q, got %q): %w",
			tokenID, mismatchIdx, mismatchExp, mismatchGot, ErrChainValidationFailed)
	}
	return nil
}

// insertSyncedTxn writes ONE net-new SyncedTxn through the same downstream path
// PubSub uses: Transactions, EventTransactions, TransactionInfo, then PTA for the
// asset/balance side-effects. Uses entry.ID directly as the PK — no hashing.
//
// All four downstream calls already use clause.OnConflict{DoNothing} or equivalent
// idempotency, but this function should only ever be invoked AFTER TransactionExists
// has returned false for entry.ID.
func insertSyncedTxn(tx *gorm.DB, tokenID string, entry model.SyncedTxn) error {
	if entry.Info == nil {
		return fmt.Errorf("synced txn %s for token %s has nil Info", entry.ID, tokenID)
	}

	infoJSON, err := json.Marshal(entry.Info)
	if err != nil {
		return fmt.Errorf("marshal info for %s: %w", entry.ID, err)
	}

	if err := SaveEventTransaction(tx, entry.ID, true, ""); err != nil {
		return fmt.Errorf("save event for %s: %w", entry.ID, err)
	}
	if err := SaveTransaction(tx, &models.Transactions{
		ID:        entry.ID,
		Info:      infoJSON,
		Signature: json.RawMessage(`{}`),
	}); err != nil {
		return fmt.Errorf("save raw txn %s: %w", entry.ID, err)
	}
	if err := SaveTransactionDetails(tx, entry.ID, entry.Info, true); err != nil {
		return fmt.Errorf("save txn details %s: %w", entry.ID, err)
	}
	if err := ProcessTransactionAssets(tx, entry.Info, entry.ID); err != nil {
		return fmt.Errorf("process assets for %s: %w", entry.ID, err)
	}
	return nil
}

// MarkTokenSyncedWithStatus clears needs_sync and re-derives token_status from the
// role of the chain's final entry. Mirrors core/wallet/fullnode_persistence.go:363-380
// in rubixgoplatform. Only these two fields are touched on the Tokens row — anything
// else is the responsibility of the per-transaction processing path (PTA).
//
// Mapping:
//
//	Mint, Transfer, Unpledge → Free
//	Pledge                   → Pledged
//	Burn                     → Burnt
//	Commit                   → Committed
//	Deploy                   → Deployed
//	Execute                  → Executed
//	Uncommit / unknown       → leave token_status unchanged
func MarkTokenSyncedWithStatus(db *gorm.DB, tokenID string, lastRole int16) error {
	if db == nil {
		db = database.WriteDB
	}

	updates := map[string]interface{}{"needs_sync": false}
	if status, ok := statusForRole(lastRole); ok {
		updates["token_status"] = status
	}

	return db.Model(&models.Token{}).
		Where("token_id = ?", tokenID).
		Updates(updates).Error
}

// statusForRole returns (status, true) for roles that map to a canonical status,
// or (_, false) for Uncommit / unknown roles where the prior status is preserved.
func statusForRole(role int16) (int16, bool) {
	switch role {
	case models.TokenRole_Mint, models.TokenRole_Transfer, models.TokenRole_Unpledge:
		return models.TokenStatus_Free, true
	case models.TokenRole_Pledge:
		return models.TokenStatus_Pledged, true
	case models.TokenRole_Burn:
		return models.TokenStatus_Burnt, true
	case models.TokenRole_Commit:
		return models.TokenStatus_Committed, true
	case models.TokenRole_Deploy:
		return models.TokenStatus_Deployed, true
	case models.TokenRole_Execute:
		return models.TokenStatus_Executed, true
	default:
		return 0, false
	}
}

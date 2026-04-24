package processor

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/database/operations"
	"explorer-server/model"
	"explorer-server/util"
	"log"

	"gorm.io/gorm"
)

// HandleIncomingTxn orchestrates the unmarshaling, validation, and enqueuing of a transaction
func HandleIncomingTxn(newEvent *model.EventTransaction) {
	if newEvent.Transaction == nil {
		log.Printf("Warning: Received transaction with nil info, skipping")
		return
	}

	// 1. Format Validation
	if ok, reason := ValidateTransactionFormatWithReason(newEvent); !ok {
		// Do not drop malformed events; force failure path so they are captured.
		newEvent.Status = false
		if newEvent.Message == "" {
			newEvent.Message = "FORMAT_VALIDATION_FAILED: " + reason
		} else {
			newEvent.Message = newEvent.Message + " | FORMAT_VALIDATION_FAILED: " + reason
		}
		log.Printf("FORMAT-FAILED: transaction=%s reason=%s", newEvent.Transaction.ID, reason)
	}

	// 2. Enqueue for processing
	if GlobalWorkerPool != nil {
		GlobalWorkerPool.EnqueueTransaction(newEvent)
	} else {
		log.Printf("Warning: GlobalWorkerPool not initialized, dropping transaction %s", newEvent.Transaction.ID)
	}
}

// ValidateTransactionFormat checks the DIDs and TokenIDs against Rubix format rules
func ValidateTransactionFormat(newEvent *model.EventTransaction) bool {
	ok, _ := ValidateTransactionFormatWithReason(newEvent)
	return ok
}

// ValidateTransactionFormatWithReason validates transaction IDs, DIDs and TokenIDs.
// It returns a boolean and failure reason so callers can route malformed events
// into failed persistence instead of dropping them.
func ValidateTransactionFormatWithReason(newEvent *model.EventTransaction) (bool, string) {
	if newEvent == nil || newEvent.Transaction == nil {
		log.Printf("ID-FORMAT-ERR: nil transaction event")
		return false, "nil transaction event"
	}
	if !util.IsValidTransactionID(newEvent.Transaction.ID) {
		log.Printf("ID-FORMAT-ERR: Invalid transaction ID format: %s", newEvent.Transaction.ID)
		return false, "invalid transaction.id"
	}
	if newEvent.TransactionID != "" && !util.IsValidTransactionID(newEvent.TransactionID) {
		log.Printf("ID-FORMAT-ERR: Invalid event transaction_id format: %s", newEvent.TransactionID)
		return false, "invalid event transaction_id"
	}
	if newEvent.TransactionID != "" && newEvent.TransactionID != newEvent.Transaction.ID {
		log.Printf("ID-FORMAT-ERR: event transaction_id does not match transaction.id: %s != %s", newEvent.TransactionID, newEvent.Transaction.ID)
		return false, "event transaction_id mismatch"
	}

	info, err := newEvent.Transaction.ParseInfo()
	if err != nil {
		log.Printf("Warning: Failed to parse transaction info for validation: %v", err)
		return false, "invalid transaction.info json"
	}
	if info == nil {
		return true, "" // No info to validate
	}

	// DID Validation
	if info.Initiator != "" && !util.IsValidDID(info.Initiator) {
		log.Printf("ID-FORMAT-ERR: Invalid Initiator DID format: %s", info.Initiator)
		return false, "invalid initiator DID"
	}
	if info.Owner != "" && !util.IsValidDID(info.Owner) {
		log.Printf("ID-FORMAT-ERR: Invalid Owner DID format: %s", info.Owner)
		return false, "invalid owner DID"
	}

	// TokenID Validation
	if info.Tokens != nil {
		for _, t := range info.Tokens.RBT {
			if !util.IsValidRBT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid RBT TokenID format: %s", t.TokenID)
				return false, "invalid RBT tokenID"
			}
			if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
				log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for RBT token %s: %s", t.TokenID, t.PreviousTransactionID)
				return false, "invalid RBT previous transaction ID"
			}
		}
		for _, t := range info.Tokens.FT {
			if !util.IsValidFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid FT TokenID format: %s", t.TokenID)
				return false, "invalid FT tokenID"
			}
			if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
				log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for FT token %s: %s", t.TokenID, t.PreviousTransactionID)
				return false, "invalid FT previous transaction ID"
			}
		}
		for _, t := range info.Tokens.NFT {
			if !util.IsValidNFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid NFT TokenID format: %s", t.TokenID)
				return false, "invalid NFT tokenID"
			}
			if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
				log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for NFT token %s: %s", t.TokenID, t.PreviousTransactionID)
				return false, "invalid NFT previous transaction ID"
			}
		}
		for _, t := range info.Tokens.SmartContract {
			if !util.IsValidSC(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid SC TokenID format: %s", t.TokenID)
				return false, "invalid SC tokenID"
			}
			if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
				log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for SC token %s: %s", t.TokenID, t.PreviousTransactionID)
				return false, "invalid SC previous transaction ID"
			}
		}
	}

	// Quorum DID and Token Validation
	for _, q := range info.Quorums {
		if !util.IsValidDID(q.Did) {
			log.Printf("ID-FORMAT-ERR: Invalid Quorum DID format: %s", q.Did)
			return false, "invalid quorum DID"
		}
		for _, t := range q.Tokens {
			if !util.IsValidRBT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid Quorum RBT TokenID format: %s", t.TokenID)
				return false, "invalid quorum tokenID"
			}
			if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
				log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for Quorum token %s: %s", t.TokenID, t.PreviousTransactionID)
				return false, "invalid quorum previous transaction ID"
			}
		}
	}

	// Committed Tokens Validation (Must be RBTs)
	for _, t := range info.CommittedTokens {
		if !util.IsValidRBT(t.TokenID) {
			log.Printf("ID-FORMAT-ERR: Invalid Committed RBT TokenID format: %s", t.TokenID)
			return false, "invalid committed tokenID"
		}
		if t.PreviousTransactionID != "" && !util.IsValidTransactionID(t.PreviousTransactionID) {
			log.Printf("ID-FORMAT-ERR: Invalid previous transaction ID for committed token %s: %s", t.TokenID, t.PreviousTransactionID)
			return false, "invalid committed previous transaction ID"
		}
	}

	return true, ""
}

// ProcessDBTransaction handles the actual logic of logging/inserting to DB
// Called by workers in the DynamicWorkerPool
func ProcessDBTransaction(newEvent *model.EventTransaction, workerID int) {
	txnID := newEvent.Transaction.ID
	log.Printf("[Worker %d] Processing transaction: %s (Status: %v)", workerID, txnID, newEvent.Status)

	// 1. Save EventTransaction always (backward-compatible behavior).
	if err := operations.SaveEventTransaction(
		nil, // Use default DB
		txnID,
		newEvent.Status,
		newEvent.Message,
	); err != nil {
		log.Printf("[Worker %d] Error saving event transaction: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 1: EventTransaction saved", workerID)
	}

	// 2. Save Transaction as JSON always.
	dbTxn := &models.Transactions{
		ID:        txnID,
		Info:      newEvent.Transaction.Info,
		Signature: newEvent.Transaction.Signature,
	}
	if err := operations.SaveTransaction(nil, dbTxn); err != nil {
		log.Printf("[Worker %d] Error saving raw transaction: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 2: raw Transaction saved", workerID)
	}

	// Parse the raw Info JSON into structured TransactionInfo
	txnInfo, err := newEvent.Transaction.ParseInfo()
	if err != nil {
		log.Printf("[Worker %d] Error parsing transaction info: %v", workerID, err)
		return
	}
	if txnInfo == nil {
		log.Printf("[Worker %d] Transaction %s has no TransactionInfo, skipping details", workerID, txnID)
		return
	}

	// Failed transactions are written to one table only: FailedTransactionInfo.
	if !newEvent.Status {
		if err := operations.SaveTransactionDetails(nil, txnID, txnInfo, false); err != nil {
			log.Printf("[Worker %d] Error saving failed transaction details: %v", workerID, err)
		} else {
			log.Printf("[Worker %d] Failed transaction %s stored in FailedTransactionInfo", workerID, txnID)
		}
		return
	}

	// 3. Save flattened TransactionInfo
	if err := operations.SaveTransactionDetails(nil, txnID, txnInfo, true); err != nil {
		log.Printf("[Worker %d] Error saving transaction details: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 3: TransactionInfo saved (Status: true)", workerID)
	}

	// 4. Process Assets (Tokens, Balances, Provenance) — ONLY for successful transactions
	// This block is ATOMIC: either all assets update or none.
	log.Printf("[Worker %d] Step 4: Starting Atomic Asset Processing...", workerID)
	err = database.WriteDB.Transaction(func(tx *gorm.DB) error {
		return operations.ProcessTransactionAssets(tx, txnInfo, txnID)
	})

	if err != nil {
		log.Printf("[Worker %d] Error in atomic asset processing: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 4: Asset Processing completed successfully", workerID)
	}

	log.Printf("[Worker %d] Transaction %s finished successfully", workerID, txnID)
}

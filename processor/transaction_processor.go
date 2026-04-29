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
	if !ValidateTransactionFormat(newEvent) {
		return
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
	info, err := newEvent.Transaction.ParseInfo()
	if err != nil {
		log.Printf("Warning: Failed to parse transaction info for validation: %v", err)
		return false
	}
	if info == nil {
		return true // No info to validate
	}

	// DID Validation
	if info.Initiator != "" && !util.IsValidDID(info.Initiator) {
		log.Printf("ID-FORMAT-ERR: Invalid Initiator DID format: %s", info.Initiator)
		return false
	}
	if info.Owner != "" && !util.IsValidDID(info.Owner) {
		log.Printf("ID-FORMAT-ERR: Invalid Owner DID format: %s", info.Owner)
		return false
	}

	// TokenID Validation
	if info.Tokens != nil {
		for _, t := range info.Tokens.RBT {
			if !util.IsValidRBT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid RBT TokenID format: %s", t.TokenID)
				return false
			}
		}
		for _, t := range info.Tokens.FT {
			if !util.IsValidFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid FT TokenID format: %s", t.TokenID)
				return false
			}
		}
		for _, t := range info.Tokens.NFT {
			if !util.IsValidNFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid NFT TokenID format: %s", t.TokenID)
				return false
			}
		}
		for _, t := range info.Tokens.SmartContract {
			if !util.IsValidSC(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid SC TokenID format: %s", t.TokenID)
				return false
			}
		}
	}

	// Quorum DID and Token Validation
	for _, q := range info.Quorums {
		if !util.IsValidDID(q.Did) {
			log.Printf("ID-FORMAT-ERR: Invalid Quorum DID format: %s", q.Did)
			return false
		}
		if q.Tokens != nil {
			for _, t := range q.Tokens {
				if !util.IsValidRBT(t.TokenID) {
					log.Printf("ID-FORMAT-ERR: Invalid Quorum RBT TokenID format: %s", t.TokenID)
					return false
				}
			}
		}
	}

	// Quorum Signature DID Validation
	sig, err := newEvent.Transaction.ParseSignature()
	if err == nil && sig != nil {
		for _, q := range sig.Quorums {
			if !util.IsValidDID(q.Did) {
				log.Printf("ID-FORMAT-ERR: Invalid Quorum Signature DID format: %s", q.Did)
				return false
			}
		}
	}

	// Committed Tokens Validation (Must be RBTs)
	for _, t := range info.CommittedTokens {
		if !util.IsValidRBT(t.TokenID) {
			log.Printf("ID-FORMAT-ERR: Invalid Committed RBT TokenID format: %s", t.TokenID)
			return false
		}
	}

	return true
}

// ProcessDBTransaction handles the actual logic of logging/inserting to DB
// Called by workers in the DynamicWorkerPool
func ProcessDBTransaction(newEvent *model.EventTransaction, workerID int) {
	txnID := newEvent.Transaction.ID

	// 1. Save EventTransaction (captures consensus result)
	if err := operations.SaveEventTransaction(nil, txnID, newEvent.Status, newEvent.Message); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save event: %v", workerID, txnID, err)
	}

	// Parse Info
	txnInfo, err := newEvent.Transaction.ParseInfo()
	if err != nil || txnInfo == nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to parse info: %v", workerID, txnID, err)
		return
	}

	// 2. Save Raw Transaction
	if err := operations.SaveTransaction(nil, &models.Transactions{ID: txnID, Info: newEvent.Transaction.Info, Signature: newEvent.Transaction.Signature}); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save raw txn: %v", workerID, txnID, err)
	}

	// 3. Save Flattened Details
	if err := operations.SaveTransactionDetails(nil, txnID, txnInfo, newEvent.Status); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save details: %v", workerID, txnID, err)
	}

	// 4. Atomic Asset Processing (only for successful consensus)
	if newEvent.Status {
		err = database.WriteDB.Transaction(func(tx *gorm.DB) error {
			return operations.ProcessTransactionAssets(tx, txnInfo, txnID)
		})
		if err != nil {
			log.Printf("[Worker %d] ERROR: Transaction %s - Asset processing failed: %v", workerID, txnID, err)
			return
		}
	}

	// ONE SUMMARY LOG: Success
	log.Printf("[Worker %d] SUCCESS: Transaction %s processed (Status: %v)", workerID, txnID, newEvent.Status)
}

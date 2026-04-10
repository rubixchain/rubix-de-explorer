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
		for _, t := range q.Tokens {
			if !util.IsValidRBT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid Quorum RBT TokenID format: %s", t.TokenID)
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

	// Quorum Signatures DID Validation
	sig, _ := newEvent.Transaction.ParseSignature()
	if sig != nil {
		for _, qs := range sig.Quorums {
			if !util.IsValidDID(qs.Did) {
				log.Printf("ID-FORMAT-ERR: Invalid Quorum Signature DID format: %s", qs.Did)
				return false
			}
		}
	}

	return true
}

// ProcessDBTransaction handles the actual logic of logging/inserting to DB
// Called by workers in the DynamicWorkerPool
func ProcessDBTransaction(newEvent *model.EventTransaction, workerID int) {
	txnID := newEvent.Transaction.ID
	log.Printf("[Worker %d] Processing transaction: %s (Status: %v)", workerID, txnID, newEvent.Status)

	// 1. Save EventTransaction (always — captures both success and failed consensus)
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

	// 2. Save Transaction as JSON (always) — pass raw bytes directly
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

	// 3. Save flattened TransactionInfo (to success or failure table)
	if err := operations.SaveTransactionDetails(nil, txnID, txnInfo, newEvent.Status); err != nil {
		log.Printf("[Worker %d] Error saving transaction details: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 3: TransactionInfo saved (Status: %v)", workerID, newEvent.Status)
	}

	// If consensus failed, stop here (don't update assets/balances/tokens)
	if !newEvent.Status {
		log.Printf("[Worker %d] Transaction %s failed consensus, skipping asset processing", workerID, txnID)
		return
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

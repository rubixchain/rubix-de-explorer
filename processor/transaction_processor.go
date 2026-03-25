package processor

import (
	"encoding/json"
	"explorer-server/database/models"
	"explorer-server/database/operations"
	"explorer-server/model"
	"explorer-server/util"
	"log"
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
		log.Printf("Warning: GlobalWorkerPool not initialized, dropping transaction %s", newEvent.Transaction.TransactionID)
	}
}

// ValidateTransactionFormat checks the DIDs and TokenIDs against Rubix format rules
func ValidateTransactionFormat(newEvent *model.EventTransaction) bool {
	info := newEvent.Transaction.TransactionInfo
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
	if newEvent.Transaction.Signatures != nil {
		for _, qs := range newEvent.Transaction.Signatures.Quorums {
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
	log.Printf("[Worker %d] Processing transaction: %s (Status: %v)", workerID, newEvent.Transaction.TransactionID, newEvent.Status)

	// 1. Save EventTransaction (always — captures both success and failed consensus)
	if err := operations.SaveEventTransaction(
		newEvent.Transaction.TransactionID,
		newEvent.Status,
		newEvent.Message,
	); err != nil {
		log.Printf("[Worker %d] Error saving event transaction: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 1: EventTransaction saved", workerID)
	}

	// If consensus failed, log it and skip token/balance processing
	if !newEvent.Status {
		log.Printf("[Worker %d] Transaction %s failed consensus: %s", workerID, newEvent.Transaction.TransactionID, newEvent.Message)
		return
	}

	txnInfo := newEvent.Transaction.TransactionInfo
	if txnInfo == nil {
		log.Printf("[Worker %d] Transaction %s has no TransactionInfo, skipping assets", workerID, newEvent.Transaction.TransactionID)
		return
	}

	// 2. Save Transaction as JSON (aligned with Rubix node's Transactions table)
	dbTxn := convertToDBTransaction(newEvent)
	if dbTxn != nil {
		if err := operations.SaveTransaction(dbTxn); err != nil {
			log.Printf("[Worker %d] Error saving raw transaction: %v", workerID, err)
		} else {
			log.Printf("[Worker %d] Step 2: raw Transaction saved", workerID)
		}
	}

	// 3. Save flattened TransactionInfo for frontend filtering
	if err := operations.SaveTransactionDetails(newEvent.Transaction.TransactionID, txnInfo); err != nil {
		log.Printf("[Worker %d] Error saving transaction details: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 3: flattened TransactionInfo saved", workerID)
	}

	// 4. Process Assets (Tokens, Balances, Provenance)
	log.Printf("[Worker %d] Step 4: Starting Asset Processing...", workerID)
	if err := operations.ProcessTransactionAssets(txnInfo, newEvent.Transaction.TransactionID); err != nil {
		log.Printf("[Worker %d] Error processing transaction assets: %v", workerID, err)
	} else {
		log.Printf("[Worker %d] Step 4: Asset Processing completed successfully", workerID)
	}

	log.Printf("[Worker %d] Transaction %s finished successfully", workerID, newEvent.Transaction.TransactionID)
}

// convertToDBTransaction serializes the DTO into the JSON-based Transactions table model
func convertToDBTransaction(event *model.EventTransaction) *models.Transactions {
	infoJSON, err := json.Marshal(event.Transaction.TransactionInfo)
	if err != nil {
		log.Printf("   Error marshaling transaction info: %v", err)
		return nil
	}

	sigJSON, err := json.Marshal(event.Transaction.Signatures)
	if err != nil {
		log.Printf("   Error marshaling signatures: %v", err)
		return nil
	}

	return &models.Transactions{
		ID:        event.Transaction.TransactionID,
		Info:      infoJSON,
		Signature: sigJSON,
	}
}

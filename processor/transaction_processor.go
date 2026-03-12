package processor

import (
	"encoding/json"
	"explorer-server/database/models"
	"explorer-server/util"
	"log"
)

// HandleIncomingTxn orchestrates the unmarshaling, validation, and enqueuing of a transaction
func HandleIncomingTxn(newEvent *models.EventTransaction) {
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
func ValidateTransactionFormat(newEvent *models.EventTransaction) bool {
	info := newEvent.Transaction.TransactionInfo
	if info == nil {
		return true // No info to validate
	}

	// DID Validation
	if info.Initiator != nil && !util.IsValidDID(*info.Initiator) {
		log.Printf("ID-FORMAT-ERR: Invalid Initiator DID format: %s", *info.Initiator)
		return false
	}
	if info.Owner != nil && !util.IsValidDID(*info.Owner) {
		log.Printf("ID-FORMAT-ERR: Invalid Owner DID format: %s", *info.Owner)
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

	return true
}

// ProcessDBTransaction handles the actual logic of logging/inserting to DB
// Called by workers in the DynamicWorkerPool
func ProcessDBTransaction(newEvent *models.EventTransaction, workerID int) {
	log.Printf("[Worker %d] Processing valid transaction from queue:", workerID)
	
	// Print the entire struct as JSON for debugging
	fullData, err := json.MarshalIndent(newEvent, "", "  ")
	if err == nil {
		log.Printf("\n%s", string(fullData))
	} else {
		log.Printf("   Error marshaling struct for logging: %v", err)
	}

	// TODO: IMPLEMENT DB UPDATES HERE
}

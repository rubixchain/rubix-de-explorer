package operations

import (
	"encoding/json"
	"log"

	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"gorm.io/gorm"
)

// RepairMissingAssetsFromTransactionInfo scans all successful transactions in the TransactionInfo table
// and re-processes any FT or NFT tokens that were skipped due to previous regex/parsing bugs.
// It is designed to be safe: it checks if the token-transaction pair already exists in the TokenChain
// before attempting any work, ensuring no double-counting of balances.
func RepairMissingAssetsFromTransactionInfo() error {
	log.Println("[Repair] Starting repair process for missing FT/NFT assets...")

	return database.WriteDB.Transaction(func(tx *gorm.DB) error {
		var txns []models.TransactionInfo
		// Only look at successful transactions
		if err := tx.Where("status = ?", true).Find(&txns).Error; err != nil {
			return err
		}

		log.Printf("[Repair] Found %d successful transactions to audit", len(txns))

		repairedCount := 0

		for _, txn := range txns {
			var tokens model.TransactionTokens
			if err := json.Unmarshal(txn.Tokens, &tokens); err != nil {
				continue
			}

			// Identify all FT, NFT and SmartContract tokens in this transaction
			assetsToCheck := make([]*model.TokenInfo, 0)
			assetsToCheck = append(assetsToCheck, tokens.FT...)
			assetsToCheck = append(assetsToCheck, tokens.NFT...)
			assetsToCheck = append(assetsToCheck, tokens.SmartContract...)

			if len(assetsToCheck) == 0 {
				continue
			}

			for _, asset := range assetsToCheck {
				// Safety check: skip if this specific asset for this specific txn is already in our history
				var count int64
				tx.Model(&models.TokenChain{}).Where("token_id = ? AND transaction_id = ?", asset.TokenID, txn.TransactionID).Count(&count)
				if count > 0 {
					continue
				}

				log.Printf("[Repair] Found missing asset %s in successful txn %s. Repairing...", asset.TokenID, txn.TransactionID)
				
				// Create a surgical partial TransactionInfo containing ONLY this missing asset.
				// This ensures ProcessTransactionAssets does not re-process RBTs or other already-recorded tokens.
				partialInfo := &model.TransactionInfo{
					Initiator: txn.Initiator,
					Owner:     txn.Owner,
					Epoch:     txn.Epoch,
					Network:   txn.Network,
					Tokens:    &model.TransactionTokens{},
				}
				
				if util.IsValidFT(asset.TokenID) {
					partialInfo.Tokens.FT = []*model.TokenInfo{asset}
				} else if util.IsValidNFT(asset.TokenID) {
					partialInfo.Tokens.NFT = []*model.TokenInfo{asset}
				} else if util.IsValidSC(asset.TokenID) {
					partialInfo.Tokens.SmartContract = []*model.TokenInfo{asset}
				} else {
					continue
				}

				// Re-run the (now fixed) ingestion logic for this specific missing asset
				if err := ProcessTransactionAssets(tx, partialInfo, txn.TransactionID); err != nil {
					log.Printf("[Repair] ERROR: Failed to repair asset %s: %v", asset.TokenID, err)
					continue
				}
				
				repairedCount++
			}
		}

		log.Printf("[Repair] Repair process completed. Successfully recovered %d assets.", repairedCount)
		return nil
	})
}

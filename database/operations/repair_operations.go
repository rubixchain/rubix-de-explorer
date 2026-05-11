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

		// Diagnostic 1: Search for "daisy" in the raw JSON to see if it's there at all
		var countDaisy int64
		tx.Table("TransactionInfo").Where("tokens::text LIKE ?", "%daisy%").Count(&countDaisy)
		log.Printf("[Repair] Diagnostic: Found %d transactions containing 'daisy' in raw JSON", countDaisy)

		// Diagnostic 2: Use JSONB operators to count non-empty arrays
		var countFT, countNFT, countSC int64
		tx.Table("TransactionInfo").Where("tokens->'ft' IS NOT NULL AND tokens->>'ft' != 'null' AND jsonb_array_length(tokens->'ft') > 0").Count(&countFT)
		tx.Table("TransactionInfo").Where("tokens->'nft' IS NOT NULL AND tokens->>'nft' != 'null' AND jsonb_array_length(tokens->'nft') > 0").Count(&countNFT)
		tx.Table("TransactionInfo").Where("tokens->'smartContract' IS NOT NULL AND tokens->>'smartContract' != 'null' AND jsonb_array_length(tokens->'smartContract') > 0").Count(&countSC)
		
		log.Printf("[Repair] Diagnostic: JSONB counts -> FT: %d, NFT: %d, SC: %d", countFT, countNFT, countSC)

		repairedCount := 0

		for i, txn := range txns {
			var tokens model.TransactionTokens
			if err := json.Unmarshal(txn.Tokens, &tokens); err != nil {
				if i < 10 { // Log only first 10 failures to avoid spam
					log.Printf("[Repair] Debug: JSON Unmarshal failed for txn %s: %v", txn.TransactionID, err)
				}
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

			log.Printf("[Repair] Found txn %s with %d assets to check (FT:%d, NFT:%d, SC:%d)", 
				txn.TransactionID, len(assetsToCheck), len(tokens.FT), len(tokens.NFT), len(tokens.SmartContract))

			for _, asset := range assetsToCheck {
				// 1. Check if this specific asset for this specific txn is already in our history
				var existingChain models.TokenChain
				err := tx.Where("token_id = ? AND transaction_id = ?", asset.TokenID, txn.TransactionID).First(&existingChain).Error
				
				if err == gorm.ErrRecordNotFound {
					// FULL REPAIR: Missing from history, re-process entirely
					log.Printf("[Repair] Found missing asset %s in successful txn %s. Repairing...", asset.TokenID, txn.TransactionID)
					
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

					if err := ProcessTransactionAssets(tx, partialInfo, txn.TransactionID); err != nil {
						log.Printf("[Repair] ERROR: Failed to repair asset %s: %v", asset.TokenID, err)
						continue
					}
					repairedCount++
				} else if err == nil {
					// REFRESH: Exists in history, but check if Token table needs value/deployer fix
					var tokenEntry models.Token
					if err := tx.Where("token_id = ?", asset.TokenID).First(&tokenEntry).Error; err == nil {
						// Only refresh if Value is 0 or Deployer is missing
						if (tokenEntry.TokenValue == 0 && asset.TokenValue > 0) || (tokenEntry.DeployerDID == "" && asset.PreviousTransactionID == "") {
							log.Printf("[Repair] Refreshing incomplete token entry for %s (updating value/deployer)", asset.TokenID)
							
							// If value is changing, we need to update the balance of the CURRENT owner
							if tokenEntry.TokenValue == 0 && asset.TokenValue > 0 {
								typeName := tokenTypeName(tokenEntry.TokenType)
								// Add the value to the current owner's balance
								if err := updateBalances(tx, tokenEntry.DID, typeName, "", "", asset.TokenValue, 0); err != nil {
									log.Printf("[Repair] Failed to update balance during refresh: %v", err)
								}
							}
							
							// Update the Token table entry
							updates := make(map[string]interface{})
							if asset.TokenValue > 0 {
								updates["token_value"] = asset.TokenValue
							}
							if tokenEntry.DeployerDID == "" && asset.PreviousTransactionID == "" {
								updates["deployer_did"] = txn.Initiator
							}
							
							if len(updates) > 0 {
								tx.Model(&tokenEntry).Updates(updates)
							}
						}
					}
				}
			}
		}

		log.Printf("[Repair] Repair process completed. Successfully recovered %d assets.", repairedCount)
		return nil
	})
}

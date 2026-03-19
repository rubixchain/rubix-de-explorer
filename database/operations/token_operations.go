package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"gorm.io/gorm"
	"strings"
	"time"
)

// Token type constants (aligned with Rubix node's token_type IDs)
const (
	TokenTypeRBT int16 = 1
	TokenTypeFT  int16 = 2
	TokenTypeNFT int16 = 3
	TokenTypeSC  int16 = 4
)

// tokenTypeMap maps human-readable names to int16 codes
var tokenTypeMap = map[string]int16{
	"RBT": TokenTypeRBT,
	"FT":  TokenTypeFT,
	"NFT": TokenTypeNFT,
	"SC":  TokenTypeSC,
}

// UpdateTokenAndBalances atomically updates the token state and increments/decrements DID balances
func UpdateTokenAndBalances(token *models.Token, prevOwner string) error {
	return database.WriteDB.Transaction(func(tx *gorm.DB) error {
		// 1. Upsert Token
		if err := tx.Save(token).Error; err != nil {
			return err
		}

		// 2. Decrement balance for previous owner (if exists)
		if prevOwner != "" && prevOwner != "0" {
			if err := updateBalances(tx, prevOwner, token, -1); err != nil {
				return err
			}
		}

		// 3. Increment balance for new owner
		if token.DID != "" && token.DID != "0" {
			if err := updateBalances(tx, token.DID, token, 1); err != nil {
				return err
			}
		}

		return nil
	})
}

// ProcessTransactionAssets orchestrates the DB updates for all assets in a transaction
func ProcessTransactionAssets(txn *model.TransactionInfo, txnID string) error {
	return database.WriteDB.Transaction(func(tx *gorm.DB) error {
		if txn.Tokens == nil {
			return nil
		}

		// Helper to process a slice of tokens
		processTokens := func(tokenInfos []*model.TokenInfo, tokenType string) error {
			typeID := tokenTypeMap[tokenType]
			for _, info := range tokenInfos {
				
				var existing models.Token
				err := tx.Where("token_id = ?", info.TokenID).First(&existing).Error
				
				isNew := (err == gorm.ErrRecordNotFound)
				if err != nil && !isNew {
					return err
				}

				var prevOwner string
				var tokenToSave models.Token

				if isNew {
					// Token appears in Explorer without prior history
					tokenToSave = models.Token{
						TokenID:       info.TokenID,
						TokenType:     typeID,
						DID:           txn.Owner,
						TransactionID: txnID,
						TokenStatus:   1, // Active
						NeedsSync:     true, // Flag for the background job to fetch true history from node
					}
					// Always assign Data if present (NFTs, Smart Contracts, etc.)
					if info.Data != "" {
						tokenToSave.Data = info.Data
					}

					// For RBT, handle whole vs part tokens
					if typeID == TokenTypeRBT {
						// Simple check: part RBTs contain more than one underscore (e.g. 1_1000_5)
						// We'll set a placeholder, but real value comes from node sync
						tokenToSave.TokenValue = 1.0 
					}
				} else {
					// Update existing token
					prevOwner = existing.DID
					tokenToSave = existing

					// Detect history gaps/mismatches
					if info.PreviousTransactionID != "" && info.PreviousTransactionID != existing.TransactionID {
						tokenToSave.NeedsSync = true
					}

					tokenToSave.DID = txn.Owner
					tokenToSave.TransactionID = txnID
					tokenToSave.TokenStatus = 1
					
					// Update Data if new data is provided
					if info.Data != "" {
						tokenToSave.Data = info.Data
					}
				}

				// 1. Write the Token updates to the Token Table First
				if err := tx.Save(&tokenToSave).Error; err != nil {
					return err
				}

				// 2. Append to Token History (with sequencing via PreviousTransactionID)
				if err := appendTokenHistory(tx, info.TokenID, txnID, info.PreviousTransactionID, 0); err != nil {
					return err
				}

				// 3. Update Balances LAST, relying on the token table exactly.
				if prevOwner != "" && prevOwner != "0" {
					if err := updateBalances(tx, prevOwner, &tokenToSave, -1); err != nil {
						return err
					}
				}
				if tokenToSave.DID != "" && tokenToSave.DID != "0" {
					if err := updateBalances(tx, tokenToSave.DID, &tokenToSave, 1); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Process each type
		if txn.Tokens.RBT != nil {
			if err := processTokens(txn.Tokens.RBT, "RBT"); err != nil {
				return err
			}
		}
		if txn.Tokens.FT != nil {
			if err := processTokens(txn.Tokens.FT, "FT"); err != nil {
				return err
			}
		}
		if txn.Tokens.NFT != nil {
			if err := processTokens(txn.Tokens.NFT, "NFT"); err != nil {
				return err
			}
		}
		if txn.Tokens.SmartContract != nil {
			if err := processTokens(txn.Tokens.SmartContract, "SC"); err != nil {
				return err
			}
		}

		return nil
	})
}

// tokenTypeName maps int16 to string for balance lookups
func tokenTypeName(t int16) string {
	switch t {
	case TokenTypeRBT:
		return "RBT"
	case TokenTypeFT:
		return "FT"
	case TokenTypeNFT:
		return "NFT"
	case TokenTypeSC:
		return "SC"
	default:
		return "UNKNOWN"
	}
}

// updateBalances handles DIDBalances (granular) table
func updateBalances(tx *gorm.DB, did string, token *models.Token, direction float64) error {
	now := time.Now().Unix()

	typeName := tokenTypeName(token.TokenType)

	// Determine value to add (TokenValue for RBT, 1 for others)
	var valueToAdd float64
	if token.TokenType == TokenTypeRBT {
		valueToAdd = token.TokenValue * direction
	} else {
		valueToAdd = 1.0 * direction
	}

	// Token Name: extract from TokenID for FTs
	assetName := typeName
	if token.TokenType == TokenTypeFT {
		parts := strings.Split(token.TokenID, "_")
		if len(parts) > 0 && parts[0] != "" {
			assetName = parts[0]
		}
	}

	var balance models.DIDBalance
	err := tx.Where("did = ? AND asset_type = ? AND token_name = ?", did, typeName, assetName).First(&balance).Error
	if err == gorm.ErrRecordNotFound {
		balance = models.DIDBalance{
			DID:        did,
			AssetType:  typeName,
			TokenName:  assetName,
			Balance:    valueToAdd,
			LastUpdate: now,
		}
		return tx.Create(&balance).Error
	} else if err != nil {
		return err
	}

	balance.Balance += valueToAdd
	balance.LastUpdate = now

	return tx.Save(&balance).Error
}

// appendTokenHistory records the token's movement and ensures the TokenChainArray is logically sequenced
func appendTokenHistory(tx *gorm.DB, tokenID, txnID, prevTxnID string, role int16) error {
	// 1. Insert into TokenChain
	historyEntry := models.TokenChain{
		TokenID:               tokenID,
		TransactionID:         txnID,
		PreviousTransactionID: prevTxnID,
		Role:                  role,
	}
	if err := tx.Create(&historyEntry).Error; err != nil {
		return err
	}

	// 2. Load existing TokenChainArray
	var tca models.TokenChainArray
	err := tx.Where("token_id = ?", tokenID).First(&tca).Error
	
	var chain []uint64
	if err == nil {
		json.Unmarshal(tca.Index, &chain)
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	// 3. Determine logical position
	newID := historyEntry.ID
	inserted := false

	if prevTxnID != "" {
		// Find the ID of the parent record in TokenChain
		var parent models.TokenChain
		if tx.Where("token_id = ? AND transaction_id = ?", tokenID, prevTxnID).Select("id").First(&parent).Error == nil {
			// Find parent position in current array
			for i, id := range chain {
				if id == parent.ID {
					// Insert after parent
					chain = append(chain[:i+1], append([]uint64{newID}, chain[i+1:]...)...)
					inserted = true
					break
				}
			}
		}
	}

	// If not inserted (root token, parent not yet in DB, or prevTxnID empty), append to end
	if !inserted {
		if prevTxnID == "" {
			// Root/Genesis: Put at start if empty, else append (should be first anyway)
			if len(chain) == 0 {
				chain = []uint64{newID}
			} else {
				// Insert at beginning for genesis
				chain = append([]uint64{newID}, chain...)
			}
		} else {
			// Out-of-order or unknown parent: just append for now
			chain = append(chain, newID)
		}
	}

	chainJSON, _ := json.Marshal(chain)

	// 4. Upsert TokenChainArray
	if err == gorm.ErrRecordNotFound {
		tca = models.TokenChainArray{
			TokenID: tokenID,
			Index:   chainJSON,
		}
		return tx.Create(&tca).Error
	}

	return tx.Model(&tca).Update("index", chainJSON).Error
}

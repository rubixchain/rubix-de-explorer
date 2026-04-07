package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

		// 2. Update Balances
		// Special Case: Smart Contracts (SC). Balance counts deployments/mints only.
		if token.TokenType == TokenTypeSC {
			if (prevOwner == "" || prevOwner == "0") && token.TokenStatus != models.TokenStatus_Burnt {
				// If it's new (Mint/Deploy), increment for the owner
				if token.DID != "" && token.DID != "0" {
					if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", 1); err != nil {
						return err
					}
				}
			}
			// SC Interaction (Execute) or Burn does not transfer balance for SCs.
		} else if prevOwner != token.DID {
			// Standard Logic: RBT, FT, NFT transfer ownership/balances
			if prevOwner != "" && prevOwner != "0" {
				if err := updateBalances(tx, prevOwner, tokenTypeName(token.TokenType), "", "", -1); err != nil {
					return err
				}
			}
			if token.DID != "" && token.DID != "0" {
				if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", 1); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// ProcessTransactionAssets orchestrates the DB updates for all assets in a transaction in a highly scalable bulk-oriented way.
func ProcessTransactionAssets(db *gorm.DB, txn *model.TransactionInfo, txnID string) error {
	// If no transaction provided, start one (failsafe, though processor should provide one)
	if db == nil {
		return database.WriteDB.Transaction(func(tx *gorm.DB) error {
			return ProcessTransactionAssets(tx, txn, txnID)
		})
	}

	tx := db

	if txn.Tokens == nil && txn.Quorums == nil && txn.CommittedTokens == nil {
		return nil
	}

	// 1. Collect all involved TokenIDs to fetch existing state in one query
	tokenIDSet := make(map[string]struct{})
	collectIDs := func(tokenInfos []*model.TokenInfo) {
		for _, info := range tokenInfos {
			tokenIDSet[info.TokenID] = struct{}{}
		}
	}
	if txn.Tokens != nil {
		collectIDs(txn.Tokens.RBT)
		collectIDs(txn.Tokens.FT)
		collectIDs(txn.Tokens.NFT)
		collectIDs(txn.Tokens.SmartContract)
	}
	for _, q := range txn.Quorums {
		for _, t := range q.Tokens {
			tokenIDSet[t.TokenID] = struct{}{}
		}
	}
	for _, ct := range txn.CommittedTokens {
		tokenIDSet[ct.TokenID] = struct{}{}
	}

	allTokenIDs := make([]string, 0, len(tokenIDSet))
	for id := range tokenIDSet {
		allTokenIDs = append(allTokenIDs, id)
	}

	// 2. Fetch existing tokens
	existingTokens := make(map[string]models.Token)
	if len(allTokenIDs) > 0 {
		var tokens []models.Token
		if err := tx.Where("token_id IN ?", allTokenIDs).Find(&tokens).Error; err != nil {
			return err
		}
		for _, t := range tokens {
			existingTokens[t.TokenID] = t
		}
	}

	// 3. Balance Aggregation Map: DID -> Net Change for this transaction
	// Since one DID might send/receive multiple tokens in one txn, we aggregate first.
	type balanceKey struct {
		DID        string
		AssetType  string
		TokenName  string
		CreatorDID string
	}
	balanceChanges := make(map[balanceKey]float64)

	addBalanceChange := func(did string, token *models.Token, direction float64) {
		if did == "" || did == "0" {
			return
		}
		typeName := tokenTypeName(token.TokenType)
		var val float64
		if token.TokenType == TokenTypeRBT {
			val = token.TokenValue
		} else {
			val = 1.0
		}

		key := balanceKey{DID: did, AssetType: typeName}
		if token.TokenType == TokenTypeFT {
			parts := strings.Split(token.TokenID, "_")
			if len(parts) >= 3 {
				key.TokenName = parts[0]
				key.CreatorDID = parts[len(parts)-1]
			} else if len(parts) > 0 {
				key.TokenName = parts[0]
			}
		}
		balanceChanges[key] += (val * direction)
	}

	tokensToUpsert := make([]models.Token, 0)
	chainsToInsert := make([]models.TokenChain, 0)

	// Sub-function to generate models for a group of tokens
	prepareTokens := func(tokenInfos []*model.TokenInfo, tokenType string) error {
		typeID := tokenTypeMap[tokenType]
		for _, info := range tokenInfos {
			existing, exists := existingTokens[info.TokenID]
			isNew := !exists
			var prevOwner string
			var tokenToSave models.Token

			if isNew {
				tokenToSave = models.Token{
					TokenID:       info.TokenID,
					TokenType:     typeID,
					TransactionID: txnID,
					TokenStatus:   models.TokenStatus_Free,
					NeedsSync:     true,
				}
				if typeID == TokenTypeSC {
					tokenToSave.DID = txn.Initiator
					tokenToSave.DeployerDID = txn.Initiator
				} else {
					tokenToSave.DID = txn.Owner
				}
				if info.Data != "" {
					tokenToSave.Data = info.Data
				}
				if typeID == TokenTypeRBT {
					val, _ := util.GetTokenValueFromTokenID(info.TokenID)
					tokenToSave.TokenValue = val
				} else {
					// FT/NFT/SC value calculation logic (kept legacy logic)
					var burnedSum float64
					for _, ct := range txn.CommittedTokens {
						if !strings.Contains(ct.TokenID, "Qm") && !strings.Contains(ct.TokenID, "_DID") && strings.Contains(ct.TokenID, "_") {
							v, _ := util.GetTokenValueFromTokenID(ct.TokenID)
							burnedSum += v
						}
					}
					if typeID == TokenTypeFT {
						ftCount := len(txn.Tokens.FT)
						if ftCount > 0 {
							tokenToSave.TokenValue = burnedSum / float64(ftCount)
						} else {
							tokenToSave.TokenValue = 1.0
						}
					} else {
						tokenToSave.TokenValue = burnedSum
					}
				}
				tokenToSave.LatestRole = models.TokenRole_Mint
				if typeID == TokenTypeSC {
					tokenToSave.LatestRole = models.TokenRole_Deploy
				}
			} else {
				prevOwner = existing.DID
				tokenToSave = existing
				if info.PreviousTransactionID != "" && info.PreviousTransactionID != existing.TransactionID {
					tokenToSave.NeedsSync = true
				}
				targetOwner := txn.Owner
				if typeID == TokenTypeSC && targetOwner == "" {
					targetOwner = existing.DID
				}
				tokenToSave.DID = targetOwner
				tokenToSave.TransactionID = txnID
				tokenToSave.TokenStatus = models.TokenStatus_Free
				if info.Data != "" {
					tokenToSave.Data = info.Data
				}
				inferredRole := models.TokenRole_Transfer
				if typeID == TokenTypeSC {
					inferredRole = models.TokenRole_Execute
				}
				tokenToSave.LatestRole = inferredRole
			}

			tokensToUpsert = append(tokensToUpsert, tokenToSave)
			chainsToInsert = append(chainsToInsert, models.TokenChain{
				TokenID:               info.TokenID,
				TransactionID:         txnID,
				PreviousTransactionID: info.PreviousTransactionID,
				Role:                  tokenToSave.LatestRole,
			})

			// Aggregate Balance Changes
			if typeID == TokenTypeSC {
				if isNew && tokenToSave.DID != "" && tokenToSave.DID != "0" {
					addBalanceChange(tokenToSave.DID, &tokenToSave, 1)
				}
			} else if prevOwner != tokenToSave.DID {
				addBalanceChange(prevOwner, &tokenToSave, -1)
				addBalanceChange(tokenToSave.DID, &tokenToSave, 1)
			}
		}
		return nil
	}

	// 4. Group Processing (Phase 2: Model Prep)
	if txn.Tokens != nil {
		prepareTokens(txn.Tokens.RBT, "RBT")
		prepareTokens(txn.Tokens.FT, "FT")
		prepareTokens(txn.Tokens.NFT, "NFT")
		prepareTokens(txn.Tokens.SmartContract, "SC")
	}

	// 5. Quorum Pledges Processing
	for _, q := range txn.Quorums {
		for _, info := range q.Tokens {
			if t, exists := existingTokens[info.TokenID]; exists {
				t.LatestRole = models.TokenRole_Pledge
				t.TransactionID = txnID
				t.TokenStatus = models.TokenStatus_Pledged
				tokensToUpsert = append(tokensToUpsert, t)
				chainsToInsert = append(chainsToInsert, models.TokenChain{
					TokenID:               info.TokenID,
					TransactionID:         txnID,
					PreviousTransactionID: info.PreviousTransactionID,
					Role:                  models.TokenRole_Pledge,
				})
				if q.Did != "" && t.TokenType != TokenTypeSC {
					addBalanceChange(q.Did, &t, -1)
				}
			}
		}
	}

	// 6. Committed Tokens Processing (Burn)
	for _, info := range txn.CommittedTokens {
		if t, exists := existingTokens[info.TokenID]; exists {
			prevDID := t.DID
			t.LatestRole = models.TokenRole_Burn
			t.TransactionID = txnID
			t.TokenStatus = models.TokenStatus_Burnt
			tokensToUpsert = append(tokensToUpsert, t)
			chainsToInsert = append(chainsToInsert, models.TokenChain{
				TokenID:               info.TokenID,
				TransactionID:         txnID,
				PreviousTransactionID: info.PreviousTransactionID,
				Role:                  models.TokenRole_Burn,
			})
			if prevDID != "" && t.TokenType != TokenTypeSC {
				addBalanceChange(prevDID, &t, -1)
			}
		}
	}

	// 7. Unpledge Detection logic integrated into bulk...
	// (Keeping the logic of Restoring balance to quorum DIDs if their pledged token is used)
	prevTxIDs := make([]string, 0)
	collectPrevIDs := func(tokenInfos []*model.TokenInfo) {
		for _, info := range tokenInfos {
			if info.PreviousTransactionID != "" && info.PreviousTransactionID != "0" {
				prevTxIDs = append(prevTxIDs, info.PreviousTransactionID)
			}
		}
	}
	if txn.Tokens != nil {
		collectPrevIDs(txn.Tokens.RBT)
		collectPrevIDs(txn.Tokens.FT)
		collectPrevIDs(txn.Tokens.NFT)
		collectPrevIDs(txn.Tokens.SmartContract)
	}

	if len(prevTxIDs) > 0 {
		var prevTxns []models.TransactionInfo
		if err := tx.Where("transaction_id IN ?", prevTxIDs).Find(&prevTxns).Error; err == nil {
			for _, pTx := range prevTxns {
				var quorums []*model.QuorumInfo
				if err := json.Unmarshal(pTx.Quorums, &quorums); err == nil {
					for _, q := range quorums {
						for _, qToken := range q.Tokens {
							if t, exists := existingTokens[qToken.TokenID]; exists {
								if t.TokenStatus == models.TokenStatus_Pledged || t.TokenStatus == models.TokenStatus_QuorumPledged {
									t.LatestRole = models.TokenRole_Unpledge
									t.TransactionID = txnID
									t.TokenStatus = models.TokenStatus_Free
									// Don't re-add to tokensToUpsert if already there with different state? 
									// In Rubix, a token can't be both "used" and "pledged" in the same block usually.
									tokensToUpsert = append(tokensToUpsert, t)
									if q.Did != "" && t.TokenType != TokenTypeSC {
										addBalanceChange(q.Did, &t, 1)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 8. FINAL BULK COMMIT
	// Bulk Upsert Tokens
	if len(tokensToUpsert) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token_id"}},
			UpdateAll: true,
		}).CreateInBatches(tokensToUpsert, 1000).Error; err != nil {
			return err
		}
	}

	// Bulk Insert History
	if len(chainsToInsert) > 0 {
		if err := tx.CreateInBatches(chainsToInsert, 1000).Error; err != nil {
			return err
		}
	}

	// Bulk Update Balances (One update per unique DID)
	for key, delta := range balanceChanges {
		if delta == 0 {
			continue
		}
		if err := updateBalances(tx, key.DID, key.AssetType, key.TokenName, key.CreatorDID, delta); err != nil {
			return err
		}
	}

	// 9. Token Chain Array Sequence (Legacy helper, still needs to be called per-token for now)
	for _, info := range chainsToInsert {
		if err := updateTokenChainArray(tx, info.TokenID, info.ID, info.PreviousTransactionID); err != nil {
			return err
		}
	}

	return nil
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

// updateBalances handles DIDBalances (granular) table updates using aggregated values.
func updateBalances(tx *gorm.DB, did, assetType, tokenName, creatorDID string, valueToAdd float64) error {
	var balance models.DIDBalance
	err := tx.Where("did = ? AND asset_type = ? AND token_name = ? AND creator_did = ?", did, assetType, tokenName, creatorDID).First(&balance).Error
	if err == gorm.ErrRecordNotFound {
		// New balance record: Try to fetch PeerID/DIDAlgo from any existing row for this DID
		var meta models.DIDBalance
		if tx.Where("did = ? AND (peer_id IS NOT NULL AND peer_id != '')", did).
			Select("peer_id, did_algo").First(&meta).Error == nil {
			balance.PeerID = meta.PeerID
			balance.DIDAlgo = meta.DIDAlgo
		}

		balance.DID = did
		balance.AssetType = assetType
		balance.TokenName = tokenName
		balance.CreatorDID = creatorDID
		balance.Balance = valueToAdd
		return tx.Create(&balance).Error
	} else if err != nil {
		return err
	}

	// Use explicit WHERE-based update with a safety clamp to prevent negative balances
	updates := map[string]interface{}{
		"balance": gorm.Expr("GREATEST(0, balance + ?)", valueToAdd),
	}

	return tx.Model(&models.DIDBalance{}).
		Where("did = ? AND asset_type = ? AND token_name = ? AND creator_did = ?", did, assetType, tokenName, creatorDID).
		Updates(updates).Error
}

// updateTokenChainArray updates the TokenChainArray index for a token that already has a new history entry ID.
func updateTokenChainArray(tx *gorm.DB, tokenID string, historyID uint64, prevTxnID string) error {
	// 1. Load existing TokenChainArray
	var tca models.TokenChainArray
	err := tx.Where("token_id = ?", tokenID).First(&tca).Error

	var chain []uint64
	if err == nil {
		json.Unmarshal(tca.Index, &chain)
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	// 2. Determine logical position
	newID := historyID
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

	// 3. Upsert TokenChainArray
	if err == gorm.ErrRecordNotFound {
		tca = models.TokenChainArray{
			TokenID: tokenID,
			Index:   chainJSON,
		}
		return tx.Create(&tca).Error
	}

	return tx.Model(&tca).Update("index", chainJSON).Error
}

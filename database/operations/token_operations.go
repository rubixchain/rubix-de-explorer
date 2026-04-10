package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"log"
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
					if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", 1, 0); err != nil {
						return err
					}
				}
			}
			// SC Interaction (Execute) or Burn does not transfer balance for SCs.
		} else if prevOwner != token.DID {
			// Standard Logic: RBT, FT, NFT transfer ownership/balances
			if prevOwner != "" && prevOwner != "0" {
				if err := updateBalances(tx, prevOwner, tokenTypeName(token.TokenType), "", "", -1, 0); err != nil {
					return err
				}
			}
			if token.DID != "" && token.DID != "0" {
				if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", 1, 0); err != nil {
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
	// Since one DID might send/receive/pledge multiple tokens in one txn, we aggregate first.
	type balanceChange struct {
		Balance        float64
		PledgedBalance float64
	}
	type balanceKey struct {
		DID        string
		AssetType  string
		TokenName  string
		CreatorDID string
	}
	balanceChanges := make(map[balanceKey]*balanceChange)

	addBalanceChange := func(did string, token *models.Token, balanceDelta, pledgeDelta float64) {
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
		if _, ok := balanceChanges[key]; !ok {
			balanceChanges[key] = &balanceChange{}
		}
		balanceChanges[key].Balance += (val * balanceDelta)
		balanceChanges[key].PledgedBalance += (val * pledgeDelta)
	}

	tokensToUpsert := make([]models.Token, 0)
	chainsToInsert := make([]models.TokenChain, 0)


	// 4. Helper closures
	ensureToken := func(tokenID string) models.Token {
		if t, ok := existingTokens[tokenID]; ok {
			return t
		}
		typeID := inferTokenType(tokenID) // uses package-level function
		t := models.Token{
			TokenID:       tokenID,
			TokenType:     typeID,
			TransactionID: txnID,
			TokenStatus:   models.TokenStatus_Free,
			NeedsSync:     true,
		}
		if typeID == TokenTypeRBT {
			val, _ := util.GetTokenValueFromTokenID(tokenID)
			t.TokenValue = val
		} else {
			t.TokenValue = 1.0
		}
		return t
	}

	// prepareTokens closure (Phase 2: Model Prep)
	prepareTokens := func(tokenInfos []*model.TokenInfo, tokenType string) error {
		typeID := tokenTypeMap[tokenType]
		for _, info := range tokenInfos {
			existing, exists := existingTokens[info.TokenID]
			isNew := !exists
			var prevOwner string
			tokenToSave := ensureToken(info.TokenID)

			if isNew {
				switch typeID {
				case TokenTypeSC:
					tokenToSave.DID = txn.Initiator
					tokenToSave.DeployerDID = txn.Initiator
				default:
					tokenToSave.DID = txn.Owner
				}

				if info.Data != "" {
					tokenToSave.Data = info.Data
				}

				// Specific logic for new RBT/FT values if available
				switch typeID {
				case TokenTypeRBT:
					val, _ := util.GetTokenValueFromTokenID(info.TokenID)
					tokenToSave.TokenValue = val
				case TokenTypeFT:
					var burnedSum float64
					for _, ct := range txn.CommittedTokens {
						if !strings.Contains(ct.TokenID, "Qm") && !strings.Contains(ct.TokenID, "_DID") && strings.Contains(ct.TokenID, "_") {
							v, _ := util.GetTokenValueFromTokenID(ct.TokenID)
							burnedSum += v
						}
					}
					ftCount := len(txn.Tokens.FT)
					if ftCount > 0 {
						tokenToSave.TokenValue = burnedSum / float64(ftCount)
					}
				}

				// Determine the correct role: if prevTxnID exists, explorer missed
				// the genesis — this is actually a transfer, not a mint.
				if info.PreviousTransactionID != "" {
					// Missed genesis: record as transfer, flag for future sync
					tokenToSave.NeedsSync = true
					switch typeID {
					case TokenTypeSC:
						tokenToSave.LatestRole = models.TokenRole_Execute
					default:
						tokenToSave.LatestRole = models.TokenRole_Transfer
					}
				} else {
					// Actual genesis/mint (no previous transaction)
					switch typeID {
					case TokenTypeSC:
						tokenToSave.LatestRole = models.TokenRole_Deploy
					default:
						tokenToSave.LatestRole = models.TokenRole_Mint
					}
				}
			} else {
				prevOwner = existing.DID
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
					addBalanceChange(tokenToSave.DID, &tokenToSave, 1, 0)
				}
			} else if prevOwner != tokenToSave.DID {
				addBalanceChange(prevOwner, &tokenToSave, -1, 0)
				addBalanceChange(tokenToSave.DID, &tokenToSave, 1, 0)
			}
		}
		return nil
	}

	// 4. Group Processing (Phase 2: Model Prep)
	if txn.Tokens != nil {
		if err := prepareTokens(txn.Tokens.RBT, "RBT"); err != nil {
			log.Printf("Warning: Error processing RBT tokens: %v", err)
		}
		if err := prepareTokens(txn.Tokens.FT, "FT"); err != nil {
			log.Printf("Warning: Error processing FT tokens: %v", err)
		}
		if err := prepareTokens(txn.Tokens.NFT, "NFT"); err != nil {
			log.Printf("Warning: Error processing NFT tokens: %v", err)
		}
		if err := prepareTokens(txn.Tokens.SmartContract, "SC"); err != nil {
			log.Printf("Warning: Error processing SC tokens: %v", err)
		}
	}

	// 5. Quorum Pledges Processing
	for _, q := range txn.Quorums {
		for _, info := range q.Tokens {
			t := ensureToken(info.TokenID)
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
				// Deduct from Regular balance, add to Pledged balance
				addBalanceChange(q.Did, &t, -1, 1)
			}
		}
	}

	// 6. Committed Tokens Processing (Burn)
	for _, info := range txn.CommittedTokens {
		t := ensureToken(info.TokenID)
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
			// Deduct from Regular balance (Burned tokens are gone)
			addBalanceChange(prevDID, &t, -1, 0)
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
				if err := json.Unmarshal(pTx.Quorums, &quorums); err != nil {
					log.Printf("Warning: Failed to unmarshal quorums for unpledge detection in txn %s: %v", pTx.TransactionID, err)
					continue
				}
				for _, q := range quorums {
					for _, qToken := range q.Tokens {
							// Determine if we have this token in our current batch or need to fetch it
							var t models.Token
							found := false
							if cached, ok := existingTokens[qToken.TokenID]; ok {
								t = cached
								found = true
							} else {
								// Fetch from DB if not in current transaction set
								if err := tx.Where("token_id = ?", qToken.TokenID).First(&t).Error; err == nil {
									found = true
								}
							}

							if found {
								if t.TokenStatus == models.TokenStatus_Pledged || t.TokenStatus == models.TokenStatus_QuorumPledged {
									t.LatestRole = models.TokenRole_Unpledge
									t.TransactionID = txnID
									t.TokenStatus = models.TokenStatus_Unpledged

									// Add to bulk upsert list
									tokensToUpsert = append(tokensToUpsert, t)

									if q.Did != "" && t.TokenType != TokenTypeSC {
										// Restore to Regular balance, deduct from Pledged balance
										addBalanceChange(q.Did, &t, 1, -1)
									}
								}
							}
						}
					}
				}
			}
		}

	// 8. FINAL BULK COMMIT

	// 8a. Deduplicate tokensToUpsert: a token can appear multiple times because
	// Unpledge Detection (step 7) may re-add a token that was already processed
	// in Quorum Pledges (step 5). We keep the LAST entry per token_id because
	// processing order is: Primary Tokens → Quorum Pledges → Burns → Unpledges,
	// so the last entry reflects the token's final state after this transaction.
	deduped := make(map[string]models.Token, len(tokensToUpsert))
	dedupOrder := make([]string, 0, len(tokensToUpsert))
	for _, t := range tokensToUpsert {
		if _, seen := deduped[t.TokenID]; !seen {
			dedupOrder = append(dedupOrder, t.TokenID)
		}
		deduped[t.TokenID] = t
	}
	finalTokens := make([]models.Token, 0, len(deduped))
	for _, id := range dedupOrder {
		finalTokens = append(finalTokens, deduped[id])
	}

	// Bulk Upsert Tokens (deduplicated)
	if len(finalTokens) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token_id"}},
			UpdateAll: true,
		}).CreateInBatches(finalTokens, 1000).Error; err != nil {
			return err
		}
	}

	// 8b. Insert TokenChain records ONE BY ONE to capture the real auto-increment ID,
	// then immediately update the TokenChainArray with that ID.
	for _, chain := range chainsToInsert {
		// Insert and get the real ID back
		if err := tx.Create(&chain).Error; err != nil {
			log.Printf("Warning: Failed to insert TokenChain for token %s txn %s: %v", chain.TokenID, chain.TransactionID, err)
			continue
		}

		// Now chain.ID is the real DB-assigned auto-increment value
		if err := updateTokenChainArray(tx, chain.TokenID, chain.ID, chain.PreviousTransactionID); err != nil {
			return err
		}
	}

	// 8c. Bulk Update Balances (One update per unique DID/Asset)
	for key, deltas := range balanceChanges {
		if deltas.Balance == 0 && deltas.PledgedBalance == 0 {
			continue
		}
		if err := updateBalances(tx, key.DID, key.AssetType, key.TokenName, key.CreatorDID, deltas.Balance, deltas.PledgedBalance); err != nil {
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
func updateBalances(tx *gorm.DB, did, assetType, tokenName, creatorDID string, balanceDelta, pledgedDelta float64) error {
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
		balance.Balance = balanceDelta
		balance.PledgedBalance = pledgedDelta
		return tx.Create(&balance).Error
	} else if err != nil {
		return err
	}

	// Use explicit WHERE-based update with a safety clamp to prevent negative balances
	updates := map[string]interface{}{
		"balance":         gorm.Expr("GREATEST(0, balance + ?)", balanceDelta),
		"pledged_balance": gorm.Expr("GREATEST(0, pledged_balance + ?)", pledgedDelta),
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

// SyncPledgedBalances is a one-time migration helper to populate the pledged_balance column
// for existing DIDs by scanning the Tokens table.
func SyncPledgedBalances(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Reset all pledged balances to 0
		if err := tx.Model(&models.DIDBalance{}).Where("asset_type = ?", "RBT").Update("pledged_balance", 0).Error; err != nil {
			return err
		}

		// 2. Aggregate RBT pledged tokens per DID
		type pledgedResult struct {
			DID          string  `gorm:"column:did"`
			TotalPledged float64 `gorm:"column:total_pledged"`
		}
		var results []pledgedResult
		err := tx.Table("Tokens").
			Select("did, SUM(token_value) as total_pledged").
			Where("token_type = ? AND token_status IN (?, ?)", TokenTypeRBT, models.TokenStatus_Pledged, models.TokenStatus_QuorumPledged).
			Group("did").
			Scan(&results).Error
		if err != nil {
			return err
		}

		// 3. Update or Create entries in DIDBalances table
		for _, res := range results {
			if res.TotalPledged <= 0 {
				continue
			}

			var balance models.DIDBalance
			err := tx.Where("did = ? AND asset_type = ? AND token_name = ? AND creator_did = ?", res.DID, "RBT", "", "").First(&balance).Error
			if err == gorm.ErrRecordNotFound {
				// Create new record for DIDs that had no balance row
				balance.DID = res.DID
				balance.AssetType = "RBT"
				balance.PledgedBalance = res.TotalPledged
				// Try to fetch peer_id/algo if possible
				var meta models.DIDBalance
				if tx.Where("did = ? AND (peer_id IS NOT NULL AND peer_id != '')", res.DID).
					Select("peer_id, did_algo").First(&meta).Error == nil {
					balance.PeerID = meta.PeerID
					balance.DIDAlgo = meta.DIDAlgo
				}
				if err := tx.Create(&balance).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				// Update existing
				if err := tx.Model(&balance).Update("pledged_balance", res.TotalPledged).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
// EnsureTokenSkeletons ensures every token mentioned in a transaction (even failed ones)
// exists in the Tokens table as at least a skeleton record.
func EnsureTokenSkeletons(txn *model.TransactionInfo, txnID string) error {
	tokenIDs := make(map[string]struct{})
	collect := func(infos []*model.TokenInfo) {
		for _, info := range infos {
			if info.TokenID != "" {
				tokenIDs[info.TokenID] = struct{}{}
			}
		}
	}

	if txn.Tokens != nil {
		collect(txn.Tokens.RBT)
		collect(txn.Tokens.FT)
		collect(txn.Tokens.NFT)
		collect(txn.Tokens.SmartContract)
	}
	for _, q := range txn.Quorums {
		collect(q.Tokens)
	}
	collect(txn.CommittedTokens)

	if len(tokenIDs) == 0 {
		return nil
	}

	return database.WriteDB.Transaction(func(tx *gorm.DB) error {
		for id := range tokenIDs {
			// Using Upsert with DoNothing to ensure we only create if it doesn't already exist.
			// This avoids overwriting live state with skeleton data.
			skeleton := models.Token{
				TokenID:       id,
				TokenType:     inferTokenType(id),
				TransactionID: txnID,
				TokenStatus:   models.TokenStatus_Free,
				NeedsSync:     true,
			}
			// RBT Value extraction logic
			if skeleton.TokenType == TokenTypeRBT {
				val, _ := util.GetTokenValueFromTokenID(id)
				skeleton.TokenValue = val
			} else {
				skeleton.TokenValue = 1.0
			}

			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&skeleton).Error; err != nil {
				log.Printf("Warning: Failed to create token skeleton for %s: %v", id, err)
			}
		}
		return nil
	})
}

// inferTokenType determines the asset type based on the ID format.
func inferTokenType(id string) int16 {
	if util.IsValidRBT(id) {
		return TokenTypeRBT
	}
	if util.IsValidFT(id) {
		return TokenTypeFT
	}
	if util.IsValidNFT(id) {
		return TokenTypeNFT
	}
	if util.IsValidSC(id) {
		return TokenTypeSC
	}
	// Fallback for cases where regex might be too strict
	if strings.HasPrefix(id, "Qm") {
		return TokenTypeNFT
	}
	if strings.Contains(id, "_bafy") {
		return TokenTypeFT
	}
	return TokenTypeRBT // Final default
}

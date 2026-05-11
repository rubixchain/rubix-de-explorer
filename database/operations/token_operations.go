package operations

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"log"
	"runtime"
	"sort"
	"strings"
	"sync"

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
				val := token.TokenValue
				if token.TokenType != TokenTypeRBT && val == 0 {
					val = 1.0 // Fallback
				}

				if token.DID != "" && token.DID != "0" {
					if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", val, 0); err != nil {
						return err
					}
				}
			}
		// SC Interaction (Execute) or Burn does not transfer balance for SCs.
		} else {
			// Standard Logic: RBT, FT, NFT transfer ownership/balances
			// NOTE: In this single-token update path, we assume the 'token' struct 
			// has already been updated with its NEW value by the caller.
			val := token.TokenValue
			if token.TokenType != TokenTypeRBT && val == 0 {
				val = 1.0 // Fallback
			}

			if prevOwner != "" && prevOwner != "0" && prevOwner != token.DID {
				if err := updateBalances(tx, prevOwner, tokenTypeName(token.TokenType), "", "", -val, 0); err != nil {
					return err
				}
			}
			if token.DID != "" && token.DID != "0" && prevOwner != token.DID {
				if err := updateBalances(tx, token.DID, tokenTypeName(token.TokenType), "", "", val, 0); err != nil {
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

	addBalanceChange := func(did string, typeID int16, tokenID string, value float64, delta float64) {
		if did == "" || did == "0" {
			return
		}
		typeName := tokenTypeName(typeID)
		
		key := balanceKey{DID: did, AssetType: typeName}
		if typeID == TokenTypeFT {
			parts := strings.Split(tokenID, "_")
			if len(parts) > 0 {
				key.TokenName = parts[0]
				// Intelligently find the DID part (it's the one starting with 'bafy')
				for _, p := range parts {
					if strings.HasPrefix(p, "bafy") && len(p) == 59 {
						key.CreatorDID = p
						break
					}
				}
			}
		}
		if _, ok := balanceChanges[key]; !ok {
			balanceChanges[key] = &balanceChange{}
		}
		// delta is now the actual value to add/subtract (no internal multiplication by token.TokenValue)
		balanceChanges[key].Balance = util.RoundToMaxDecimals(balanceChanges[key].Balance + delta)
	}

	tokensToUpsert := make([]models.Token, 0)
	chainsToInsert := make([]models.TokenChain, 0)

	// Check if this transaction has an SC deployment
	hasSCDeploy := false
	if txn.Tokens != nil && len(txn.Tokens.SmartContract) > 0 {
		for _, sc := range txn.Tokens.SmartContract {
			if sc.PreviousTransactionID == "" {
				hasSCDeploy = true
				break
			}
		}
	}

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
					NeedsSync:     true,
				}
				if typeID == TokenTypeSC || typeID == TokenTypeNFT {
					// 1. Preserve existing DeployerDID if we have it
					if exists {
						tokenToSave.DeployerDID = existing.DeployerDID
					}
					// 2. If this is the mint/deploy, set the DeployerDID
					if isNew {
						tokenToSave.DeployerDID = txn.Initiator
					}

					// For deployments, if Owner is empty, the Initiator is the owner
					if txn.Owner != "" {
						tokenToSave.DID = txn.Owner
					} else if isNew {
						tokenToSave.DID = txn.Initiator
					} else if exists {
						tokenToSave.DID = existing.DID
					}
				} else if typeID == TokenTypeFT {
					tokenToSave.DID = txn.Owner
					// Extract Creator from FT TokenID
					parts := strings.Split(info.TokenID, "_")
					for _, p := range parts {
						if strings.HasPrefix(p, "bafy") && len(p) == 59 {
							tokenToSave.DeployerDID = p
							break
						}
					}
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
					// Use explicit TokenValue if provided (common for NFT/FT), otherwise fallback to burnedSum
					if info.TokenValue > 0 {
						tokenToSave.TokenValue = info.TokenValue
					} else if typeID == TokenTypeFT {
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
				// Determine the correct role: if prevTxnID exists, explorer missed
				// the genesis — this is actually a transfer, not a mint.
				if info.PreviousTransactionID != "" {
					// Missed genesis: record as transfer, flag for future sync
					tokenToSave.NeedsSync = true
					switch typeID {
					case TokenTypeSC:
						tokenToSave.LatestRole = models.TokenRole_Execute
					case TokenTypeNFT:
						// NFT execution without ownership change vs transfer
						if tokenToSave.DID == txn.Initiator {
							tokenToSave.LatestRole = models.TokenRole_Execute
						} else {
							tokenToSave.LatestRole = models.TokenRole_Transfer
						}
					default:
						tokenToSave.LatestRole = models.TokenRole_Transfer
					}
				} else {
					// Actual genesis/mint (no previous transaction)
					switch typeID {
					case TokenTypeSC, TokenTypeNFT:
						tokenToSave.LatestRole = models.TokenRole_Deploy
					default:
						tokenToSave.LatestRole = models.TokenRole_Mint
					}
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
				if info.Data != "" {
					tokenToSave.Data = info.Data
				}
				
				inferredRole := models.TokenRole_Transfer
				if typeID == TokenTypeSC {
					inferredRole = models.TokenRole_Execute
				} else if typeID == TokenTypeNFT {
					if targetOwner == prevOwner {
						inferredRole = models.TokenRole_Execute
					} else {
						inferredRole = models.TokenRole_Transfer
					}
				}
			// Assign TokenStatus based on the role (following Core)
			if tokenToSave.LatestRole == models.TokenRole_Deploy {
				tokenToSave.TokenStatus = models.TokenStatus_Deployed
			} else if tokenToSave.LatestRole == models.TokenRole_Execute {
				tokenToSave.TokenStatus = models.TokenStatus_Executed
			} else {
				tokenToSave.TokenStatus = models.TokenStatus_Free
			}

			tokensToUpsert = append(tokensToUpsert, tokenToSave)
			chainsToInsert = append(chainsToInsert, models.TokenChain{
				TokenID:               info.TokenID,
				TransactionID:         txnID,
				PreviousTransactionID: info.PreviousTransactionID,
				Role:                  tokenToSave.LatestRole,
			})

			// Aggregate Balance Changes with deltas
			oldVal := existing.TokenValue
			newVal := tokenToSave.TokenValue
			if typeID != TokenTypeRBT {
				if oldVal == 0 { oldVal = 1.0 }
				if newVal == 0 { newVal = 1.0 }
			}

			if typeID == TokenTypeSC {
				if isNew && tokenToSave.DID != "" && tokenToSave.DID != "0" {
					addBalanceChange(tokenToSave.DID, typeID, info.TokenID, 0, newVal)
				}
			} else {
				// Standard Logic: Subtract OLD, Add NEW
				addBalanceChange(prevOwner, typeID, info.TokenID, 0, -oldVal)
				addBalanceChange(tokenToSave.DID, typeID, info.TokenID, 0, newVal)
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
				// Check if the token is already pledged (prevents double-counting pledge balance)
				alreadyPledged := t.TokenStatus == models.TokenStatus_Pledged

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
				if q.Did != "" && t.TokenType != TokenTypeSC && !alreadyPledged {
					// Only adjust balance when transitioning Free → Pledged (not re-pledge)
					addBalanceChange(q.Did, &t, -1, 1)
				}
			}
		}
	}

	// 6. Committed Tokens Processing (Commit or Burn)
	for _, info := range txn.CommittedTokens {
		if t, exists := existingTokens[info.TokenID]; exists {
			prevDID := t.DID
			
			if hasSCDeploy {
				t.LatestRole = models.TokenRole_Commit
				t.TokenStatus = models.TokenStatus_Committed
			} else {
				t.LatestRole = models.TokenRole_Burn
				t.TokenStatus = models.TokenStatus_Burnt
			}
			
			t.TransactionID = txnID
			
			tokensToUpsert = append(tokensToUpsert, t)
			chainsToInsert = append(chainsToInsert, models.TokenChain{
				TokenID:               info.TokenID,
				TransactionID:         txnID,
				PreviousTransactionID: info.PreviousTransactionID,
				Role:                  t.LatestRole,
			})
			if prevDID != "" && t.TokenType != TokenTypeSC {
				// Deduct from Regular balance (Burned/Committed tokens are no longer Free)
				addBalanceChange(prevDID, &t, -1, 0)
			}
		}
	}

	// 7. FINAL BULK COMMIT
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
		if err := tx.CreateInBatches(&chainsToInsert, 1000).Error; err != nil {
			return err
		}
	}

	// Bulk Update Balances (One update per unique DID/Asset)
	// Sort keys to prevent Postgres deadlocks caused by concurrent non-deterministic locking order
	var balanceKeys []balanceKey
	for key := range balanceChanges {
		balanceKeys = append(balanceKeys, key)
	}
	sort.Slice(balanceKeys, func(i, j int) bool {
		if balanceKeys[i].DID != balanceKeys[j].DID {
			return balanceKeys[i].DID < balanceKeys[j].DID
		}
		if balanceKeys[i].AssetType != balanceKeys[j].AssetType {
			return balanceKeys[i].AssetType < balanceKeys[j].AssetType
		}
		if balanceKeys[i].TokenName != balanceKeys[j].TokenName {
			return balanceKeys[i].TokenName < balanceKeys[j].TokenName
		}
		return balanceKeys[i].CreatorDID < balanceKeys[j].CreatorDID
	})

	for _, key := range balanceKeys {
		deltas := balanceChanges[key]
		if deltas.Balance == 0 && deltas.PledgedBalance == 0 {
			continue
		}
		if err := updateBalances(tx, key.DID, key.AssetType, key.TokenName, key.CreatorDID, deltas.Balance, deltas.PledgedBalance); err != nil {
			return err
		}
	}

	// 8. Token Chain Array Sequence (Hardened against Race Conditions & OOM)
	if len(chainsToInsert) > 0 {
		// A. Group incoming chains by TokenID
		tcaTokenIDsMap := make(map[string][]models.TokenChain)
		uniqueTcaIDs := []string{}
		for _, info := range chainsToInsert {
			if len(tcaTokenIDsMap[info.TokenID]) == 0 {
				uniqueTcaIDs = append(uniqueTcaIDs, info.TokenID)
			}
			tcaTokenIDsMap[info.TokenID] = append(tcaTokenIDsMap[info.TokenID], info)
		}

		// B. Process in batches to prevent Memory Overflow (OOM) on large transactions
		const assetBatchSize = 5000
		for bStart := 0; bStart < len(uniqueTcaIDs); bStart += assetBatchSize {
			bEnd := bStart + assetBatchSize
			if bEnd > len(uniqueTcaIDs) {
				bEnd = len(uniqueTcaIDs)
			}
			batchTokenIDs := uniqueTcaIDs[bStart:bEnd]

			// C. Bulk fetch existing TokenChainArrays for this batch
			existingTCA := make(map[string][]uint64)
			var tcaBatch []models.TokenChainArray
			if err := tx.Where("token_id IN ?", batchTokenIDs).Find(&tcaBatch).Error; err != nil {
				return err
			}
			for _, tca := range tcaBatch {
				var chain []uint64
				if len(tca.Index) > 0 {
					_ = json.Unmarshal(tca.Index, &chain)
				}
				existingTCA[tca.TokenID] = chain
			}

			// D. Bulk fetch Parent TokenChain IDs for this batch
			parentTxnIDMap := make(map[string]map[string]uint64)
			prevTxnIDs := make(map[string]struct{})
			for _, tid := range batchTokenIDs {
				for _, info := range tcaTokenIDsMap[tid] {
					if info.PreviousTransactionID != "" {
						prevTxnIDs[info.PreviousTransactionID] = struct{}{}
					}
				}
			}

			if len(prevTxnIDs) > 0 {
				uniquePrevTxnIDs := make([]string, 0, len(prevTxnIDs))
				for pid := range prevTxnIDs {
					uniquePrevTxnIDs = append(uniquePrevTxnIDs, pid)
				}
				var parentChains []models.TokenChain
				if err := tx.Where("transaction_id IN ?", uniquePrevTxnIDs).Select("id, token_id, transaction_id").Find(&parentChains).Error; err != nil {
					return err
				}
				for _, ptc := range parentChains {
					if parentTxnIDMap[ptc.TokenID] == nil {
						parentTxnIDMap[ptc.TokenID] = make(map[string]uint64)
					}
					parentTxnIDMap[ptc.TokenID][ptc.TransactionID] = ptc.ID
				}
			}
			// E. Parallel In-Memory Mutation (RAM-Adaptive: prevents explosion under high load)
			memPercent := util.GetSystemMemoryUsage()
			numWorkers := runtime.NumCPU()

			// Don't stress RAM above 70%. If high, downshift to sequential or half-parallel.
			if memPercent > 70 {
				numWorkers = 1
				log.Printf("[Asset Processor] High RAM detected (%.2f%%). Processing batch sequentially.", memPercent)
			} else if memPercent > 50 {
				numWorkers = numWorkers / 2
				if numWorkers < 1 {
					numWorkers = 1
				}
				log.Printf("[Asset Processor] Moderate RAM detected (%.2f%%). Reducing parallelism to %d workers.", memPercent, numWorkers)
			}

			if numWorkers < 1 {
				numWorkers = 1
			}

			type tokenWork struct {
				TokenID string
				Chains  []models.TokenChain
			}
			type tokenResult struct {
				TokenID string
				Chain   []uint64
			}

			workChan := make(chan tokenWork, len(batchTokenIDs))
			resultChan := make(chan tokenResult, len(batchTokenIDs))

			// Load work into the channel
			for _, tid := range batchTokenIDs {
				workChan <- tokenWork{TokenID: tid, Chains: tcaTokenIDsMap[tid]}
			}
			close(workChan)

			var wg sync.WaitGroup
			wg.Add(numWorkers)

			// Start fixed number of workers
			for i := 0; i < numWorkers; i++ {
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[Asset Processor] recovered from panic: %v", r)
						}
					}()

					for work := range workChan {
						// Create a local copy of the chain to ensure thread-safety
						newChain := append([]uint64{}, existingTCA[work.TokenID]...)
						tmap := parentTxnIDMap[work.TokenID]

						for _, info := range work.Chains {
							newID := info.ID
							inserted := false
							if info.PreviousTransactionID != "" && tmap != nil {
								if parentID, ok := tmap[info.PreviousTransactionID]; ok {
									for j, id := range newChain {
										if id == parentID {
											// Insert newID after parentID
											newChain = append(newChain[:j+1], append([]uint64{newID}, newChain[j+1:]...)...)
											inserted = true
											break
										}
									}
								}
							}
							if !inserted {
								if info.PreviousTransactionID == "" {
									if len(newChain) == 0 {
										newChain = []uint64{newID}
									} else {
										newChain = append([]uint64{newID}, newChain...)
									}
								} else {
									newChain = append(newChain, newID)
								}
							}
						}
						resultChan <- tokenResult{TokenID: work.TokenID, Chain: newChain}
					}
				}()
			}

			// F. Collect results into a SEPARATE map to avoid R/W race with active workers
			finalResults := make(map[string][]uint64)
			resultsDone := make(chan bool)
			go func() {
				for res := range resultChan {
					finalResults[res.TokenID] = res.Chain
				}
				resultsDone <- true
			}()

			// Wait for all workers to finish reading from existingTCA
			wg.Wait()
			close(resultChan)
			<-resultsDone // Ensure all results are collected

			// G. Bulk Upsert results for this batch
			var tcaUpserts []models.TokenChainArray
			for _, tid := range batchTokenIDs {
				chain := finalResults[tid]
				b, _ := json.Marshal(chain)
				tcaUpserts = append(tcaUpserts, models.TokenChainArray{
					TokenID: tid,
					Index:   b,
				})
			}

			if len(tcaUpserts) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "token_id"}},
					UpdateAll: true,
				}).CreateInBatches(tcaUpserts, 1000).Error; err != nil {
					return err
				}
			}
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

// updateBalances handles DIDBalances (granular) table updates using atomic UPSERTs.
func updateBalances(tx *gorm.DB, did, assetType, tokenName, creatorDID string, balanceDelta, pledgedDelta float64) error {
	// Try to fetch PeerID/DIDAlgo from any existing row for this DID if this is a new row.
	// This is a best-effort metadata sync for the DIDBalances table.
	var peerID string
	var didAlgo int64
	var meta models.DIDBalance
	if tx.Where("did = ? AND (peer_id IS NOT NULL AND peer_id != '')", did).
		Select("peer_id, did_algo").First(&meta).Error == nil {
		peerID = meta.PeerID
		didAlgo = meta.DIDAlgo
	}

	// Atomic UPSERT logic:
	// 1. Try to INSERT the new balance record.
	// 2. IF a conflict occurs (DID+AssetType+TokenName+CreatorDID already exists),
	//    THEN update the existing balance using GREATEST(0, balance + delta) to prevent negative values.
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "did"},
			{Name: "asset_type"},
			{Name: "token_name"},
			{Name: "creator_did"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"balance":         gorm.Expr("ROUND(GREATEST(0, \"DIDBalances\".balance + ?), 3)", balanceDelta),
			"pledged_balance": gorm.Expr("ROUND(GREATEST(0, \"DIDBalances\".pledged_balance + ?), 3)", pledgedDelta),
			"peer_id":         gorm.Expr("COALESCE(\"DIDBalances\".peer_id, EXCLUDED.peer_id)"),
			"did_algo":        gorm.Expr("COALESCE(\"DIDBalances\".did_algo, EXCLUDED.did_algo)"),
		}),
	}).Create(&models.DIDBalance{
		DID:            did,
		AssetType:      assetType,
		TokenName:      tokenName,
		CreatorDID:     creatorDID,
		Balance:        balanceDelta,
		PledgedBalance: pledgedDelta,
		PeerID:         peerID,
		DIDAlgo:        didAlgo,
	}).Error
}



// SyncAllBalances is a startup migration helper to populate both the balance and pledged_balance columns
// for existing DIDs by dynamically calculating them from the Tokens table.
func SyncAllBalances(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Reset all balances to 0
		if err := tx.Model(&models.DIDBalance{}).Where("asset_type = ?", "RBT").Updates(map[string]interface{}{
			"balance": 0, 
			"pledged_balance": 0,
		}).Error; err != nil {
			return err
		}

		// 2. Aggregate RBT tokens per DID
		type balanceResult struct {
			DID          string  `gorm:"column:did"`
			TotalFree    float64 `gorm:"column:total_free"`
			TotalPledged float64 `gorm:"column:total_pledged"`
		}
		var results []balanceResult
		err := tx.Raw(`
			SELECT did, 
			       SUM(CASE WHEN token_status IN (?, ?) THEN token_value ELSE 0 END) as total_free,
			       SUM(CASE WHEN token_status IN (?, ?) THEN token_value ELSE 0 END) as total_pledged
			FROM "Tokens"
			WHERE token_type = ? AND token_status IN (?, ?, ?, ?)
			GROUP BY did
		`, 
			models.TokenStatus_Free, models.TokenStatus_Locked,
			models.TokenStatus_Pledged, models.TokenStatus_QuorumPledged,
			TokenTypeRBT, 
			models.TokenStatus_Free, models.TokenStatus_Locked, 
			models.TokenStatus_Pledged, models.TokenStatus_QuorumPledged,
		).Scan(&results).Error
		if err != nil {
			return err
		}

		// 3. Update or Create entries in DIDBalances table
		for _, res := range results {
			if res.TotalFree <= 0 && res.TotalPledged <= 0 {
				continue
			}

			var balance models.DIDBalance
			err := tx.Where("did = ? AND asset_type = ? AND token_name = ? AND creator_did = ?", res.DID, "RBT", "", "").First(&balance).Error
			if err == gorm.ErrRecordNotFound {
				// Create new record for DIDs that had no balance row
				balance.DID = res.DID
				balance.AssetType = "RBT"
				balance.Balance = res.TotalFree
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
				if err := tx.Model(&balance).Updates(map[string]interface{}{
					"balance": res.TotalFree,
					"pledged_balance": res.TotalPledged,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// ProcessUnpledgeTokens handles the unpledge event from quorum nodes.
// It transitions Pledged/QuorumPledged tokens to Free, inserts TokenChain
// entries, updates the TokenChainArray sequence, and adjusts DIDBalances.
func ProcessUnpledgeTokens(event *model.UnpledgeEvent) error {
	if len(event.UnpledgeInfo) == 0 {
		return nil
	}

	unpledgeTxnID := event.UnpledgeTransactionID
	pledgeTxnID := event.PledgeTransactionID

	return database.WriteDB.Transaction(func(tx *gorm.DB) error {
		// Collect all incoming token IDs
		tokenIDs := make([]string, 0, len(event.UnpledgeInfo))
		for _, info := range event.UnpledgeInfo {
			tokenIDs = append(tokenIDs, info.TokenID)
		}

		// 1. Fetch tokens that are currently pledged
		var pledgedTokens []models.Token
		if err := tx.Where("token_id IN ? AND token_status IN (?, ?)",
			tokenIDs,
			models.TokenStatus_Pledged,
			models.TokenStatus_QuorumPledged,
		).Find(&pledgedTokens).Error; err != nil {
			return err
		}

		if len(pledgedTokens) == 0 {
			log.Printf("[Unpledge] No pledged tokens found for %d incoming IDs, skipping", len(tokenIDs))
			return nil
		}

		log.Printf("[Unpledge] Processing %d pledged tokens (out of %d received)", len(pledgedTokens), len(tokenIDs))

		// 2. Aggregate balance deltas per DID
		type balanceDelta struct {
			FreeDelta   float64
			PledgeDelta float64
		}
		didDeltas := make(map[string]*balanceDelta)

		tokenIDsToUpdate := make([]string, 0, len(pledgedTokens))
		for _, t := range pledgedTokens {
			tokenIDsToUpdate = append(tokenIDsToUpdate, t.TokenID)

			if t.DID == "" || t.DID == "0" {
				continue
			}

			val := t.TokenValue
			if t.TokenType != TokenTypeRBT {
				val = 1.0
			}

			if _, ok := didDeltas[t.DID]; !ok {
				didDeltas[t.DID] = &balanceDelta{}
			}
			didDeltas[t.DID].FreeDelta = util.RoundToMaxDecimals(didDeltas[t.DID].FreeDelta + val)
			didDeltas[t.DID].PledgeDelta = util.RoundToMaxDecimals(didDeltas[t.DID].PledgeDelta - val)
		}

		// 3. Bulk update tokens: Pledged → Free, role → Unpledge
		if err := tx.Model(&models.Token{}).
			Where("token_id IN ?", tokenIDsToUpdate).
			Updates(map[string]interface{}{
				"token_status":   models.TokenStatus_Free,
				"latest_role":    models.TokenRole_Unpledge,
				"transaction_id": unpledgeTxnID,
			}).Error; err != nil {
			return err
		}

		// 4. Insert TokenChain entries for each unpledged token
		// All tokens share the same pledgeTransactionID as their previous transaction
		chainsToInsert := make([]models.TokenChain, 0, len(tokenIDsToUpdate))
		for _, tid := range tokenIDsToUpdate {
			chainsToInsert = append(chainsToInsert, models.TokenChain{
				TokenID:               tid,
				TransactionID:         unpledgeTxnID,
				PreviousTransactionID: pledgeTxnID,
				Role:                  models.TokenRole_Unpledge,
			})
		}

		if err := tx.CreateInBatches(&chainsToInsert, 1000).Error; err != nil {
			return err
		}

		// 5. Update TokenChainArray — single query since all tokens share the same parent txnID
		// 5a. Fetch parent TokenChain IDs for the shared pledgeTransactionID
		parentChainIDMap := make(map[string]uint64) // tokenID -> parent chainID
		if pledgeTxnID != "" {
			var parentChains []models.TokenChain
			if err := tx.Where("transaction_id = ? AND token_id IN ?", pledgeTxnID, tokenIDsToUpdate).
				Select("id, token_id").
				Find(&parentChains).Error; err != nil {
				return err
			}
			for _, ptc := range parentChains {
				parentChainIDMap[ptc.TokenID] = ptc.ID
			}
		}

		// 5b. Bulk fetch existing TokenChainArrays
		existingTCA := make(map[string][]uint64)
		var tcaBatch []models.TokenChainArray
		if err := tx.Where("token_id IN ?", tokenIDsToUpdate).Find(&tcaBatch).Error; err != nil {
			return err
		}
		for _, tca := range tcaBatch {
			var chain []uint64
			if len(tca.Index) > 0 {
				_ = json.Unmarshal(tca.Index, &chain)
			}
			existingTCA[tca.TokenID] = chain
		}

		// 5c. Build new chain sequences (insert after parent position)
		var tcaUpserts []models.TokenChainArray
		for i, tid := range tokenIDsToUpdate {
			newID := chainsToInsert[i].ID
			chain := append([]uint64{}, existingTCA[tid]...)

			inserted := false
			if parentID, ok := parentChainIDMap[tid]; ok {
				for j, id := range chain {
					if id == parentID {
						// Insert newID right after parentID
						chain = append(chain[:j+1], append([]uint64{newID}, chain[j+1:]...)...)
						inserted = true
						break
					}
				}
			}
			if !inserted {
				// Append at the end (fallback: parent not found)
				chain = append(chain, newID)
			}

			b, _ := json.Marshal(chain)
			tcaUpserts = append(tcaUpserts, models.TokenChainArray{
				TokenID: tid,
				Index:   b,
			})
		}

		if len(tcaUpserts) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "token_id"}},
				UpdateAll: true,
			}).CreateInBatches(tcaUpserts, 1000).Error; err != nil {
				return err
			}
		}

		// 6. Apply balance adjustments (increment free, decrement pledged)
		// Sort keys to prevent Postgres deadlocks from concurrent non-deterministic locking order
		sortedDIDs := make([]string, 0, len(didDeltas))
		for did := range didDeltas {
			sortedDIDs = append(sortedDIDs, did)
		}
		sort.Strings(sortedDIDs)

		for _, did := range sortedDIDs {
			delta := didDeltas[did]
			if delta.FreeDelta == 0 && delta.PledgeDelta == 0 {
				continue
			}
			if err := updateBalances(tx, did, "RBT", "", "", delta.FreeDelta, delta.PledgeDelta); err != nil {
				return err
			}
		}

		log.Printf("[Unpledge] Successfully unpledged %d tokens across %d DIDs (txn=%s, pledgeTxn=%s)",
			len(tokenIDsToUpdate), len(didDeltas), unpledgeTxnID, pledgeTxnID)
		return nil
	})
}



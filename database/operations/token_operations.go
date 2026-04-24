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
	if db == nil {
		return database.WriteDB.Transaction(func(tx *gorm.DB) error {
			return ProcessTransactionAssets(tx, txn, txnID)
		})
	}

	tx := db

	if txn.Tokens == nil && txn.Quorums == nil && txn.CommittedTokens == nil {
		return nil
	}

	// 1. Collect all involved TokenIDs
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

	// 3. Balance aggregation
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

	// Track token IDs that already had a balance change applied,
	// to prevent double-debiting when a token appears in both Quorums and CommittedTokens
	balanceChangedTokens := make(map[string]struct{})

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
				if info.PreviousTransactionID != "" {
					tokenToSave.NeedsSync = true
					switch typeID {
					case TokenTypeSC:
						tokenToSave.LatestRole = models.TokenRole_Execute
					case TokenTypeNFT:
						if tokenToSave.DID == txn.Initiator {
							tokenToSave.LatestRole = models.TokenRole_Execute
						} else {
							tokenToSave.LatestRole = models.TokenRole_Transfer
						}
					default:
						tokenToSave.LatestRole = models.TokenRole_Transfer
					}
				} else {
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
				tokenToSave.LatestRole = inferredRole
			}

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

	// 4. Group Processing
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
					addBalanceChange(q.Did, &t, -1, 1)
					balanceChangedTokens[info.TokenID] = struct{}{}
				}
			}
		}
	}

	// 6. Committed Tokens Processing (Commit or Burn)
	for _, info := range txn.CommittedTokens {
		t, exists := existingTokens[info.TokenID]
		if !exists {
			// Token not yet seen by explorer — flag for sync, do not silently skip
			log.Printf("[ProcessTransactionAssets] WARNING: committed token %s not found in DB for txn %s — balance debit skipped, flagging NeedsSync", info.TokenID, txnID)
			tokensToUpsert = append(tokensToUpsert, models.Token{
				TokenID:       info.TokenID,
				TokenType:     TokenTypeRBT,
				TransactionID: txnID,
				LatestRole:    models.TokenRole_Burn,
				TokenStatus:   models.TokenStatus_Burnt,
				NeedsSync:     true,
			})
			continue
		}

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

		// Only debit balance if not already debited during Quorum Pledge processing
		if _, alreadyChanged := balanceChangedTokens[info.TokenID]; !alreadyChanged {
			if prevDID != "" && t.TokenType != TokenTypeSC {
				addBalanceChange(prevDID, &t, -1, 0)
			}
		}
	}

	// 7. FINAL BULK COMMIT
	if len(tokensToUpsert) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token_id"}},
			UpdateAll: true,
		}).CreateInBatches(tokensToUpsert, 1000).Error; err != nil {
			return err
		}
	}

	if len(chainsToInsert) > 0 {
		if err := tx.CreateInBatches(&chainsToInsert, 1000).Error; err != nil {
			return err
		}
	}

	// Sort keys to prevent Postgres deadlocks
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

	// 8. Token Chain Array Sequence
	if len(chainsToInsert) > 0 {
		tcaTokenIDsMap := make(map[string][]models.TokenChain)
		uniqueTcaIDs := []string{}
		for _, info := range chainsToInsert {
			if len(tcaTokenIDsMap[info.TokenID]) == 0 {
				uniqueTcaIDs = append(uniqueTcaIDs, info.TokenID)
			}
			tcaTokenIDsMap[info.TokenID] = append(tcaTokenIDsMap[info.TokenID], info)
		}

		const assetBatchSize = 5000
		for bStart := 0; bStart < len(uniqueTcaIDs); bStart += assetBatchSize {
			bEnd := bStart + assetBatchSize
			if bEnd > len(uniqueTcaIDs) {
				bEnd = len(uniqueTcaIDs)
			}
			batchTokenIDs := uniqueTcaIDs[bStart:bEnd]

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

			numWorkers := runtime.NumCPU()
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

			for _, tid := range batchTokenIDs {
				workChan <- tokenWork{TokenID: tid, Chains: tcaTokenIDsMap[tid]}
			}
			close(workChan)

			var wg sync.WaitGroup
			wg.Add(numWorkers)

			for i := 0; i < numWorkers; i++ {
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[Asset Processor] recovered from panic: %v", r)
						}
					}()

					for work := range workChan {
						chain := append([]uint64{}, existingTCA[work.TokenID]...)
						tmap := parentTxnIDMap[work.TokenID]

						for _, info := range work.Chains {
							newID := info.ID
							inserted := false
							if info.PreviousTransactionID != "" && tmap != nil {
								if parentID, ok := tmap[info.PreviousTransactionID]; ok {
									for j, id := range chain {
										if id == parentID {
											chain = append(chain[:j+1], append([]uint64{newID}, chain[j+1:]...)...)
											inserted = true
											break
										}
									}
								}
							}
							if !inserted {
								if info.PreviousTransactionID == "" {
									if len(chain) == 0 {
										chain = []uint64{newID}
									} else {
										chain = append([]uint64{newID}, chain...)
									}
								} else {
									chain = append(chain, newID)
								}
							}
						}
						resultChan <- tokenResult{TokenID: work.TokenID, Chain: chain}
					}
				}()
			}

			wg.Wait()
			close(resultChan)

			for res := range resultChan {
				existingTCA[res.TokenID] = res.Chain
			}

			var tcaUpserts []models.TokenChainArray
			for _, tid := range batchTokenIDs {
				chain := existingTCA[tid]
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

// updateBalances handles DIDBalances (granular) table updates using aggregated values.
func updateBalances(tx *gorm.DB, did, assetType, tokenName, creatorDID string, balanceDelta, pledgedDelta float64) error {
	var peerID string
	var didAlgo int64

	var meta models.DIDBalance
	if tx.Where("did = ? AND peer_id IS NOT NULL AND peer_id != ''", did).
		Select("peer_id, did_algo").First(&meta).Error == nil {
		peerID = meta.PeerID
		didAlgo = meta.DIDAlgo
	}

	return tx.Exec(`
		INSERT INTO did_balances
			(did, asset_type, token_name, creator_did, balance, pledged_balance, peer_id, did_algo)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (did, asset_type, token_name, creator_did)
		DO UPDATE SET
			balance         = GREATEST(0, did_balances.balance         + excluded.balance),
			pledged_balance = GREATEST(0, did_balances.pledged_balance + excluded.pledged_balance),
			peer_id  = CASE WHEN did_balances.peer_id  IS NULL OR did_balances.peer_id  = ''
			                THEN excluded.peer_id  ELSE did_balances.peer_id  END,
			did_algo = CASE WHEN did_balances.did_algo IS NULL OR did_balances.did_algo = 0
			                THEN excluded.did_algo ELSE did_balances.did_algo END
	`, did, assetType, tokenName, creatorDID,
		balanceDelta, pledgedDelta,
		peerID, didAlgo,
	).Error
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

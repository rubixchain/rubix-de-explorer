package services

import (
	"encoding/json"
	"explorer-server/constants"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Block processing counter for progress logging
var blocksProcessed int64

// MapTxnTypeToTokenStatus mirrors the Fullnode logic for token lifecycle
func MapTxnTypeToTokenStatus(txnType string) int {
	switch txnType {
	case constants.TokenBurntType:
		return 8 // wallet.TokenIsBurnt
	case constants.TokenPledgedType:
		return 4 // wallet.TokenIsPledged
	case constants.TokenDeployedType:
		return 9 // wallet.TokenIsDeployed
	case constants.TokenExecutedType:
		return 10 // wallet.TokenIsExecuted
	case constants.TokenIsBurntForFT:
		return 13 // wallet.TokenIsBurntForFT
	case constants.TokenCommittedType, constants.TokenContractCommited:
		return 7 // wallet.TokenIsCommitted
	case constants.TokenPinnedAsService:
		return 12 // wallet.TokenIsPinnedAsService
	default:
		// Covers Minted, Transferred, Migrated, Unpledged, Generated
		return 1 // wallet.TokenIsFree
	}
}

// UpdateBlocks orchestrates the block storage and delegates token updates
func UpdateBlocks(info *model.IncomingBlockInfo) {
	if info == nil || info.BlockMap == nil {
		return
	}

	// 1. Extract and Process the internal block map
	mappedBlock := ProcessIncomingBlock(info.BlockMap)

	// Progress logging
	count := atomic.AddInt64(&blocksProcessed, 1)
	transType := fmt.Sprintf("%v", mappedBlock["TCTransTypeKey"])
	blockType := constants.TxTypeToString(transType)
	blockHash := fmt.Sprintf("%v", mappedBlock["TCBlockHashKey"])

	log.Printf("📦 Block #%d [%s] %s", count, blockType, blockHash)

	if count%50 == 0 {
		if GlobalTxnProcessor != nil {
			GlobalTxnProcessor.workersMutex.RLock()
			currentWorkers := GlobalTxnProcessor.currentWorkers
			GlobalTxnProcessor.workersMutex.RUnlock()

			log.Printf("📊 Progress Summary: %d blocks processed | Queue: %d | Workers: %d",
				count, len(GlobalTxnProcessor.txnQueue), currentWorkers)
		} else {
			log.Printf("📊 Progress Summary: %d blocks processed", count)
		}
	}

	// 2. Data Backfilling: Ensure info fields are populated for the DB modules
	// If Fullnode sends empty strings, we pull from the mappedBlock
	if info.ReceiverDID == "" {
		if owner, ok := mappedBlock["TCTokenOwnerKey"].(string); ok && owner != "" {
			info.ReceiverDID = owner
		} else {
			info.ReceiverDID = info.PublisherDID
		}
	}

	// Extract TxnID if missing
	if info.TransactionID == "" || info.TransactionID == "<nil>" {
		transInfo, _ := mappedBlock["TCTransInfoKey"].(map[string]interface{})
		if tid := stringPtr(getNested(transInfo, "TITIDKey")); tid != nil {
			info.TransactionID = *tid
		}
	}

	// Extract CreatorDID if missing (common in FT minting)
	if info.CreatorDID == "" || info.CreatorDID == "<nil>" {
		transInfo, _ := mappedBlock["TCTransInfoKey"].(map[string]interface{})
		if cdid := stringPtr(getNested(transInfo, "TICreatorDIDKey")); cdid != nil {
			info.CreatorDID = *cdid
		}
	}

	// Backfill TxnType from block map (critical for DID analytics logic)
	// transType already declared above in progress logging
	if info.TxnType == "" {
		info.TxnType = transType
	}

	// Backfill TransactionValue from block map (needed for RBT balance updates)
	if info.TransactionValue == 0 {
		if val, ok := mappedBlock["TCTokenValueKey"].(float64); ok {
			info.TransactionValue = val
		}
	}

	// 3. Execute ALL DB operations in a single transaction
	err := database.WriteDB.Transaction(func(tx *gorm.DB) error {
		StoreBlockInAllBlocks(tx, mappedBlock)

		switch transType {
		case constants.TokenTransferredType:
			StoreTransferBlock(tx, mappedBlock, info)
		case constants.TokenBurntType, constants.TokenIsBurntForFT:
			StoreBurntBlock(tx, mappedBlock)
		case constants.TokenDeployedType:
			StoreSCDeployBlock(tx, mappedBlock)
		case constants.TokenExecutedType:
			StoreSCExecuteBlock(tx, mappedBlock)
		case constants.TokenGeneratedType, constants.TokenMintedType:
			StoreMintBlock(tx, mappedBlock, info)
		default:
			// Handled in AllBlocks only
		}

		// Process Live Updates within the same transaction
		return ProcessLiveTokenUpdates(tx, info)
	})
	if err != nil {
		log.Printf("⚠️ Block processing error: %v", err)
	}
}

// ==========================================
//           Block Storage Functions
// ==========================================

func StoreTransferBlock(tx *gorm.DB, blockMap map[string]interface{}, info *model.IncomingBlockInfo) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})
	tokensJSON, _ := json.Marshal(tokensKey)

	// Receiver DID: try transInfo first, fallback to tokenOwner
	receiverDID := stringPtr(getNested(transInfo, "TIReceiverDIDKey"))
	if receiverDID == nil {
		receiverDID = stringPtr(getNested(blockMap, "TCTokenOwnerKey"))
	}

	// Amount: prefer info.TransactionValue, fallback to block map
	var amount *float64
	if info != nil && info.TransactionValue > 0 {
		amount = &info.TransactionValue
	} else {
		amount = float64Ptr(blockMap["TCTokenValueKey"])
	}

	// Asset type from info
	var assetTypeStr string
	if info != nil {
		assetTypeStr = constants.AssetTypeToString(info.AssetType)
	}

	// Validators: serialize quorumSignature if present
	var validatorsJSON datatypes.JSON
	if qs := blockMap["TCQuorumSignatureKey"]; qs != nil {
		if data, err := json.Marshal(qs); err == nil {
			validatorsJSON = datatypes.JSON(data)
		}
	}

	// Token count: number of tokens in this transaction (useful for FT)
	var tokenCount *int
	if count := len(tokensKey); count > 0 {
		tokenCount = &count
	}

	tb := models.TransactionBlocks{
		BlockHash:   fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		SenderDID:   stringPtr(getNested(transInfo, "TISenderDIDKey")),
		ReceiverDID: receiverDID,
		AssetType:   assetTypeStr,
		TxnID:       stringPtr(getNested(transInfo, "TITIDKey")),
		Amount:      amount,
		TokenCount:  tokenCount,
		Epoch:       int64Ptr(blockMap["TCEpoch"]),
		Tokens:      datatypes.JSON(tokensJSON),
		Validators:  validatorsJSON,
	}
	tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&tb)
}

func StoreBurntBlock(tx *gorm.DB, blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	var childTokensJSON datatypes.JSON
	if tokensKey != nil {
		if data, err := json.Marshal(tokensKey); err == nil {
			childTokensJSON = datatypes.JSON(data)
		}
	}

	var epoch *int64
	if comment, ok := transInfo["TICommentKey"].(string); ok {
		re := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
		if match := re.FindString(comment); match != "" {
			ist, _ := time.LoadLocation("Asia/Kolkata")
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", match, ist); err == nil {
				val := t.Unix()
				epoch = &val
			}
		}
	}

	bb := models.BurntBlocks{
		BlockHash: fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		OwnerDID:  fmt.Sprintf("%v", blockMap["TCTokenOwnerKey"]),
		Epoch:     epoch,
		Tokens:    childTokensJSON,
	}
	tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&bb)
}

func StoreSCDeployBlock(tx *gorm.DB, blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})
	blockID := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])
	var contractID string
	var blockHeight int64
	for k, v := range tokensKey {
		contractID = k
		if tk, ok := v.(map[string]interface{}); ok {
			if bh, ok := tk["TTBlockNumberKey"].(string); ok {
				blockHeight, _ = strconv.ParseInt(bh, 10, 64)
			}
		}
		break
	}
	var epoch time.Time
	if e, ok := blockMap["TCEpoch"].(float64); ok {
		epoch = time.Unix(int64(e), 0)
	}
	scBlock := models.SCBlocks{
		BlockHash:   blockID,
		TokenID:     contractID,
		BlockHeight: blockHeight,
		Epoch:       epoch,
		DeployerDID: fmt.Sprintf("%v", getNested(transInfo, "TIDeployerDIDKey")),
	}
	tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&scBlock)
}

func StoreSCExecuteBlock(tx *gorm.DB, blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})
	blockID := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])
	var contractID string
	var blockHeight int64
	for k, v := range tokensKey {
		contractID = k
		if tk, ok := v.(map[string]interface{}); ok {
			if bh, ok := tk["TTBlockNumberKey"].(string); ok {
				blockHeight, _ = strconv.ParseInt(bh, 10, 64)
			}
		}
		break
	}
	var epoch time.Time
	if e, ok := blockMap["TCEpoch"].(float64); ok {
		epoch = time.Unix(int64(e), 0)
	}
	scBlock := models.SCBlocks{
		BlockHash:   blockID,
		TokenID:     contractID,
		ExecutorDID: stringPtr(getNested(transInfo, "TIExecutorDIDKey")),
		BlockHeight: blockHeight,
		Epoch:       epoch,
		DeployerDID: fmt.Sprintf("%v", getNested(transInfo, "TIDeployerDIDKey")),
	}
	tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&scBlock)
}

func StoreMintBlock(tx *gorm.DB, blockMap map[string]interface{}, info *model.IncomingBlockInfo) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	var tokenIDs []string
	for k := range tokensKey {
		tokenIDs = append(tokenIDs, k)
	}

	var creatorDID string
	if info.CreatorDID != "" {
		creatorDID = info.CreatorDID
	} else {
		creatorDID = info.PublisherDID
	}
	var ftName *string
	if info.FTName != "" {
		ftName = &info.FTName
	}

	mb := models.MintBlocks{
		BlockHash:  fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		TokenIDs:   pq.StringArray(tokenIDs),
		TokenType:  constants.AssetTypeToString(info.AssetType),
		TokenValue: float64Ptr(blockMap["TCTokenValueKey"]),
		CreatorDID: creatorDID,
		FTName:     ftName,
		Epoch:      int64Ptr(blockMap["TCEpoch"]),
	}
	tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&mb)
}

func StoreBlockInAllBlocks(tx *gorm.DB, blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	blockHash := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])
	txnID := fmt.Sprintf("%v", transInfo["TITIDKey"])

	// Get the raw type string (e.g., "02")
	rawType := fmt.Sprintf("%v", blockMap["TCTransTypeKey"])

	// INTEGRATION: Call your helper function to get "Transferred", "Minted", etc.
	blockType := constants.TxTypeToString(rawType)

	record := models.AllBlocks{
		BlockHash: blockHash,
		BlockType: blockType,
		TxnID:     txnID,
	}

	tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
}

// ==========================================
//           Token Update Functions
// ==========================================

// ProcessLiveTokenUpdates orchestrates the registry update and delegates to specific asset modules
func ProcessLiveTokenUpdates(tx *gorm.DB, info *model.IncomingBlockInfo) error {
	if len(info.TokenDetails) == 0 {
		return nil
	}

	tokenStatus := MapTxnTypeToTokenStatus(info.TxnType)

	for _, token := range info.TokenDetails {
		// 1. Update Global Token Registry
		if err := updateTokenRegistry(tx, token.TokenID, info.AssetType); err != nil {
			return err
		}

		// 2. Delegate to specific token modules
		var err error
		switch info.AssetType {
		case constants.RBTTokenAssetType:
			err = handleRBTUpdate(tx, info, token, tokenStatus)
		case constants.FTTokenAssetType:
			err = handleFTUpdate(tx, info, token, tokenStatus)
		case constants.NFTTokenAssetType:
			err = handleNFTUpdate(tx, info, token, tokenStatus)
		case constants.SmartContractTokenAssetType:
			err = handleSCUpdate(tx, info, token, tokenStatus)
		}

		if err != nil {
			return err
		}
	}

	// 3. Update DID level analytics
	return updateDIDAnalytics(tx, info)
}

// ==========================================
//           Asset Specific Modules
// ==========================================

func updateTokenRegistry(tx *gorm.DB, tokenID string, assetType int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"token_type"}),
	}).Create(&models.AllTokens{
		TokenID:   tokenID,
		TokenType: constants.AssetTypeToString(assetType),
	}).Error
}

func handleRBTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_hash", "block_height", "token_status"}),
	}).Create(&models.RBT{
		TokenID:     token.TokenID,
		OwnerDID:    info.ReceiverDID,
		BlockHash:   info.BlockHash,
		BlockHeight: fmt.Sprintf("%d", info.LatestBlockHeight),
		TokenValue:  token.TokenValue,
		TokenStatus: status,
	}).Error
}

func handleFTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_height", "token_status"}),
	}).Create(&models.FT{
		TokenID:     token.TokenID,
		TokenValue:  token.TokenValue,
		FTName:      info.FTName,
		OwnerDID:    info.ReceiverDID,
		CreatorDID:  info.CreatorDID,
		BlockHeight: info.LatestBlockHeight,
		TokenStatus: status,
	}).Error
}

func handleNFTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_hash", "txn_id", "block_height", "token_status"}),
	}).Create(&models.NFT{
		TokenID:     token.TokenID,
		TokenValue:  fmt.Sprintf("%f", token.TokenValue),
		OwnerDID:    info.ReceiverDID,
		BlockHeight: info.LatestBlockHeight,
		TokenStatus: status,
	}).Error
}

func handleSCUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"block_hash", "block_height", "token_status", "executor_did"}),
	}).Create(&models.SC{
		TokenID:     token.TokenID,
		BlockHash:   info.BlockHash,
		DeployerDID: info.CreatorDID,
		ExecutorDID: info.ReceiverDID,
		BlockHeight: uint64(info.LatestBlockHeight),
		TokenStatus: status,
	}).Error
}

func updateDIDAnalytics(tx *gorm.DB, info *model.IncomingBlockInfo) error {
	// Collect all unique DIDs involved
	didSet := make(map[string]bool)
	if info.PublisherDID != "" {
		didSet[info.PublisherDID] = true
	}
	if info.ReceiverDID != "" {
		didSet[info.ReceiverDID] = true
	}
	if info.CreatorDID != "" {
		didSet[info.CreatorDID] = true
	}

	// Ensure all DIDs exist in the table
	for did := range didSet {
		tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.DIDs{DID: did})
	}

	isTransfer := info.TxnType == constants.TokenTransferredType
	isBurn := info.TxnType == constants.TokenBurntType || info.TxnType == constants.TokenIsBurntForFT

	// Logic:
	// 1. Decrement Sender/Publisher ONLY if it's a collected transfer or burn.
	//    (If it's a Mint, we do NOT decrement, effectively creating new value).
	// 2. Increment Receiver ALWAYS (except maybe burns, but usually receiver is nil or burn address there).

	// Update asset-specific counters (use GREATEST to prevent negatives)
	switch info.AssetType {
	case constants.RBTTokenAssetType:
		if info.TransactionValue > 0 {
			if info.PublisherDID != "" && (isTransfer || isBurn) {
				tx.Model(&models.DIDs{}).Where("did = ?", info.PublisherDID).
					UpdateColumn("total_rbts", gorm.Expr("GREATEST(0, total_rbts - ?)", info.TransactionValue))
			}
			if info.ReceiverDID != "" && !isBurn {
				tx.Model(&models.DIDs{}).Where("did = ?", info.ReceiverDID).
					UpdateColumn("total_rbts", gorm.Expr("total_rbts + ?", info.TransactionValue))
			}
		}
	case constants.FTTokenAssetType:
		count := len(info.TokenDetails)
		if count > 0 {
			if info.PublisherDID != "" && (isTransfer || isBurn) {
				tx.Model(&models.DIDs{}).Where("did = ?", info.PublisherDID).
					UpdateColumn("total_fts", gorm.Expr("GREATEST(0, total_fts - ?)", count))
			}
			if info.ReceiverDID != "" && !isBurn {
				tx.Model(&models.DIDs{}).Where("did = ?", info.ReceiverDID).
					UpdateColumn("total_fts", gorm.Expr("total_fts + ?", count))
			}
		}
	case constants.NFTTokenAssetType:
		count := len(info.TokenDetails)
		if count > 0 {
			if info.PublisherDID != "" && (isTransfer || isBurn) {
				tx.Model(&models.DIDs{}).Where("did = ?", info.PublisherDID).
					UpdateColumn("total_nfts", gorm.Expr("GREATEST(0, total_nfts - ?)", count))
			}
			if info.ReceiverDID != "" && !isBurn {
				tx.Model(&models.DIDs{}).Where("did = ?", info.ReceiverDID).
					UpdateColumn("total_nfts", gorm.Expr("total_nfts + ?", count))
			}
		}
	case constants.SmartContractTokenAssetType:
		count := len(info.TokenDetails)
		if count > 0 {
			if info.PublisherDID != "" && (isTransfer || isBurn) {
				tx.Model(&models.DIDs{}).Where("did = ?", info.PublisherDID).
					UpdateColumn("total_sc", gorm.Expr("GREATEST(0, total_sc - ?)", count))
			}
			if info.ReceiverDID != "" && !isBurn {
				tx.Model(&models.DIDs{}).Where("did = ?", info.ReceiverDID).
					UpdateColumn("total_sc", gorm.Expr("total_sc + ?", count))
			}
		}
	}

	return nil
}

// Helpers
func getNested(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}

func stringPtr(v interface{}) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprintf("%v", v)
	if s == "" || s == "<nil>" {
		return nil
	}
	return &s
}

func float64Ptr(v interface{}) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func int64Ptr(v interface{}) *int64 {
	if f, ok := v.(float64); ok {
		i := int64(f)
		return &i
	}
	return nil
}

func ProcessIncomingBlock(blockData map[string]interface{}) map[string]interface{} {
	flattened := util.FlattenKeys("", blockData).(map[string]interface{})
	mapped := util.ApplyKeyMapping(flattened).(map[string]interface{})
	return mapped
}

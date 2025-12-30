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
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
func UpdateBlocks(blockMap map[string]interface{}, info *model.IncomingBlockInfo) {
	if blockMap == nil || info == nil {
		return
	}

	mappedBlock := ProcessIncomingBlock(blockMap)
	StoreBlockInAllBlocks(mappedBlock)

	transType := fmt.Sprintf("%v", mappedBlock["TCTransTypeKey"])

	switch transType {
	case constants.TokenTransferredType:
		StoreTransferBlock(mappedBlock)
	case constants.TokenBurntType, constants.TokenIsBurntForFT:
		StoreBurntBlock(mappedBlock)
	case constants.TokenDeployedType:
		StoreSCDeployBlock(mappedBlock)
	case constants.TokenExecutedType:
		StoreSCExecuteBlock(mappedBlock)
	case constants.TokenGeneratedType, constants.TokenMintedType:
		StoreMintBlock(mappedBlock)
	default:
		log.Printf("📥 Block Type: %s (Handled in AllBlocks only)", transType)
	}

	if err := ProcessLiveTokenUpdates(info); err != nil {
		log.Printf("⚠️ Token/DID Update error: %v", err)
	}
}

// ==========================================
//           Block Storage Functions
// ==========================================

func StoreTransferBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})
	tokensJSON, _ := json.Marshal(tokensKey)

	tb := models.TransferBlocks{
		BlockHash:   fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		PrevBlockID: stringPtr(getNested(transInfo, "TTPreviousBlockIDKey")),
		SenderDID:   stringPtr(getNested(transInfo, "TISenderDIDKey")),
		ReceiverDID: stringPtr(getNested(transInfo, "TIReceiverDIDKey")),
		TxnType:     stringPtr(getNested(blockMap, "TCTransTypeKey")),
		TxnID:       stringPtr(getNested(transInfo, "TITIDKey")),
		Amount:      float64Ptr(blockMap["TCTokenValueKey"]),
		Epoch:       int64Ptr(blockMap["TCEpoch"]),
		Tokens:      datatypes.JSON(tokensJSON),
		// ValidatorPledgeMap removed as requested
	}
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&tb)
}

func StoreBurntBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})
	tokensJSON, _ := json.Marshal(tokensKey)

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

	txnType := fmt.Sprintf("%v", blockMap["TCTransTypeKey"])
	bb := models.BurntBlocks{
		BlockHash: fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		TxnType:   &txnType,
		OwnerDID:  fmt.Sprintf("%v", blockMap["TCTokenOwnerKey"]),
		Epoch:     epoch,
		Tokens:    datatypes.JSON(tokensJSON),
		// ChildTokens removed as requested
	}
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&bb)
}

func StoreSCDeployBlock(blockMap map[string]interface{}) {
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
	scBlock := models.SC_Block{
		Block_ID:     blockID,
		Contract_ID:  contractID,
		Block_Height: blockHeight,
		Epoch:        epoch,
		Owner_DID:    fmt.Sprintf("%v", getNested(transInfo, "TIDeployerDIDKey")),
	}
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&scBlock)
}

func StoreSCExecuteBlock(blockMap map[string]interface{}) {
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
	scBlock := models.SC_Block{
		Block_ID:     blockID,
		Contract_ID:  contractID,
		Executor_DID: stringPtr(getNested(transInfo, "TIExecutorDIDKey")),
		Block_Height: blockHeight,
		Epoch:        epoch,
		Owner_DID:    fmt.Sprintf("%v", getNested(transInfo, "TIReceiverDIDKey")),
	}
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&scBlock)
}

func StoreMintBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	var tokenIDs []string
	for k := range tokensKey {
		tokenIDs = append(tokenIDs, k)
	}

	assetTypeInt := 0
	if val, ok := blockMap["TCAssetTypeKey"].(float64); ok {
		assetTypeInt = int(val)
	}

	txnType := fmt.Sprintf("%v", blockMap["TCTransTypeKey"])
	mb := models.MintBlocks{
		BlockHash:  fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		TokenIDs:   pq.StringArray(tokenIDs),
		TokenType:  constants.AssetTypeToString(assetTypeInt),
		TokenValue: float64Ptr(blockMap["TCTokenValueKey"]),
		OwnerDID:   fmt.Sprintf("%v", blockMap["TCTokenOwnerKey"]),
		CreatorDID: stringPtr(getNested(transInfo, "TICreatorDIDKey")),
		FTName:     stringPtr(blockMap["TCFTNameKey"]),
		Epoch:      int64Ptr(blockMap["TCEpoch"]),
		TxnType:    &txnType,
	}
	database.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&mb)
}

func StoreBlockInAllBlocks(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	blockHash := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])
	txnID := fmt.Sprintf("%v", transInfo["TITIDKey"])

	var blockType string
	switch fmt.Sprintf("%v", blockMap["TCTransTypeKey"]) {
	case "02":
		blockType = "transfer"
	case "08":
		blockType = "burnt"
	case "13":
		blockType = "burnt_for_ft"
	case "09":
		blockType = "deploy"
	case "10":
		blockType = "execute"
	case "05":
		blockType = "mint"
	default:
		blockType = "unknown"
	}

	var epochTime time.Time
	if v, ok := blockMap["TCEpoch"].(float64); ok {
		epochTime = time.Unix(int64(v), 0)
	} else {
		epochTime = time.Now()
	}

	record := models.AllBlocks{BlockHash: blockHash, BlockType: blockType, Epoch: epochTime, TxnID: txnID}
	database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
}

// ==========================================
//           Token Update Functions
// ==========================================

// ProcessLiveTokenUpdates orchestrates the registry update and delegates to specific asset modules
func ProcessLiveTokenUpdates(info *model.IncomingBlockInfo) error {
	if len(info.TokenDetails) == 0 {
		return nil
	}

	tokenStatus := MapTxnTypeToTokenStatus(info.TxnType)

	return database.DB.Transaction(func(tx *gorm.DB) error {
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
	})
}

// ==========================================
//           Asset Specific Modules
// ==========================================

func updateTokenRegistry(tx *gorm.DB, tokenID string, assetType int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"token_type", "last_updated"}),
	}).Create(&models.TokenType{
		TokenID:     tokenID,
		TokenType:   constants.AssetTypeToString(assetType),
		LastUpdated: time.Now(),
	}).Error
}

func handleRBTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "rbt_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_id", "block_height", "token_status"}),
	}).Create(&models.RBT{
		TokenID:     token.TokenID,
		OwnerDID:    info.ReceiverDID,
		BlockID:     info.BlockHash,
		BlockHeight: fmt.Sprintf("%d", info.LatestBlockHeight),
		TokenValue:  token.TokenValue,
		TokenStatus: status,
	}).Error
}

func handleFTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ft_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_height", "block_id", "txn_id", "token_status"}),
	}).Create(&models.FT{
		FtID:        token.TokenID,
		TokenValue:  token.TokenValue,
		FTName:      info.FTName,
		OwnerDID:    info.ReceiverDID,
		CreatorDID:  info.CreatorDID,
		BlockHeight: info.LatestBlockHeight,
		BlockID:     info.BlockHash,
		Txn_ID:      info.TransactionID,
		TokenStatus: status,
	}).Error
}

func handleNFTUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "nft_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_did", "block_hash", "txn_id", "block_height", "token_status"}),
	}).Create(&models.NFT{
		TokenID:     token.TokenID,
		TokenValue:  fmt.Sprintf("%f", token.TokenValue),
		OwnerDID:    info.ReceiverDID,
		BlockHash:   info.BlockHash,
		Txn_ID:      info.TransactionID,
		BlockHeight: info.LatestBlockHeight,
		TokenStatus: status,
	}).Error
}

func handleSCUpdate(tx *gorm.DB, info *model.IncomingBlockInfo, token model.TokenDetails, status int) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "contract_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"block_hash", "txn_id", "block_height", "token_status"}),
	}).Create(&models.SmartContract{
		ContractID:  token.TokenID,
		BlockHash:   info.BlockHash,
		DeployerDID: info.CreatorDID,
		TxnId:       info.TransactionID,
		BlockHeight: uint64(info.LatestBlockHeight),
		TokenStatus: status,
	}).Error
}

func updateDIDAnalytics(tx *gorm.DB, info *model.IncomingBlockInfo) error {
	dids := []string{info.PublisherDID}
	if info.ReceiverDID != "" {
		dids = append(dids, info.ReceiverDID)
	}
	for _, did := range dids {
		if did == "" {
			continue
		}
		tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.DIDs{DID: did, CreatedAt: time.Now()})
	}

	if info.AssetType == constants.RBTTokenAssetType && info.ReceiverDID != "" {
		tx.Model(&models.DIDs{}).Where("did = ?", info.PublisherDID).UpdateColumn("total_rbts", gorm.Expr("total_rbts - ?", info.TransactionValue))
		tx.Model(&models.DIDs{}).Where("did = ?", info.ReceiverDID).UpdateColumn("total_rbts", gorm.Expr("total_rbts + ?", info.TransactionValue))
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

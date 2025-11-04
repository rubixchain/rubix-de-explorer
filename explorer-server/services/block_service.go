package services

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"fmt"
)

// GetTxnsCount returns the total number of Blocks in the database
func GetTxnsCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.RBT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	fmt.Printf("count", count)
	return count, nil
}

func GetTransferBlocksList(limit, page int) (model.TransactionsResponse, error) {
	var blocks []models.TransferBlocks
	var response model.TransactionsResponse

	// Calculate correct offset
	offset := (page - 1) * limit

	// Fetch paginated data
	if err := database.DB.
		Limit(limit).
		Offset(offset).
		Order("epoch DESC"). // optional: keeps pagination consistent
		Find(&blocks).Error; err != nil {
		return model.TransactionsResponse{}, err
	}

	// Count total records
	var count int64
	if err := database.DB.Model(&models.TransferBlocks{}).Count(&count).Error; err != nil {
		return model.TransactionsResponse{}, err
	}

	// Map DB model to response model
	for _, b := range blocks {
		tx := model.TransactionResponse{
			TxnHash:     deref(b.TxnID),
			TxnType:     deref(b.TxnType),
			Amount:      derefFloat(b.Amount),
			SenderDID:   deref(b.SenderDID),
			ReceiverDID: deref(b.ReceiverDID),
		}
		response.TransactionsResponse = append(response.TransactionsResponse, tx)
	}

	response.Count = count
	return response, nil
}


func GetTransferBlockInfoFromTxnID(hash string) (models.TransferBlocks, error) {
	var block models.TransferBlocks

	// Fetch block where block_hash matches
	if err := database.DB.
		Where("txn_id = ?", hash).
		First(&block).Error; err != nil {
		return models.TransferBlocks{}, err
	}

	// Unmarshal JSON fields
	var tokens []string
	if err := json.Unmarshal(block.Tokens, &tokens); err != nil {
		fmt.Println("Error unmarshaling tokens:", err)
	}

	var validatorMap map[string][]string
	if err := json.Unmarshal(block.ValidatorPledgeMap, &validatorMap); err != nil {
		fmt.Println("Error unmarshaling validator map:", err)
	}

	// Print nicely
	fmt.Printf(
		"BlockHash: %s\nSender: %s\nReceiver: %s\nTxnType: %s\nAmount: %v\nEpoch: %v\nTokens: %v\nValidatorPledgeMap: %v\nTxnID: %s\n",
		block.BlockHash, // already string
		derefStringPtr(block.SenderDID),
		derefStringPtr(block.ReceiverDID),
		derefStringPtr(block.TxnType),
		derefFloatPtr(block.Amount),
		derefInt64Ptr(block.Epoch),
		tokens,
		validatorMap,
		derefStringPtr(block.TxnID),
	)

	return block, nil
}

func GetTransferBlockInfoFromBlockHash(hash string) (models.TransferBlocks, error) {
	var block models.TransferBlocks

	// Fetch block where block_hash matches
	if err := database.DB.
		Where("block_hash = ?", hash).
		First(&block).Error; err != nil {
		return models.TransferBlocks{}, err
	}

	// Unmarshal JSON fields
	var tokens []string
	if err := json.Unmarshal(block.Tokens, &tokens); err != nil {
		fmt.Println("Error unmarshaling tokens:", err)
	}

	var validatorMap map[string][]string
	if err := json.Unmarshal(block.ValidatorPledgeMap, &validatorMap); err != nil {
		fmt.Println("Error unmarshaling validator map:", err)
	}

	// Print nicely
	fmt.Printf(
		"BlockHash: %s\nSender: %s\nReceiver: %s\nTxnType: %s\nAmount: %v\nEpoch: %v\nTokens: %v\nValidatorPledgeMap: %v\nTxnID: %s\n",
		block.BlockHash, // already string
		derefStringPtr(block.SenderDID),
		derefStringPtr(block.ReceiverDID),
		derefStringPtr(block.TxnType),
		derefFloatPtr(block.Amount),
		derefInt64Ptr(block.Epoch),
		tokens,
		validatorMap,
		derefStringPtr(block.TxnID),
	)

	return block, nil
}

func GetBlockType(txnId string) (int64, error) {
	var blockType int64

	err := database.DB.
		Table("all_blocks").
		Select("block_type").
		Where("txn_id = ?", txnId).
		Scan(&blockType).Error

	if err != nil {
		return 0, fmt.Errorf("❌ failed to get block_type for txn_id %s: %v", txnId, err)
	}

	return blockType, nil
}

func GetSCBlockInfoFromTxnId(hash string) (interface{}, error) {

	var block models.SC_Block

	// Fetch block where block_hash matches
	if err := database.DB.
		Where("block_id = ?", hash).
		First(&block).Error; err != nil {
		return models.SC_Block{}, err
	}

	return block, nil

}
func GetBurntBlockInfo(hash string) (interface{}, error) {
	var block models.BurntBlocks

	// Fetch block where block_hash matches
	if err := database.DB.
		Where("block_hash = ?", hash).
		First(&block).Error; err != nil {
		return models.BurntBlocks{}, err
	}

	return block, nil
}

func GetBurntBlockList(limit, page int) (interface{}, error) {
	var blocks []models.BurntBlocks

	offset := (page - 1) * limit

	// Fetch all blocks with pagination
	if err := database.DB.
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&blocks).Error; err != nil {
		return nil, err
	}

	var count int64
	if err := database.DB.Model(&models.BurntBlocks{}).Count(&count).Error; err != nil {
		return model.BurntBlocksListResponse{}, err
	}

	// Wrap in response struct
	response := model.BurntBlocksListResponse{
		BurntBlocks: blocks,
		Count : count,
	}

	return response, nil
}

// Helper functions
func derefStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefFloatPtr(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func derefInt64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// Helper functions to safely deref pointers
// func derefString(s *string) *string {
// 	if s == nil {
// 		empty := ""
// 		return &empty
// 	}
// 	return s
// }

// func derefInt64(i *int64) *int64 {
// 	if i == nil {
// 		zero := int64(0)
// 		return &zero
// 	}
// 	return i
// }

// helper functions
func deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func derefFloat(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

// ProcessIncomingBlock flattens numeric keys and maps them to readable names
func ProcessIncomingBlock(blockData map[string]interface{}) map[string]interface{} {
	// Step 1: Flatten nested numeric keys like "4-2-5"
	flattened := util.FlattenKeys("", blockData).(map[string]interface{})

	// Step 2: Apply mapping (e.g., 1 → TCTokenTypeKey, etc.)
	mapped := util.ApplyKeyMapping(flattened).(map[string]interface{})

	return mapped
}

// UpdateBlocks processes an incoming block and routes it to the right storage function
func UpdateBlocks(blockMap map[string]interface{}) {

	// Convert numeric keys → named keys
	mappedBlock := ProcessIncomingBlock(blockMap)

	// Store in AllBlocks first (universal record)
	StoreBlockInAllBlocks(mappedBlock)

	// Identify transaction type
	transType, _ := mappedBlock["TCTransTypeKey"].(string)

	switch transType {
	case "02", "2":
		fmt.Println("Storing transfer block")
		StoreTransferBlock(mappedBlock)

	case "08", "13":
		fmt.Println("Storing burnt block")
		StoreBurntBlock(mappedBlock)

	case "09", "9":
		fmt.Println("Storing smart contract deploy block")
		StoreSCDeployBlock(mappedBlock)

	case "10":
		fmt.Println("Storing smart contract execute block")
		StoreSCExecuteBlock(mappedBlock)
	}
}

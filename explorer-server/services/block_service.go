package services

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
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

func GetTransferBlocksList(limit, offset int) (model.TransactionsResponse, error) {
	var blocks []models.TransferBlocks
	var response model.TransactionsResponse

	// Fetch all blocks with pagination
	if err := database.DB.
		Limit(limit).
		Offset(offset).
		Find(&blocks).Error; err != nil {
		return model.TransactionsResponse{}, err
	}

	// Map DB model to response model
	for _, b := range blocks {
		tx := model.TransactionResponse{
			TxnHash:     *b.TxnID,
			TxnType:     deref(b.TxnType),
			Amount:      derefFloat(b.Amount),
			SenderDID:   deref(b.SenderDID),
			ReceiverDID: deref(b.ReceiverDID),
		}
		response.TransactionsResponse = append(response.TransactionsResponse, tx)
	}

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

func GetSCBlockInfoFromTxnId(hash string ) (interface{},error){
  
	var block models.SC_Block

	// Fetch block where block_hash matches
	if err := database.DB.
		Where("txn_id = ?", hash).
		First(&block).Error; err != nil {
		return models.SC_Block{}, err
	}

	return block, nil
	
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



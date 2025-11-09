package services

import (
	"encoding/json"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"explorer-server/util"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// GetTxnsCount returns total number of RBT records
func GetTxnsCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.RBT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	fmt.Printf("Total RBT count: %d\n", count)
	return count, nil
}

func GetTransferBlocksList(limit, page int) (model.TransactionsResponse, error) {
	var blocks []models.TransferBlocks
	var response model.TransactionsResponse

	offset := (page - 1) * limit

	// Fetch paginated blocks
	if err := database.DB.
		Order("epoch DESC").
		Limit(limit).
		Offset(offset).
		Find(&blocks).Error; err != nil {
		return response, err
	}

	// Count total records
	var count int64
	if err := database.DB.Model(&models.TransferBlocks{}).Count(&count).Error; err != nil {
		return response, err
	}

	for _, b := range blocks {
		if (b.Amount == nil || *b.Amount == 0) && b.TxnID != nil && *b.TxnID != "" {
			if newAmt := fetchTxnAmountFromFullNode(*b.TxnID); newAmt != nil {
				b.Amount = newAmt
				database.DB.Model(&b).Update("amount", *newAmt)
			}
		}

		response.TransactionsResponse = append(response.TransactionsResponse, model.TransactionResponse{
			TxnHash:     deref(b.TxnID),
			TxnType:     deref(b.TxnType),
			Amount:      derefFloat(b.Amount),
			SenderDID:   deref(b.SenderDID),
			ReceiverDID: deref(b.ReceiverDID),
			Epoch:       b.Epoch,
		})

		log.Printf("epoch: %d\n", b.Epoch)
	}
	log.Printf("Total Transfer Blocks fetched: %d\n", len(response.TransactionsResponse))

	response.Count = count
	return response, nil
}

func GetTransferBlockInfoFromTxnID(hash string) (models.TransferBlocks, error) {
	var block models.TransferBlocks

	if err := database.DB.Where("txn_id = ?", hash).First(&block).Error; err != nil {
		return block, err
	}

	// Fetch missing amount if needed
	if (block.Amount == nil || *block.Amount == 0) && block.TxnID != nil && *block.TxnID != "" {
		if newAmt := fetchTxnAmountFromFullNode(*block.TxnID); newAmt != nil {
			block.Amount = newAmt
			if err := database.DB.Model(&block).Update("amount", *newAmt).Error; err != nil {
				fmt.Printf("⚠️ Failed to update amount in DB for txnID %s: %v\n", *block.TxnID, err)
			} else {
				fmt.Printf("✅ Updated amount %.6f for txnID %s\n", *newAmt, *block.TxnID)
			}
		}
	}

	return block, nil
}

func GetTransferBlockInfoFromBlockHash(hash string) (models.TransferBlocks, error) {
	var block models.TransferBlocks

	if err := database.DB.Where("block_hash = ?", hash).First(&block).Error; err != nil {
		return block, err
	}

	if (block.Amount == nil || *block.Amount == 0) && block.TxnID != nil && *block.TxnID != "" {
		apiURL := fmt.Sprintf("%s/api/de-exp/get-txn-amount-by-txnID?txnID=%s", config.RubixNodeURL, *block.TxnID)
		resp, err := http.Get(apiURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var result struct {
				Status  bool   `json:"status"`
				Message string `json:"message"`
				Result  struct {
					TransactionID    string  `json:"TransactionID"`
					TransactionValue float64 `json:"TransactionValue"`
					BlockHash        string  `json:"BlockHash"`
				} `json:"result"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Status && result.Result.TransactionValue != 0 {
				block.Amount = &result.Result.TransactionValue
				database.DB.Model(&models.TransferBlocks{}).Where("txn_id = ?", block.TxnID).Update("amount", block.Amount)
			}
		}
	}

	return block, nil
}

func fetchTxnAmountFromFullNode(txnID string) *float64 {
	url := fmt.Sprintf("%s/api/de-exp/get-txn-amount-by-txnID?txnID=%s", config.RubixNodeURL, txnID)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ Failed to call fullnode for txnID %s: %v\n", txnID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("⚠️ Fullnode API returned %d for txnID %s\n", resp.StatusCode, txnID)
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("⚠️ Error reading response for txnID %s: %v\n", txnID, err)
		return nil
	}

	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Result  struct {
			TransactionID    string  `json:"TransactionID"`
			TransactionValue float64 `json:"TransactionValue"`
			BlockHash        string  `json:"BlockHash"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil || !result.Status {
		fmt.Printf("⚠️ Could not parse amount for txnID %s: %v\n", txnID, err)
		return nil
	}

	return &result.Result.TransactionValue
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
		Order("epoch DESC").
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
		Count:       count,
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

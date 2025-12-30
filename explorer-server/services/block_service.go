package services

import (
	"encoding/json"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"io"
	"log"
	"net/http"
)

// GetTxnsCount returns total number of TransferBlocks records
func GetTxnsCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.TransferBlocks{}).Count(&count).Error; err != nil {
		return 0, err
	}
	fmt.Printf("Total RBT count: %d\n", count)
	return count, nil
}

func GetTransferBlocksList(limit, page int) (model.TransactionsResponse, error) {
	var blocks []models.TransferBlocks
	var response model.TransactionsResponse

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Fetch paginated blocks
	if err := database.DB.
		Where("epoch IS NOT NULL AND epoch <> 0").
		Order("epoch DESC").
		Limit(limit).
		Offset(offset).
		Find(&blocks).Error; err != nil {
		return response, err
	}

	// Count total records
	var count int64
	if err := database.DB.
		Model(&models.TransferBlocks{}).
		Where("epoch IS NOT NULL AND epoch <> 0").
		Count(&count).Error; err != nil {
		return response, err
	}

	for _, b := range blocks {
		if (b.Amount == nil || *b.Amount == 0) && b.TxnID != nil && *b.TxnID != "" {
			if newAmt := fetchTxnAmountFromFullNode(*b.TxnID); newAmt != nil {
				b.Amount = newAmt
				_ = database.DB.Model(&b).Update("amount", *newAmt).Error
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

	// If amount is missing, fetch it from fullnode
	if (block.Amount == nil || *block.Amount == 0) && block.TxnID != nil && *block.TxnID != "" {
		apiURL := fmt.Sprintf("%s/api/de-exp/get-txn-amount-by-txnID?txnID=%s",
			config.RubixNodeURL, *block.TxnID,
		)

		client := GetNodeHTTPClient()
		release := acquireNodeSlot()
		defer release()

		resp, err := client.Get(apiURL)
		if err != nil {
			log.Printf("⚠️ Failed to call fullnode for txn %s: %v", *block.TxnID, err)
		} else {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("⚠️ Fullnode returned %d for txn %s", resp.StatusCode, *block.TxnID)
			} else {
				var result struct {
					Status  bool   `json:"status"`
					Message string `json:"message"`
					Result  struct {
						TransactionID    string  `json:"TransactionID"`
						TransactionValue float64 `json:"TransactionValue"`
						BlockHash        string  `json:"BlockHash"`
					} `json:"result"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Status {
					if result.Result.TransactionValue != 0 {
						block.Amount = &result.Result.TransactionValue

						_ = database.DB.
							Model(&models.TransferBlocks{}).
							Where("txn_id = ?", block.TxnID).
							Update("amount", block.Amount).Error
					}
				}
			}
		}
	}

	return block, nil
}

func fetchTxnAmountFromFullNode(txnID string) *float64 {
	url := fmt.Sprintf("%s/api/de-exp/get-txn-amount-by-txnID?txnID=%s",
		config.RubixNodeURL, txnID,
	)

	client := GetNodeHTTPClient()
	release := acquireNodeSlot()
	defer release()

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("❌ Failed to call fullnode for txnID %s: %v\n", txnID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("⚠️ Fullnode API returned %d for txnID %s\n", resp.StatusCode, txnID)
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
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
	// NOTE: DB column block_type is string (transfer/burnt/etc.).
	// Keeping signature as int64 to match existing usage.
	var blockTypeStr string

	err := database.DB.
		Table("all_blocks").
		Select("block_type").
		Where("txn_id = ?", txnId).
		Scan(&blockTypeStr).Error

	if err != nil {
		return 0, fmt.Errorf("❌ failed to get block_type for txn_id %s: %v", txnId, err)
	}

	// Map string types to numeric codes if needed (keeping simple & backward-compatible).
	switch blockTypeStr {
	case "transfer":
		return 1, nil
	case "burnt":
		return 2, nil
	case "burnt_for_ft":
		return 3, nil
	case "deploy":
		return 4, nil
	case "execute":
		return 5, nil
	case "mint":
		return 6, nil
	default:
		return 0, nil
	}
}

func GetSCBlockInfoFromTxnId(hash string) (interface{}, error) {
	var block models.SC_Block

	if err := database.DB.
		Where("block_id = ?", hash).
		First(&block).Error; err != nil {
		return models.SC_Block{}, err
	}

	return block, nil
}

func GetBurntBlockInfo(hash string) (interface{}, error) {
	var block models.BurntBlocks

	if err := database.DB.
		Where("block_hash = ?", hash).
		First(&block).Error; err != nil {
		return models.BurntBlocks{}, err
	}

	return block, nil
}

func GetBurntBlockList(limit, page int) (interface{}, error) {
	var blocks []models.BurntBlocks

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	if err := database.DB.
		Order("epoch DESC").
		Limit(limit).
		Offset(offset).
		Find(&blocks).Error; err != nil {
		return nil, err
	}

	var count int64
	if err := database.DB.Model(&models.BurntBlocks{}).Count(&count).Error; err != nil {
		return model.BurntBlocksListResponse{}, err
	}

	response := model.BurntBlocksListResponse{
		BurntBlocks: blocks,
		Count:       count,
	}

	return response, nil
}

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

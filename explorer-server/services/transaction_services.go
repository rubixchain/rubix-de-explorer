package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"explorer-server/database"
	"explorer-server/database/models"
)

// SyncMissingTxnAmounts checks TransferBlocks for missing amounts,
// fetches from fullnode API, and updates DB.
func SyncMissingTxnAmounts() {
	const apiURL = "http://localhost:20010/api/de-exp/get-txn-amount-by-txnID?txnID=%s"

	var blocks []models.TransferBlocks
	if err := database.DB.Where("amount IS NULL OR amount = 0").Find(&blocks).Error; err != nil {
		log.Printf("❌ Failed to query TransferBlocks: %v", err)
		return
	}

	log.Printf("🔍 Found %d transactions without amount", len(blocks))
	if len(blocks) == 0 {
		return
	}

	for _, b := range blocks {
		if b.TxnID == nil || *b.TxnID == "" {
			continue
		}

		url := fmt.Sprintf(apiURL, *b.TxnID)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("⚠️ Failed to fetch txn amount for %s: %v", *b.TxnID, err)
			continue
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		var result struct {
			Status  bool   `json:"status"`
			Message string `json:"message"`
			Result  struct {
				TransactionID    string  `json:"TransactionID"`
				TransactionValue float64 `json:"TransactionValue"`
				BlockHash        string  `json:"BlockHash"`
			} `json:"result"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			log.Printf("⚠️ JSON parse error for %s: %v", *b.TxnID, err)
			continue
		}
		if !result.Status {
			log.Printf("⚠️ Fullnode returned failure for %s: %s", *b.TxnID, result.Message)
			continue
		}

		// Update the amount in DB
		if err := database.DB.Model(&models.TransferBlocks{}).
			Where("txn_id = ?", *b.TxnID).
			Update("amount", result.Result.TransactionValue).Error; err != nil {
			log.Printf("❌ Failed to update amount for txn_id %s: %v", *b.TxnID, err)
			continue
		}

		log.Printf("✅ Updated txn_id=%s with amount=%.6f (block_hash=%s)",
			*b.TxnID, result.Result.TransactionValue, result.Result.BlockHash)
	}

	log.Println("🎯 Sync completed.")
}

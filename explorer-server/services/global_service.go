package services

import (
	"encoding/json"
	"errors"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
	"io"
	"net/http"
)

// Map token type codes to readable names
var transactionTypeNames = map[string]string{
	"01": "Minted",
	"02": "Transfer",
	"03": "Migrated",
	"04": "Pledged",
	"05": "Generation",
	"06": "Unpledged",
	"07": "Committed",
	"08": "Burnt",
	"09": "Deployed",
	"10": "Executed",
	"11": "ContractCommitted",
	"12": "PinnedAsService",
	"13": "BurntForFT",
}

func GetAssetType(id string) (string, error) {
	var asset models.TokenType // assuming this matches your table structure

	// Fetch asset by ID
	result := database.DB.Where("token_id= ?", id).First(&asset)
	if result.Error != nil {
		return "", fmt.Errorf("failed to fetch asset type: %w", result.Error)
	}

	if asset.TokenType == "" {
		return "", errors.New("asset type not found")
	}

	return asset.TokenType, nil
}

// GetTokenChainFromTokenID fetches the complete token chain from the full node for a given tokenID
func GetTokenChainFromTokenID(tokenID string) (map[string]interface{}, error) {
	// Step 1: Get token type from database
	tokenType, err := GetAssetType(tokenID)
	if err != nil {
		return nil, fmt.Errorf("❌ failed to get token type from asset table: %v", err)
	}

	// Step 2: Build full node API URL
	apiURL := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
		config.RubixNodeURL, tokenID, tokenType)

	// Step 3: Call the API
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("❌ error fetching token chain for %s: %v", tokenID, err)
	}
	defer resp.Body.Close()

	// Step 4: Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("❌ error reading response for %s: %v", tokenID, err)
	}

	// Step 5: Decode JSON
	var chainData map[string]interface{}
	if err := json.Unmarshal(body, &chainData); err != nil {
		return nil, fmt.Errorf("❌ error decoding JSON for %s: %v", tokenID, err)
	}

	// Step 6: Return the full token chain response
	return chainData, nil
}

// Fetches all blocks from a given token chain
func GetTokenBlocksFromTokenID(tokenID string) ([]map[string]interface{}, error) {
	// Step 1: Get token type
	tokenType, err := GetAssetType(tokenID)
	if err != nil {
		return nil, fmt.Errorf("❌ failed to get token type from asset table: %v", err)
	}

	// Step 2: Build API URL
	apiURL := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
		config.RubixNodeURL, tokenID, tokenType)

	// Step 3: Fetch data
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("❌ error fetching token chain for %s: %v", tokenID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("❌ error reading response for %s: %v", tokenID, err)
	}

	// Step 4: Decode JSON
	var chainData map[string]interface{}
	if err := json.Unmarshal(body, &chainData); err != nil {
		return nil, fmt.Errorf("❌ error decoding JSON for %s: %v", tokenID, err)
	}

	tokenChainData, ok := chainData["TokenChainData"].([]interface{})
	if !ok || len(tokenChainData) == 0 {
		return nil, fmt.Errorf("❌ TokenChainData not found or empty for %s", tokenID)
	}

	var allBlocks []map[string]interface{}

	for i, blk := range tokenChainData {
		block, ok := blk.(map[string]interface{})
		if !ok {
			fmt.Printf("⚠️ Skipping invalid block at index %d for %s\n", i, tokenID)
			continue
		}

		blockData := make(map[string]interface{})

		// Extract core fields (numeric or string keys)
		blockHash := getValue(block, "98", "TCBlockHashKey")
		owner := getValue(block, "3", "TCTokenOwnerKey")
		epoch := getValue(block, "epoch", "TCEpoch")
		transType := getValue(block, "2", "TCTransTypeKey")
		transInfo := getMap(block, "5", "TCTransInfoKey")

		if blockHash != nil {
			blockData["block_hash"] = blockHash
		}
		if owner != nil {
			blockData["owner_did"] = owner
		}
		if epoch != nil {
			blockData["epoch"] = epoch
		}

		// Decode transaction type to readable form
		if transType != nil {
			if transTypeStr, ok := transType.(string); ok {
				if name, found := transactionTypeNames[transTypeStr]; found {
					blockData["transaction_type"] = name
				}
			}
		}

		// Extract TID from TransInfo if available
		if len(transInfo) > 0 {
			tid := getValue(transInfo, "4", "TITIDKey")
			if tid != nil {
				blockData["transaction_id"] = tid
			}
		}

		allBlocks = append(allBlocks, blockData)
	}

	if len(allBlocks) == 0 {
		return nil, fmt.Errorf("❌ no valid token chain blocks found for %s", tokenID)
	}

	return allBlocks, nil
}

// Helper: safely get value using either numeric or string key
func getValue(m map[string]interface{}, numKey, strKey string) interface{} {
	if val, ok := m[numKey]; ok {
		return val
	}
	if val, ok := m[strKey]; ok {
		return val
	}
	return nil
}

// Helper: safely get map using either numeric or string key
func getMap(m map[string]interface{}, numKey, strKey string) map[string]interface{} {
	if val, ok := m[numKey]; ok {
		if subMap, ok := val.(map[string]interface{}); ok {
			return subMap
		}
	}
	if val, ok := m[strKey]; ok {
		if subMap, ok := val.(map[string]interface{}); ok {
			return subMap
		}
	}
	return map[string]interface{}{}
}

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
	"strings"
)

// Transaction type names for token-chain display
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

// -------------------------------------------------------------------
// GetAssetType (from TokenType table)
// -------------------------------------------------------------------
func GetAssetType(id string) (string, error) {
	var entry models.TokenType

	if err := database.DB.Where("token_id = ?", id).First(&entry).Error; err != nil {
		return "", fmt.Errorf("failed to fetch asset type: %w", err)
	}
	if entry.TokenType == "" {
		return "", errors.New("asset type not found")
	}
	return entry.TokenType, nil
}

// -------------------------------------------------------------------
// Fetch FULL token-chain for UI
// -------------------------------------------------------------------
func GetTokenChainFromTokenID(tokenID string) (map[string]interface{}, error) {

	tokenType, err := GetAssetType(tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token type: %v", err)
	}

	// RBT → PART if fractional
	if strings.ToUpper(tokenType) == "RBT" {
		rbt, err := GetRBTInfoFromRBTID(tokenID)
		if err != nil {
			return nil, fmt.Errorf("failed to get RBT: %v", err)
		}
		if rbt.TokenValue < 1.0 {
			tokenType = "PART"
		}
	}

	url := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
		config.RubixNodeURL, tokenID, tokenType)

	client := GetNodeHTTPClient()
	release := acquireNodeSlot()
	defer release()

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fullnode error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("fullnode returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("json decode error: %v", err)
	}

	return out, nil
}

// -------------------------------------------------------------------
// Paginated blocks-from-token-chain
// -------------------------------------------------------------------
func GetTokenBlocksFromTokenID(tokenID string, page, limit int) ([]map[string]interface{}, int, error) {

	tokenType, err := GetAssetType(tokenID)
	if err != nil {
		return nil, 0, err
	}

	if strings.ToUpper(tokenType) == "RBT" {
		rbt, err := GetRBTInfoFromRBTID(tokenID)
		if err != nil {
			return nil, 0, err
		}
		if rbt.TokenValue < 1.0 {
			tokenType = "PART"
		}
	}

	url := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
		config.RubixNodeURL, tokenID, tokenType)

	client := GetNodeHTTPClient()
	release := acquireNodeSlot()
	defer release()

	resp, err := client.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, 0, fmt.Errorf("fullnode error %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var chain map[string]interface{}
	if err := json.Unmarshal(body, &chain); err != nil {
		return nil, 0, err
	}

	arr, ok := chain["TokenChainData"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("TokenChainData missing")
	}

	var blocks []map[string]interface{}
	for _, b := range arr {
		m, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		entry := map[string]interface{}{}

		entry["block_hash"] = getVal(m, "98", "TCBlockHashKey")
		entry["owner_did"] = getVal(m, "3", "TCTokenOwnerKey")
		entry["epoch"] = getVal(m, "epoch", "TCEpoch")

		tx := getVal(m, "2", "TCTransTypeKey")
		if s, ok := tx.(string); ok {
			if n, ok2 := transactionTypeNames[s]; ok2 {
				entry["transaction_type"] = n
			}
		}

		ti := getMap(m, "5", "TCTransInfoKey")
		if len(ti) > 0 {
			entry["transaction_id"] = getVal(ti, "4", "TITIDKey")
		}

		blocks = append(blocks, entry)
	}

	total := len(blocks)
	if total == 0 {
		return nil, 0, fmt.Errorf("no blocks")
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	start := (page - 1) * limit
	if start >= total {
		return []map[string]interface{}{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}

	return blocks[start:end], total, nil
}

// -------------------------------------------------------------------
// Helpers for chain parsing
// -------------------------------------------------------------------
func getVal(m map[string]interface{}, nk, sk string) interface{} {
	if v, ok := m[nk]; ok {
		return v
	}
	if v, ok := m[sk]; ok {
		return v
	}
	return nil
}

func getMap(m map[string]interface{}, nk, sk string) map[string]interface{} {
	if v, ok := m[nk].(map[string]interface{}); ok {
		return v
	}
	if v, ok := m[sk].(map[string]interface{}); ok {
		return v
	}
	return map[string]interface{}{}
}

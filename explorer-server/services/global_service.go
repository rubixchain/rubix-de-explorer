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

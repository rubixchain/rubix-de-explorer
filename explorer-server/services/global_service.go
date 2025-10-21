package services

import (
	"errors"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
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

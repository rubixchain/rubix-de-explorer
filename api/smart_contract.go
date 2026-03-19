package api

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

func GetSCCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 4).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCInfoFromSCID(scID string) (*models.Token, error) {
	var token models.Token
	if err := database.ReadDB.Where("token_id = ? AND token_type = ?", scID, 4).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func GetSCList(limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit

	// Fetch paginated Smart Contracts (TokenType = 4)
	if err := database.ReadDB.Model(&models.Token{}).
		Where("token_type = ?", 4).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tokens).Error; err != nil {
		return nil, err
	}

	return tokens, nil
}


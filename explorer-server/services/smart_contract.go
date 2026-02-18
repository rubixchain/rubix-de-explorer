package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
)

func GetSCCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.SC{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCInfoFromSCID(scID string) (*models.SC, error) {
	var scInfo models.SC
	if err := database.DB.First(&scInfo, "token_id = ?", scID).Error; err != nil {
		return nil, err
	}
	return &scInfo, nil
}

func GetSCBlockList(limit, page int) (interface{}, error) {
	var blocks []models.SCBlocks

	offset := (page - 1) * limit

	// Fetch all blocks with pagination
	if err := database.DB.
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&blocks).Error; err != nil {
		return nil, err
	}

	var count int64
	if err := database.DB.Model(&models.SCBlocks{}).Count(&count).Error; err != nil {
		return model.SCBlocksListResponse{}, err
	}

	// Wrap in response struct
	response := model.SCBlocksListResponse{
		SC_Blocks: blocks,
		Count:     count,
	}

	return response, nil
}

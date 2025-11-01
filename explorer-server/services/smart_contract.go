package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
)

func GetSCCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.SmartContract{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCInfoFromSCID(scID string) (*models.SmartContract, error) {
	var scInfo models.SmartContract
	if err := database.DB.First(&scInfo, "contract_id = ?", scID).Error; err != nil {
		return nil, err
	}
	return &scInfo, nil
}

func GetSCBlockList(limit, page int) (interface{}, error) {
	var blocks []models.SC_Block

	// offset := (page - 1) * limit

	// Fetch all blocks with pagination
	if err := database.DB.
		// Limit(int(limit)).
		// Offset(int(offset)).
		Find(&blocks).Error; err != nil {
		return nil, err
	}

	// Wrap in response struct
	response := model.SCBlocksListResponse{
		SC_Blocks: blocks,
	}

	return response, nil
}

package api

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
)

func GetSCCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.SC{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetSCInfoFromSCID(scID string) (*models.SC, error) {
	var scInfo models.SC
	if err := database.ReadDB.First(&scInfo, "token_id = ?", scID).Error; err != nil {
		return nil, err
	}
	return &scInfo, nil
}

func GetSCList(limit, page int) (model.SCListResponse, error) {
	var scs []models.SC

	offset := (page - 1) * limit

	if err := database.ReadDB.
		Limit(limit).
		Offset(offset).
		Find(&scs).Error; err != nil {
		return model.SCListResponse{}, err
	}

	var count int64
	if err := database.ReadDB.Model(&models.SC{}).Count(&count).Error; err != nil {
		return model.SCListResponse{}, err
	}

	return model.SCListResponse{SCs: scs, Count: count}, nil
}

func GetSCBlockList(limit, page int) (interface{}, error) {
	var blocks []models.SCBlocks

	offset := (page - 1) * limit

	// Fetch all blocks with pagination
	if err := database.ReadDB.
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&blocks).Error; err != nil {
		return nil, err
	}

	var count int64
	if err := database.ReadDB.Model(&models.SCBlocks{}).Count(&count).Error; err != nil {
		return model.SCBlocksListResponse{}, err
	}

	// Wrap in response struct
	response := model.SCBlocksListResponse{
		SC_Blocks: blocks,
		Count:     count,
	}

	return response, nil
}

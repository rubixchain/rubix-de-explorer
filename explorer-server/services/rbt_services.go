package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
)

// GetRBTCount returns the total number of RBTs in the database
func GetRBTCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.RBT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
	var rbt models.RBT
	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
		return nil, err
	}
	return &rbt, nil
}

func GetRBTList(limit, page int) (interface{}, error) {
	var rbtModels []models.RBT
	offset := (page - 1) * limit

	// Fetch paginated RBTs
	if err := database.DB.
		Limit(limit).
		Offset(offset).
		Find(&rbtModels).Error; err != nil {
		return nil, err
	}

	// Map to response Tokens
	tokens := make([]model.Token, len(rbtModels))
	for i, r := range rbtModels {
		tokens[i] = model.Token{
			TokenId:    r.TokenID,
			OwnerDID:   r.OwnerDID,
			TokenValue: r.TokenValue,
		}
	}

	// Get total count of RBTs
	var count int64
	if err := database.DB.Model(&models.RBT{}).Count(&count).Error; err != nil {
		return nil, err
	}

	// Wrap in response
	response := model.RBTListResponse{
		Tokens: tokens,
		Count:  count,
	}

	return response, nil
}


func GetRBTListFromDID(did string, limit, page int) ([]models.RBT, int64, error) {
	var rbts []models.RBT
	var totalCount int64

	// Count total records for the DID with TokenStatus = 0
	if err := database.DB.Model(&models.RBT{}).
		Where("owner_did = ? AND token_status = ?", did, 0).
		Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch only TokenStatus = 0
	offset := (page - 1) * limit

	if err := database.DB.
		Where("owner_did = ? AND token_status = ?", did, 0).
		Limit(limit).
		Offset(offset).
		Find(&rbts).Error; err != nil {
		return nil, 0, err
	}

	return rbts, totalCount, nil
}

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetRBTCount returns the total number of RBTs in the database
func GetFTCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.FT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetFTInfoFromFTID(ftID string) (*models.FT, error) {
	var ftInfo models.FT
	if err := database.DB.First(&ftInfo, "ft_id = ?", ftID).Error; err != nil {
		return nil, err
	}
	return &ftInfo, nil
}
func GetFTListFromDID(did string) ([]models.FT, error) {
	var ftList []models.FT

	// Fetch all FTs where owner_did = given DID
	if err := database.DB.
		Where("owner_did = ?", did).
		Find(&ftList).Error; err != nil {
		return nil, err
	}

	return ftList, nil
}
// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

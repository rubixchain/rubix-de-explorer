package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetRBTCount returns the total number of RBTs in the database
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

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

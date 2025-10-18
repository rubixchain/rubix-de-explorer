package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetRBTCount returns the total number of RBTs in the database
func GetNFTCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.NFT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetNFTInfoFromNFTID(nftID string) (*models.NFT, error) {
	var nftInfo models.NFT
	if err := database.DB.First(&nftInfo, "token_id = ?", nftID).Error; err != nil {
		return nil, err
	}
	return &nftInfo, nil
}

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

package api

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetNFTCount returns the total number of NFTs in the database
func GetNFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.NFT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetNFTInfoFromNFTID fetches a single NFT by its ID
func GetNFTInfoFromNFTID(nftID string) (*models.NFT, error) {
	var nftInfo models.NFT
	if err := database.ReadDB.First(&nftInfo, "token_id = ?", nftID).Error; err != nil {
		return nil, err
	}
	return &nftInfo, nil
}

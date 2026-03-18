package api

import (
	"explorer-server/database"
	"explorer-server/database/models"
)

// GetRBTCount returns the total number of RBTs in the database
func GetDIDCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.DIDs{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetDIDInfoFromDID(did string) (*models.DIDs, error) {
	var didInfo models.DIDs
	if err := database.ReadDB.First(&didInfo, "did = ?", did).Error; err != nil {
		return nil, err
	}
	return &didInfo, nil
}

func GetDIDHoldersList(limit, page int) ([]models.DIDBalance, error) {
	var balances []models.DIDBalance

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Fetch top RBT holders from DIDBalances table
	if err := database.ReadDB.
		Table("DIDBalances").
		Where("asset_type = ? AND balance > 0", "RBT").
		Order("balance DESC").
		Limit(limit).
		Offset(offset).
		Find(&balances).Error; err != nil {
		return nil, err
	}

	return balances, nil
}

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.ReadDB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

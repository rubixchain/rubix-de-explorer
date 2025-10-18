
package services

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
)

// GetRBTCount returns the total number of RBTs in the database
func GetDIDCount() (int64, error) {
	var count int64
	if err := database.DB.Model(&models.DIDs{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetDIDInfoFromDID(did string) (*models.DIDs, error) {
	var didInfo models.DIDs
	if err := database.DB.First(&didInfo, "did = ?", did).Error; err != nil {
		return nil, err
	}
	return &didInfo, nil
}

func GetDIDHoldersList(limit, page int) ([]model.HolderResponse, error) {
	var dids []models.DIDs
	offset := (page - 1) * limit

	// Fetch paginated DIDs ordered by TotalRBTs descending
	if err := database.DB.Order("total_rbts desc").
		Limit(limit).
		Offset(offset).
		Find(&dids).Error; err != nil {
		return nil, err
	}

	// Map to response format
	holders := make([]model.HolderResponse, len(dids))
	for i, d := range dids {
		holders[i] = model.HolderResponse{
			OwnerDID:  d.DID,
			TokenCount: d.TotalRBTs,
		}
	}

	return holders, nil
}

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
// func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
// 	var rbt models.RBT
// 	if err := database.DB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
// 		return nil, err
// 	}
// 	return &rbt, nil
// }

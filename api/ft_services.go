package api

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"strings"
)

// GetRBTCount returns the total number of RBTs in the database
func GetFTCount() (int64, error) {
	var count int64
	if err := database.ReadDB.Model(&models.FT{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func GetFTInfoFromFTID(ftID string) (*models.FT, error) {
	var ftInfo models.FT
	if err := database.ReadDB.First(&ftInfo, "token_id = ?", ftID).Error; err != nil {
		return nil, err
	}
	return &ftInfo, nil
}

func GetFTGroupList(limit, page int) ([]model.FTGroup, error) {
	var tokens []models.Token
	// We need to fetch all FTs to group them properly.
	// In a real production scenario with millions of FTs, this would be slow
	// and we should ideally have a separate table for FT groups/classes.
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", 2).Find(&tokens).Error; err != nil {
		return nil, err
	}

	groupsMap := make(map[string]*model.FTGroup)
	for _, t := range tokens {
		parts := strings.Split(t.TokenID, "_")
		if len(parts) >= 3 {
			ftName := parts[0]
			creatorDID := parts[len(parts)-1]
			key := ftName + "_" + creatorDID
			if g, ok := groupsMap[key]; ok {
				g.Count++
			} else {
				groupsMap[key] = &model.FTGroup{
					FTName:     ftName,
					Count:      1,
					CreatorDID: creatorDID,
				}
			}
		}
	}

	var allGroups []model.FTGroup
	for _, g := range groupsMap {
		allGroups = append(allGroups, *g)
	}

	start := (page - 1) * limit
	if start > len(allGroups) {
		start = len(allGroups)
	}
	end := start + limit
	if end > len(allGroups) {
		end = len(allGroups)
	}

	paginatedGroups := allGroups[start:end]

	return paginatedGroups, nil
}

func GetFTListByFTName(ftName string, creatorDID string, limit, page int) ([]models.Token, error) {
	var tokens []models.Token
	offset := (page - 1) * limit

	// FT TokenID format: {Name}_{Index}_{CreatorDID}
	// We filter by name prefix and creatorDID suffix
	query := database.ReadDB.Model(&models.Token{}).
		Where("token_type = ?", 2).
		Where("token_id LIKE ?", ftName+"_%")

	if creatorDID != "" {
		query = query.Where("token_id LIKE ?", "%_"+creatorDID)
	}

	if err := query.
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tokens).Error; err != nil {
		return nil, err
	}

	return tokens, nil
}

// // GetRBTInfoFromRBTID fetches a single RBT by its ID
//
//	func GetRBTInfoFromRBTID(rbtID string) (*models.RBT, error) {
//		var rbt models.RBT
//		if err := database.ReadDB.First(&rbt, "rbt_id = ?", rbtID).Error; err != nil {
//			return nil, err
//		}
//		return &rbt, nil
//	}

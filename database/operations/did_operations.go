package operations

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"

	"gorm.io/gorm/clause"
)

// SaveDIDInfo updates DID meta-information (PeerID, DIDAlgo) in the DIDBalances table.
// 1. Updates PeerID/DIDAlgo on all existing asset rows for this DID.
// 2. Upserts a base entry (empty AssetType) so the DID is searchable even before tokens arrive.
func SaveDIDInfo(pm *model.DIDInfo) error {
	// Update all existing asset rows for this DID
	database.WriteDB.Model(&models.DIDBalance{}).
		Where("did = ?", pm.DID).
		Updates(map[string]interface{}{
			"peer_id":  pm.PeerID,
			"did_algo": pm.DIDAlgo,
		})

	// Upsert a base entry with empty AssetType (balance stays 0)
	base := models.DIDBalance{
		DID:     pm.DID,
		PeerID:  pm.PeerID,
		DIDAlgo: pm.DIDAlgo,
	}
	return database.WriteDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "did"}, {Name: "asset_type"}, {Name: "token_name"}, {Name: "creator_did"}},
		DoUpdates: clause.AssignmentColumns([]string{"peer_id", "did_algo"}),
	}).Create(&base).Error
}

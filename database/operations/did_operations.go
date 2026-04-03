package operations

import (
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"

	"gorm.io/gorm/clause"
)

// SaveDIDInfo updates DID meta-information (PeerID, DIDAlgo) in the DIDBalances table.
// It updates all existing assets for the DID and ensures a base mapping entry exists.
func SaveDIDInfo(pm *model.DIDInfo) error {
	// 1. Update all existing asset rows for this DID with the PeerID and DIDAlgo
	err := database.WriteDB.Model(&models.DIDBalance{}).
		Where("did = ?", pm.DID).
		Updates(map[string]interface{}{
			"peer_id":  pm.PeerID,
			"did_algo": pm.DIDAlgo,
		}).Error
	if err != nil {
		return err
	}

	// 2. Ensure a "base" DID mapping entry exists (AssetType: "DID_INFO")
	// This ensures we save the mapping even if no other assets exist yet.
	baseBalance := models.DIDBalance{
		DID:       pm.DID,
		AssetType: "DID_INFO",
		PeerID:    pm.PeerID,
		DIDAlgo:   pm.DIDAlgo,
	}

	// Upsert the base entry
	return database.WriteDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "did"}, {Name: "asset_type"}, {Name: "token_name"}, {Name: "creator_did"}},
		DoUpdates: clause.AssignmentColumns([]string{"peer_id", "did_algo"}),
	}).Create(&baseBalance).Error
}

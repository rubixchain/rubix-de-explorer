package services

import (
	"encoding/json"
	"errors"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

// UpdateTokens routes token updates to the appropriate handler
func UpdateTokens(tableName string, tokenData interface{}) {
	switch tableName {
	case "FullnodeRBTtable":
		log.Println("Processing RBT token update")
		UpdateRBTToken(tokenData)

	case "FullnodeFTtable":
		log.Println("Processing FT token update")
		UpdateFTToken(tokenData)

	case "FullnodeNFTtable":
		log.Println("Processing NFT token update")
		UpdateNFTToken(tokenData)

	case "FullnodeSCtable":
		log.Println("Processing SmartContract update")
		UpdateSCToken(tokenData)

	default:
		log.Printf("⚠️ Unknown token table: %s\n", tableName)
	}
}

// UpdateRBTToken updates RBT token or creates it if not exists, updates token_type and DID tables
func UpdateRBTToken(tokenData interface{}) error {
	var rbt RBT
	jsonBytes, err := json.Marshal(tokenData)
	if err != nil {
		log.Printf("❌ Failed to marshal RBT data: %v", err)
		return err
	}

	if err := json.Unmarshal(jsonBytes, &rbt); err != nil {
		log.Printf("❌ Failed to unmarshal RBT data: %v", err)
		return err
	}

	updateData := models.RBT{
		TokenID:     rbt.TokenID,
		TokenValue:  rbt.TokenValue,
		OwnerDID:    rbt.OwnerDID,
		BlockID:     rbt.BlockHash,
		BlockHeight: fmt.Sprintf("%d", rbt.BlockHeight),
		TokenStatus: rbt.TokenStatus,
	}

	var existingRBT models.RBT
	result := database.DB.Where("token_id = ?", rbt.TokenID).First(&existingRBT)

	isNewToken := errors.Is(result.Error, gorm.ErrRecordNotFound)

	// Upsert RBT token
	if isNewToken {
		if err := database.DB.Create(&updateData).Error; err != nil {
			log.Printf("❌ Failed to create RBT %s: %v", rbt.TokenID, err)
			return err
		}
		log.Printf("✅ RBT token created: %s", rbt.TokenID)
	} else if result.Error != nil {
		log.Printf("❌ Error querying RBT %s: %v", rbt.TokenID, result.Error)
		return result.Error
	} else {
		if err := database.DB.Model(&existingRBT).Updates(updateData).Error; err != nil {
			log.Printf("❌ Failed to update RBT %s: %v", rbt.TokenID, err)
			return err
		}
		log.Printf("✅ RBT token updated: %s", rbt.TokenID)
	}

	// Ensure token_type entry exists
	tokenType := models.TokenType{
		TokenID:     rbt.TokenID,
		TokenType:   "RBT",
		LastUpdated: time.Now(),
	}
	if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: rbt.TokenID}).Error; err != nil {
		log.Printf("⚠️ Failed to ensure token_type for %s: %v", rbt.TokenID, err)
	}

	// Update DID table if token is free (TokenStatus == 0)
	if rbt.TokenStatus == 0 {
		if err := updateDIDForRBT(rbt.OwnerDID, rbt.TokenValue, isNewToken); err != nil {
			log.Printf("⚠️ Failed to update DID %s: %v", rbt.OwnerDID, err)
		}
	}

	return nil
}

// UpdateFTToken updates FT token or creates it if not exists, updates token_type and DID tables
func UpdateFTToken(tokenData interface{}) error {
	var ft FT
	jsonBytes, err := json.Marshal(tokenData)
	if err != nil {
		log.Printf("❌ Failed to marshal FT data: %v", err)
		return err
	}

	if err := json.Unmarshal(jsonBytes, &ft); err != nil {
		log.Printf("❌ Failed to unmarshal FT data: %v", err)
		return err
	}

	updateData := models.FT{
		FtID:        ft.TokenID,
		TokenValue:  ft.TokenValue,
		FTName:      ft.FTName,
		OwnerDID:    ft.OwnerDID,
		CreatorDID:  ft.CreatorDID,
		BlockID:     ft.BlockHash,
		Txn_ID:      ft.TransactionID,
		TokenStatus: ft.TokenStatus,
	}

	var existingFT models.FT
	result := database.DB.Where("ft_id = ?", ft.TokenID).First(&existingFT)

	isNewToken := errors.Is(result.Error, gorm.ErrRecordNotFound)

	if isNewToken {
		if err := database.DB.Create(&updateData).Error; err != nil {
			log.Printf("❌ Failed to create FT %s: %v", ft.TokenID, err)
			return err
		}
		log.Printf("✅ FT token created: %s", ft.TokenID)
	} else if result.Error != nil {
		log.Printf("❌ Error querying FT %s: %v", ft.TokenID, result.Error)
		return result.Error
	} else {
		if err := database.DB.Model(&existingFT).Updates(updateData).Error; err != nil {
			log.Printf("❌ Failed to update FT %s: %v", ft.TokenID, err)
			return err
		}
		log.Printf("✅ FT token updated: %s", ft.TokenID)
	}

	// Ensure token_type entry exists
	tokenType := models.TokenType{
		TokenID:     ft.TokenID,
		TokenType:   "FT",
		LastUpdated: time.Now(),
	}
	if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: ft.TokenID}).Error; err != nil {
		log.Printf("⚠️ Failed to ensure token_type for %s: %v", ft.TokenID, err)
	}

	// Update DID table if token is free (TokenStatus == 0)
	if ft.TokenStatus == 0 {
		if err := updateDIDForFT(ft.OwnerDID, isNewToken); err != nil {
			log.Printf("⚠️ Failed to update DID %s: %v", ft.OwnerDID, err)
		}
	}

	return nil
}

// UpdateNFTToken updates NFT token or creates it if not exists, updates token_type and DID tables
func UpdateNFTToken(tokenData interface{}) error {
	var nft NFT
	jsonBytes, err := json.Marshal(tokenData)
	if err != nil {
		log.Printf("❌ Failed to marshal NFT data: %v", err)
		return err
	}

	if err := json.Unmarshal(jsonBytes, &nft); err != nil {
		log.Printf("❌ Failed to unmarshal NFT data: %v", err)
		return err
	}

	updateData := models.NFT{
		TokenID:     nft.TokenID,
		TokenValue:  fmt.Sprintf("%f", nft.TokenValue),
		OwnerDID:    nft.OwnerDID,
		BlockHash:   nft.BlockHash,
		Txn_ID:      nft.TransactionID,
		BlockHeight: nft.BlockHeight,
		TokenStatus: nft.TokenStatus,
	}

	var existingNFT models.NFT
	result := database.DB.Where("nft_id = ?", nft.TokenID).First(&existingNFT)

	isNewToken := errors.Is(result.Error, gorm.ErrRecordNotFound)

	if isNewToken {
		if err := database.DB.Create(&updateData).Error; err != nil {
			log.Printf("❌ Failed to create NFT %s: %v", nft.TokenID, err)
			return err
		}
		log.Printf("✅ NFT token created: %s", nft.TokenID)
	} else if result.Error != nil {
		log.Printf("❌ Error querying NFT %s: %v", nft.TokenID, result.Error)
		return result.Error
	} else {
		if err := database.DB.Model(&existingNFT).Updates(updateData).Error; err != nil {
			log.Printf("❌ Failed to update NFT %s: %v", nft.TokenID, err)
			return err
		}
		log.Printf("✅ NFT token updated: %s", nft.TokenID)
	}

	// Ensure token_type entry exists
	tokenType := models.TokenType{
		TokenID:     nft.TokenID,
		TokenType:   "NFT",
		LastUpdated: time.Now(),
	}
	if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: nft.TokenID}).Error; err != nil {
		log.Printf("⚠️ Failed to ensure token_type for %s: %v", nft.TokenID, err)
	}

	// Update DID table for all NFTs (no TokenStatus check for NFTs)
	if err := updateDIDForNFT(nft.OwnerDID, isNewToken); err != nil {
		log.Printf("⚠️ Failed to update DID %s: %v", nft.OwnerDID, err)
	}

	return nil
}

// UpdateSCToken updates Smart Contract or creates it if not exists, updates token_type and DID tables
func UpdateSCToken(tokenData interface{}) error {
	var sc SC
	jsonBytes, err := json.Marshal(tokenData)
	if err != nil {
		log.Printf("❌ Failed to marshal SC data: %v", err)
		return err
	}

	if err := json.Unmarshal(jsonBytes, &sc); err != nil {
		log.Printf("❌ Failed to unmarshal SC data: %v", err)
		return err
	}

	updateData := models.SmartContract{
		ContractID:  sc.SmartContractHash,
		BlockHash:   sc.BlockHash,
		DeployerDID: sc.Deployer,
		TxnId:       sc.TransactionID,
		BlockHeight: sc.BlockHeight,
		TokenStatus: sc.TokenStatus,
	}

	var existingSC models.SmartContract
	result := database.DB.Where("contract_id = ?", sc.SmartContractHash).First(&existingSC)

	isNewToken := errors.Is(result.Error, gorm.ErrRecordNotFound)

	if isNewToken {
		if err := database.DB.Create(&updateData).Error; err != nil {
			log.Printf("❌ Failed to create SC %s: %v", sc.SmartContractHash, err)
			return err
		}
		log.Printf("✅ Smart Contract created: %s", sc.SmartContractHash)
	} else if result.Error != nil {
		log.Printf("❌ Error querying SC %s: %v", sc.SmartContractHash, result.Error)
		return result.Error
	} else {
		if err := database.DB.Model(&existingSC).Updates(updateData).Error; err != nil {
			log.Printf("❌ Failed to update SC %s: %v", sc.SmartContractHash, err)
			return err
		}
		log.Printf("✅ Smart Contract updated: %s", sc.SmartContractHash)
	}

	// Ensure token_type entry exists
	tokenType := models.TokenType{
		TokenID:     sc.SmartContractHash,
		TokenType:   "SC",
		LastUpdated: time.Now(),
	}
	if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: sc.SmartContractHash}).Error; err != nil {
		log.Printf("⚠️ Failed to ensure token_type for SC %s: %v", sc.SmartContractHash, err)
	}

	// Update DID table for smart contracts
	if err := updateDIDForSC(sc.Deployer, isNewToken); err != nil {
		log.Printf("⚠️ Failed to update DID %s: %v", sc.Deployer, err)
	}

	return nil
}

// Helper function to update DID table for RBT tokens
func updateDIDForRBT(ownerDID string, tokenValue float64, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new DID entry
		newDID := models.DIDs{
			DID:       ownerDID,
			CreatedAt: time.Now(),
			TotalRBTs: tokenValue,
		}
		if err := database.DB.Create(&newDID).Error; err != nil {
			return err
		}
		log.Printf("✅ Created DID entry for %s with RBT value: %f", ownerDID, tokenValue)
	} else if err != nil {
		return err
	} else {
		// Update existing DID entry
		if isNewToken {
			existing.TotalRBTs += tokenValue
		}
		existing.TotalRBTs = math.Round(existing.TotalRBTs*1000) / 1000
		if err := database.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("✅ Updated DID entry for %s, new total RBTs: %f", ownerDID, existing.TotalRBTs)
	}

	return nil
}

// Helper function to update DID table for FT tokens
func updateDIDForFT(ownerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new DID entry
		newDID := models.DIDs{
			DID:       ownerDID,
			CreatedAt: time.Now(),
			TotalFTs:  1,
		}
		if err := database.DB.Create(&newDID).Error; err != nil {
			return err
		}
		log.Printf("✅ Created DID entry for %s with FT count: 1", ownerDID)
	} else if err != nil {
		return err
	} else {
		// Update existing DID entry
		if isNewToken {
			existing.TotalFTs += 1
		}
		if err := database.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("✅ Updated DID entry for %s, new total FTs: %f", ownerDID, existing.TotalFTs)
	}

	return nil
}

// Helper function to update DID table for NFT tokens
func updateDIDForNFT(ownerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new DID entry
		newDID := models.DIDs{
			DID:       ownerDID,
			CreatedAt: time.Now(),
			TotalNFTs: 1,
		}
		if err := database.DB.Create(&newDID).Error; err != nil {
			return err
		}
		log.Printf("✅ Created DID entry for %s with NFT count: 1", ownerDID)
	} else if err != nil {
		return err
	} else {
		// Update existing DID entry
		if isNewToken {
			existing.TotalNFTs += 1
		}
		if err := database.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("✅ Updated DID entry for %s, new total NFTs: %d", ownerDID, existing.TotalNFTs)
	}

	return nil
}

// Helper function to update DID table for Smart Contracts
func updateDIDForSC(deployerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", deployerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new DID entry
		newDID := models.DIDs{
			DID:       deployerDID,
			CreatedAt: time.Now(),
			TotalSC:   1,
		}
		if err := database.DB.Create(&newDID).Error; err != nil {
			return err
		}
		log.Printf("✅ Created DID entry for %s with SC count: 1", deployerDID)
	} else if err != nil {
		return err
	} else {
		// Update existing DID entry
		if isNewToken {
			existing.TotalSC += 1
		}
		if err := database.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("✅ Updated DID entry for %s, new total SCs: %d", deployerDID, existing.TotalSC)
	}

	return nil
}

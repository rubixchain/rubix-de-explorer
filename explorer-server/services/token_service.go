package services

import (
	"encoding/json"
	"errors"
	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/model"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

// UpdateTokens routes token updates to the appropriate handler
func UpdateTokens(tableName string, tokenData interface{}, operation string) {
	switch tableName {
	case "FullnodeRBTtable":
		log.Printf("Processing RBT token %s", operation)
		UpdateRBTToken(tokenData, operation)

	case "FullnodeFTtable":
		log.Printf("Processing FT token %s", operation)
		UpdateFTToken(tokenData, operation)

	case "FullnodeNFTtable":
		log.Printf("Processing NFT token %s", operation)
		UpdateNFTToken(tokenData, operation)

	case "FullnodeSCtable":
		log.Printf("Processing SmartContract %s", operation)
		UpdateSCToken(tokenData, operation)

	// case "FullnodeFailedToSyncTokens":
	// 	log.Printf("Processing Failed Token %s", operation)
	// 	UpdateFailedTokens(tokenData, operation)

	default:
		log.Printf("⚠️ Unknown token table: %s\n", tableName)
	}
}

// ========== RBT Token Operations ==========

// UpdateRBTToken handles CREATE and UPDATE operations for RBT tokens
func UpdateRBTToken(tokenData interface{}, operation string) error {
	if operation == "DELETE" {
		return deleteRBTToken(tokenData)
	}

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
	result := database.DB.Where("rbt_id = ?", rbt.TokenID).First(&existingRBT)

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
		if err := database.DB.Where("rbt_id = ?", rbt.TokenID).Updates(updateData).Error; err != nil {
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

// deleteRBTToken handles DELETE operation for RBT tokens
func deleteRBTToken(tokenData interface{}) error {
	deletePayload := tokenData.(map[string]interface{})
	tokenID := deletePayload["token_id"].(string)

	var rbt models.RBT
	result := database.DB.Where("rbt_id = ?", tokenID).First(&rbt)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ RBT token not found for deletion: %s", tokenID)
		return nil
	} else if result.Error != nil {
		log.Printf("❌ Error querying RBT %s: %v", tokenID, result.Error)
		return result.Error
	}

	// Delete the RBT token
	if err := database.DB.Delete(&rbt).Error; err != nil {
		log.Printf("❌ Failed to delete RBT %s: %v", tokenID, err)
		return err
	}

	// Update DID table - subtract the token value
	if err := decrementDIDForRBT(rbt.OwnerDID, rbt.TokenValue); err != nil {
		log.Printf("⚠️ Failed to decrement DID %s: %v", rbt.OwnerDID, err)
	}

	// Delete token_type entry
	if err := database.DB.Where("token_id = ?", tokenID).Delete(&models.TokenType{}).Error; err != nil {
		log.Printf("⚠️ Failed to delete token_type for %s: %v", tokenID, err)
	}

	log.Printf("✅ RBT token deleted: %s", tokenID)
	return nil
}

// ========== FT Token Operations ==========

// UpdateFTToken handles CREATE and UPDATE operations for FT tokens
func UpdateFTToken(tokenData interface{}, operation string) error {
	if operation == "DELETE" {
		return deleteFTToken(tokenData)
	}

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
		if err := database.DB.Where("ft_id = ?", ft.TokenID).Updates(updateData).Error; err != nil {
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

// deleteFTToken handles DELETE operation for FT tokens
func deleteFTToken(tokenData interface{}) error {
	deletePayload := tokenData.(map[string]interface{})
	tokenID := deletePayload["token_id"].(string)

	var ft models.FT
	result := database.DB.Where("ft_id = ?", tokenID).First(&ft)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ FT token not found for deletion: %s", tokenID)
		return nil
	} else if result.Error != nil {
		log.Printf("❌ Error querying FT %s: %v", tokenID, result.Error)
		return result.Error
	}

	// Delete the FT token
	if err := database.DB.Delete(&ft).Error; err != nil {
		log.Printf("❌ Failed to delete FT %s: %v", tokenID, err)
		return err
	}

	// Update DID table - decrement count
	if err := decrementDIDForFT(ft.OwnerDID); err != nil {
		log.Printf("⚠️ Failed to decrement DID %s: %v", ft.OwnerDID, err)
	}

	// Delete token_type entry
	if err := database.DB.Where("token_id = ?", tokenID).Delete(&models.TokenType{}).Error; err != nil {
		log.Printf("⚠️ Failed to delete token_type for %s: %v", tokenID, err)
	}

	log.Printf("✅ FT token deleted: %s", tokenID)
	return nil
}

// ========== NFT Token Operations ==========

// UpdateNFTToken handles CREATE and UPDATE operations for NFT tokens
func UpdateNFTToken(tokenData interface{}, operation string) error {
	if operation == "DELETE" {
		return deleteNFTToken(tokenData)
	}

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
		if err := database.DB.Where("nft_id = ?", nft.TokenID).Updates(updateData).Error; err != nil {
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

// deleteNFTToken handles DELETE operation for NFT tokens
func deleteNFTToken(tokenData interface{}) error {
	deletePayload := tokenData.(map[string]interface{})
	tokenID := deletePayload["token_id"].(string)

	var nft models.NFT
	result := database.DB.Where("nft_id = ?", tokenID).First(&nft)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ NFT token not found for deletion: %s", tokenID)
		return nil
	} else if result.Error != nil {
		log.Printf("❌ Error querying NFT %s: %v", tokenID, result.Error)
		return result.Error
	}

	// Delete the NFT token
	if err := database.DB.Delete(&nft).Error; err != nil {
		log.Printf("❌ Failed to delete NFT %s: %v", tokenID, err)
		return err
	}

	// Update DID table - decrement count
	if err := decrementDIDForNFT(nft.OwnerDID); err != nil {
		log.Printf("⚠️ Failed to decrement DID %s: %v", nft.OwnerDID, err)
	}

	// Delete token_type entry
	if err := database.DB.Where("token_id = ?", tokenID).Delete(&models.TokenType{}).Error; err != nil {
		log.Printf("⚠️ Failed to delete token_type for %s: %v", tokenID, err)
	}

	log.Printf("✅ NFT token deleted: %s", tokenID)
	return nil
}

// ========== Smart Contract Operations ==========

// UpdateSCToken handles CREATE and UPDATE operations for Smart Contracts
func UpdateSCToken(tokenData interface{}, operation string) error {
	if operation == "DELETE" {
		return deleteSCToken(tokenData)
	}

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
		if err := database.DB.Where("contract_id = ?", sc.SmartContractHash).Updates(updateData).Error; err != nil {
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

// deleteSCToken handles DELETE operation for Smart Contracts
func deleteSCToken(tokenData interface{}) error {
	deletePayload := tokenData.(map[string]interface{})
	contractHash := deletePayload["smart_contract_hash"].(string)

	var sc models.SmartContract
	result := database.DB.Where("contract_id = ?", contractHash).First(&sc)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ Smart Contract not found for deletion: %s", contractHash)
		return nil
	} else if result.Error != nil {
		log.Printf("❌ Error querying SC %s: %v", contractHash, result.Error)
		return result.Error
	}

	// Delete the Smart Contract
	if err := database.DB.Delete(&sc).Error; err != nil {
		log.Printf("❌ Failed to delete SC %s: %v", contractHash, err)
		return err
	}

	// Update DID table - decrement count
	if err := decrementDIDForSC(sc.DeployerDID); err != nil {
		log.Printf("⚠️ Failed to decrement DID %s: %v", sc.DeployerDID, err)
	}

	// Delete token_type entry
	if err := database.DB.Where("token_id = ?", contractHash).Delete(&models.TokenType{}).Error; err != nil {
		log.Printf("⚠️ Failed to delete token_type for SC %s: %v", contractHash, err)
	}

	log.Printf("✅ Smart Contract deleted: %s", contractHash)
	return nil
}

// ========== Failed Tokens Operations ==========

// UpdateFailedTokens handles CREATE and DELETE operations for Failed Tokens
func UpdateFailedTokens(tokenData interface{}, operation string) error {
	if operation == "DELETE" {
		return deleteFailedToken(tokenData)
	}

	// For CREATE operation
	var failedToken model.FailedToSyncTokenDetailsInfo
	jsonBytes, err := json.Marshal(tokenData)
	if err != nil {
		log.Printf("❌ Failed to marshal failed token data: %v", err)
		return err
	}

	if err := json.Unmarshal(jsonBytes, &failedToken); err != nil {
		log.Printf("❌ Failed to unmarshal failed token data: %v", err)
		return err
	}

	log.Printf("✅ Failed token recorded: %s", failedToken.TokenID)
	return nil
}

// deleteFailedToken handles DELETE operation for Failed Tokens
func deleteFailedToken(tokenData interface{}) error {
	deletePayload := tokenData.(map[string]interface{})
	tokenID := deletePayload["token_id"].(string)

	var failedToken model.FailedToSyncTokenDetailsInfo

	result := database.DB.Where("token_id = ?", tokenID).First(&failedToken)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ Failed token not found for deletion: %s", tokenID)
		return nil
	} else if result.Error != nil {
		log.Printf("❌ Error querying failed token %s: %v", tokenID, result.Error)
		return result.Error
	}

	// Delete the failed token entry
	if err := database.DB.Delete(&failedToken).Error; err != nil {
		log.Printf("❌ Failed to delete failed token %s: %v", tokenID, err)
		return err
	}

	log.Printf("✅ Failed token deleted: %s", tokenID)
	return nil
}

// ========== DID Update Helpers (Increment) ==========

func updateDIDForRBT(ownerDID string, tokenValue float64, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
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

func updateDIDForFT(ownerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
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
		if isNewToken {
			existing.TotalFTs += 1
		}
		if err := database.DB.Save(&existing).Error; err != nil {
			return err
		}
		log.Printf("✅ Updated DID entry for %s, new total FTs: %d", ownerDID, existing.TotalFTs)
	}

	return nil
}

func updateDIDForNFT(ownerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
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

func updateDIDForSC(deployerDID string, isNewToken bool) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", deployerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
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

// ========== DID Update Helpers (Decrement for Deletions) ==========

func decrementDIDForRBT(ownerDID string, tokenValue float64) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ DID entry not found for %s", ownerDID)
		return nil
	} else if err != nil {
		return err
	}

	existing.TotalRBTs -= tokenValue
	existing.TotalRBTs = math.Round(existing.TotalRBTs*1000) / 1000
	if existing.TotalRBTs < 0 {
		existing.TotalRBTs = 0
	}

	if err := database.DB.Save(&existing).Error; err != nil {
		return err
	}
	log.Printf("✅ Decremented DID entry for %s, new total RBTs: %f", ownerDID, existing.TotalRBTs)

	return nil
}

func decrementDIDForFT(ownerDID string) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ DID entry not found for %s", ownerDID)
		return nil
	} else if err != nil {
		return err
	}

	existing.TotalFTs -= 1
	if existing.TotalFTs < 0 {
		existing.TotalFTs = 0
	}

	if err := database.DB.Save(&existing).Error; err != nil {
		return err
	}
	log.Printf("✅ Decremented DID entry for %s, new total FTs: %d", ownerDID, existing.TotalFTs)

	return nil
}

func decrementDIDForNFT(ownerDID string) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", ownerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ DID entry not found for %s", ownerDID)
		return nil
	} else if err != nil {
		return err
	}

	existing.TotalNFTs -= 1
	if existing.TotalNFTs < 0 {
		existing.TotalNFTs = 0
	}

	if err := database.DB.Save(&existing).Error; err != nil {
		return err
	}
	log.Printf("✅ Decremented DID entry for %s, new total NFTs: %d", ownerDID, existing.TotalNFTs)

	return nil
}

func decrementDIDForSC(deployerDID string) error {
	var existing models.DIDs
	err := database.DB.First(&existing, "did = ?", deployerDID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("⚠️ DID entry not found for %s", deployerDID)
		return nil
	} else if err != nil {
		return err
	}

	existing.TotalSC -= 1
	if existing.TotalSC < 0 {
		existing.TotalSC = 0
	}

	if err := database.DB.Save(&existing).Error; err != nil {
		return err
	}
	log.Printf("✅ Decremented DID entry for %s, new total SCs: %d", deployerDID, existing.TotalSC)

	return nil
}

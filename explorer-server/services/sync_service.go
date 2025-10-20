package services

import (
	"encoding/json"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	RBTType = "RBT"
	FTType  = "FT"
	NFTType = "NFT"
	SCType  = "SC"
)

// RBT matches the structure returned by the API
type RBT struct {
	TokenID       string  `json:"TokenID"`
	TokenValue    float64 `json:"TokenValue"`
	OwnerDID      string  `json:"OwnerDID"`
	PublisherDID  string  `json:"PublisherDID"`
	TransactionID string  `json:"TransactionID"`
	BlockHash     string  `json:"BlockHash"`
	BlockHeight   uint64  `json:"BlockHeight"`
	SyncStaus     int     `json:"SyncStaus"`
}

type FT struct {
	TokenID       string  `gorm:"column:token_id;primaryKey"`
	FTName        string  `gorm:"column:ft_name"`
	OwnerDID      string  `gorm:"column:owner_did"`
	CreatorDID    string  `gorm:"column:creator_did"`
	TokenValue    float64 `gorm:"column:token_value"`
	TransactionID string  `gorm:"column:transaction_id"`
	BlockHash     string  `gorm:"column:block_hash"`
	SyncStatus    int     `gorm:"column:sync_status"`
}

type NFT struct {
	TokenID       string  `gorm:"column:token_id;primaryKey" json:"token_id"`
	TokenValue    float64 `gorm:"column:token_value;" json:"token_value"`
	OwnerDID      string  `gorm:"column:owner_did"`
	TransactionID string  `gorm:"column:transaction_id"`
	BlockHash     string  `gorm:"column:block_hash"`
	SyncStatus    int     `gorm:"column:sync_status"`
}

// GetRBTListResponse represents the structure of the API response
type GetRBTListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Result  []RBT  `json:"result"`
}

type GetFTListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Result  []FT   `json:"result"`
}

type GetNFTListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Result  []NFT  `json:"result"`
}

// FetchAndStoreAllRBTsFromFullNodeDB fetches RBTs from full node API and stores them
func FetchAndStoreAllRBTsFromFullNodeDB() error {
	apiURL := config.RubixNodeURL + "/api/de-exp/get-rbt-list"

	log.Println("📡 Fetching RBT list from:", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to call get-rbt-list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp GetRBTListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	if err := StoreRBTInfoInDB(apiResp.Result); err != nil {
		return fmt.Errorf("failed to store RBTs: %w", err)
	}

	log.Printf("✅ Successfully fetched and stored %d RBTs\n", len(apiResp.Result))
	return nil
}

func FetchAndStoreAllFTsFromFullNodeDB() error {
	apiURL := config.RubixNodeURL + "/api/de-exp/get-ft-list"

	log.Println("📡 Fetching FT list from:", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to call get-ft-list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp GetFTListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	if err := StoreFTInfoInDB(apiResp.Result); err != nil {
		return fmt.Errorf("failed to store FTs: %w", err)
	}

	log.Printf("✅ Successfully fetched and stored %d FTs\n", len(apiResp.Result))
	return nil
}

func FetchAndStoreAllNFTsFromFullNodeDB() error {
	apiURL := config.RubixNodeURL + "/api/de-exp/get-nft-list"

	log.Println("📡 Fetching NFT list from:", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to call get-nft-list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp GetNFTListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	if err := StoreNFTInfoInDB(apiResp.Result); err != nil {
		return fmt.Errorf("failed to store NFTs: %w", err)
	}

	log.Printf("✅ Successfully fetched and stored %d NFTs\n", len(apiResp.Result))
	return nil
}

// StoreRBTInfoInDB inserts RBTs into DB and ensures a corresponding token_type entry exists
func StoreRBTInfoInDB(RBTs []RBT) error {
	for _, rbt := range RBTs {
		rbtModel := models.RBT{
			TokenID:     rbt.TokenID,
			TokenValue:  rbt.TokenValue,
			OwnerDID:    rbt.OwnerDID,
			BlockID:     rbt.BlockHash,
			BlockHeight: fmt.Sprintf("%d", rbt.BlockHeight),
		}

		if err := database.DB.FirstOrCreate(&rbtModel, models.RBT{TokenID: rbt.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert RBT %s: %v", rbt.TokenID, err)
			continue
		}
		log.Printf("✅ RBT inserted or exists: %s", rbt.TokenID)

		tokenType := models.TokenType{
			TokenID:     rbt.TokenID,
			TokenType:   RBTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: rbt.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", rbt.TokenID, err)
		} else {
			log.Printf("✅ TokenType inserted or exists: %s (%s)", rbt.TokenID, RBTType)
		}
	}

	return nil
}

// StoreRBTInfoInDB inserts RBTs into DB and ensures a corresponding token_type entry exists
func StoreFTInfoInDB(FTs []FT) error {
	for _, ft := range FTs {
		ftmodel := models.FT{
			FtID:       ft.TokenID,
			TokenValue: ft.TokenValue,
			FTName:     ft.FTName,
			OwnerDID:   ft.OwnerDID,
			CreatorDID: ft.CreatorDID,
			BlockID:    ft.BlockHash,
			Txn_ID:     ft.TransactionID,
			// BlockHeight: fmt.Sprintf("%d", ft.BlockHeight),
		}

		if err := database.DB.FirstOrCreate(&ftmodel, models.FT{FtID: ft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert FT %s: %v", ft.TokenID, err)
			continue
		}
		log.Printf("✅ FT inserted or exists: %s", ft.TokenID)

		tokenType := models.TokenType{
			TokenID:     ft.TokenID,
			TokenType:   FTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: ft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", ft.TokenID, err)
		} else {
			log.Printf("✅ TokenType inserted or exists: %s (%s)", ft.TokenID, FTType)
		}
	}
	return nil
}

func StoreNFTInfoInDB(NFTs []NFT) error {
	for _, nft := range NFTs {
		nftmodel := models.NFT{
			TokenID:    nft.TokenID,
			TokenValue: nft.TokenValue,
			OwnerDID:   nft.OwnerDID,
			BlockHash:  nft.BlockHash,
			Txn_ID:     nft.TransactionID,
			// BlockHeight: fmt.Sprintf("%d", ft.BlockHeight),
		}

		if err := database.DB.FirstOrCreate(&nftmodel, models.NFT{TokenID: nft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert NFT %s: %v", nft.TokenID, err)
			continue
		}
		log.Printf("✅ NFT inserted or exists: %s", nft.TokenID)

		tokenType := models.TokenType{
			TokenID:     nft.TokenID,
			TokenType:   NFTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: nft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", nft.TokenID, err)
		} else {
			log.Printf("✅ TokenType inserted or exists: %s (%s)", nft.TokenID, NFTType)
		}
	}
	return nil
}

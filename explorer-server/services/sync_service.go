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

// GetRBTListResponse represents the structure of the API response
type GetRBTListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Result  []RBT  `json:"result"`
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

// StoreRBTInfoInDB inserts RBTs into DB and ensures a corresponding token_type entry exists
func StoreRBTInfoInDB(RBTs []RBT) error {
	for _, rbt := range RBTs {
		// Map API RBT to DB model
		rbtModel := models.RBT{
			TokenID:     rbt.TokenID,
			TokenValue:  rbt.TokenValue,
			OwnerDID:    rbt.OwnerDID,
			BlockID:     rbt.BlockHash,
			BlockHeight: fmt.Sprintf("%d", rbt.BlockHeight),
		}

		// Insert into rbt table using FirstOrCreate (same style as insertDummyRBTs)
		if err := database.DB.FirstOrCreate(&rbtModel, models.RBT{TokenID: rbt.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert RBT %s: %v", rbt.TokenID, err)
			continue
		}
		log.Printf("✅ RBT inserted or exists: %s", rbt.TokenID)

		// Insert into token_types table using FirstOrCreate (same style)
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

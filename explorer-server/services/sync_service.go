package services

import (
	"encoding/json"
	"explorer-server/config"
	"fmt"
	"io/ioutil"
	"net/http"
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

	// Make GET request
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

	// Parse JSON response
	var apiResp GetRBTListResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	storeRBTErr := StoreRBTInfoInDB(apiResp.Result)
	if storeRBTErr != nil {
		return fmt.Errorf("failed to store RBTs")
	}

	fmt.Printf("Successfully fetched and stored %d RBTs\n", len(apiResp.Result))
	return nil
}

func StoreRBTInfoInDB(RBTs []RBT) error {
	// Loop through RBTs and store them
	for _, rbt := range RBTs {
		fmt.Printf("Storing RBT: %+v\n", rbt)

	}
	return nil
}

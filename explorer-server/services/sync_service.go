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

type RBT struct {
	TokenID       string
	TokenValue    float64
	OwnerDID      string
	PublisherDID  string
	TransactionID string
	BlockHash     string
	BlockHeight   uint64
	SyncStaus     int
}

type FT struct {
	TokenID       string
	FTName        string
	OwnerDID      string
	CreatorDID    string
	TokenValue    float64
	TransactionID string
	BlockHash     string
	SyncStatus    int
}

type NFT struct {
	TokenID       string
	TokenValue    float64
	OwnerDID      string
	TransactionID string
	BlockHash     string
	SyncStatus    int
}

type SC struct {
	SmartContractHash string `json:"smart_contract_hash"`
	Deployer          string `json:"deployer"`
	TransactionID     string `json:"TransactionID"`
	BlockHash         string `json:"BlockHash"`
	SyncStatus        int    `json:"SyncStatus"`
}

// API response structs
type GetRBTListResponse struct {
	Status  bool
	Message string
	Result  []RBT
}

type GetFTListResponse struct {
	Status  bool
	Message string
	Result  []FT
}

type GetNFTListResponse struct {
	Status  bool
	Message string
	Result  []NFT
}

type GetSCListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Result  []SC   `json:"result"`
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

func FetchAndStoreAllSCsFromFullNodeDB() error {
	apiURL := config.RubixNodeURL + "/api/de-exp/get-smart-contract-list"

	log.Println("📡 Fetching SC list from:", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to call get-smart-contract-list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp GetSCListResponse
	fmt.Println("API response from SC API is:", apiResp)
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	if err := StoreSCInfoInDB(apiResp.Result); err != nil {
		return fmt.Errorf("failed to store SCs: %w", err)
	}

	log.Printf("✅ Successfully fetched and stored %d SCs\n", len(apiResp.Result))
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
			TokenValue: fmt.Sprintf("%f", nft.TokenValue),
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

func StoreSCInfoInDB(SCs []SC) error {
	for _, sc := range SCs {
		scmodel := models.SmartContract{
			ContractID:  sc.SmartContractHash,
			BlockHash:   sc.BlockHash,
			DeployerDID: sc.Deployer,
			TxnId:       sc.TransactionID,
			// BlockHeight: fmt.Sprintf("%d", ft.BlockHeight),
		}

		fmt.Printf("Inserting SC: %+v\n", scmodel)

		if err := database.DB.FirstOrCreate(&scmodel, models.SmartContract{ContractID: sc.SmartContractHash}).Error; err != nil {
			log.Printf("⚠️ Failed to insert SC %s: %v", sc.SmartContractHash, err)
			continue
		}
		log.Printf("✅ SC inserted or exists: %s", sc.SmartContractHash)

		// Optionally, insert into token type table
		tokenType := models.TokenType{
			TokenID:     sc.SmartContractHash,
			TokenType:   SCType,
			LastUpdated: time.Now(),
		}
		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: sc.SmartContractHash}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token type for SC %s: %v", sc.SmartContractHash, err)
		}
	}
	return nil
}

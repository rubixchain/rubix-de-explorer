package services

import (
	"encoding/json"
	"errors"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	didCount := make(map[string]int)
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
		}
		didCount[rbt.OwnerDID]++
	}

	for did, count := range didCount {
		var existing models.DIDs
		if err := database.DB.First(&existing, "did = ?", did).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newDID := models.DIDs{
					DID:       did,
					CreatedAt: time.Now(),
					TotalRBTs: float64(count),
				}
				if err := database.DB.Create(&newDID).Error; err != nil {
					log.Printf("⚠️ Failed to create DID %s: %v", did, err)
				}
			}
		} else {
			existing.TotalRBTs += float64(count)
			if err := database.DB.Save(&existing).Error; err != nil {
				log.Printf("⚠️ Failed to update TotalRBTs for DID %s: %v", did, err)
			}
		}
	}

	return nil
}

func StoreFTInfoInDB(FTs []FT) error {
	didCount := make(map[string]int)
	for _, ft := range FTs {
		ftmodel := models.FT{
			FtID:       ft.TokenID,
			TokenValue: ft.TokenValue,
			FTName:     ft.FTName,
			OwnerDID:   ft.OwnerDID,
			CreatorDID: ft.CreatorDID,
			BlockID:    ft.BlockHash,
			Txn_ID:     ft.TransactionID,
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
		}
		didCount[ft.OwnerDID]++
	}

	for did, count := range didCount {
		var existing models.DIDs
		if err := database.DB.First(&existing, "did = ?", did).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newDID := models.DIDs{
					DID:       did,
					CreatedAt: time.Now(),
					TotalFTs:  float64(count),
				}
				if err := database.DB.Create(&newDID).Error; err != nil {
					log.Printf("⚠️ Failed to create DID %s: %v", did, err)
				}
			}
		} else {
			existing.TotalFTs += float64(count)
			if err := database.DB.Save(&existing).Error; err != nil {
				log.Printf("⚠️ Failed to update TotalFTs for DID %s: %v", did, err)
			}
		}
	}

	return nil
}

func StoreNFTInfoInDB(NFTs []NFT) error {
	didCount := make(map[string]int)
	for _, nft := range NFTs {
		nftmodel := models.NFT{
			TokenID:    nft.TokenID,
			TokenValue: fmt.Sprintf("%f", nft.TokenValue),
			OwnerDID:   nft.OwnerDID,
			BlockHash:  nft.BlockHash,
			Txn_ID:     nft.TransactionID,
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
		}
		didCount[nft.OwnerDID]++
	}

	for did, count := range didCount {
		var existing models.DIDs
		if err := database.DB.First(&existing, "did = ?", did).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newDID := models.DIDs{
					DID:       did,
					CreatedAt: time.Now(),
					TotalNFTs: int64(count),
				}
				if err := database.DB.Create(&newDID).Error; err != nil {
					log.Printf("⚠️ Failed to create DID %s: %v", did, err)
				}
			}
		} else {
			existing.TotalNFTs += int64(count)
			if err := database.DB.Save(&existing).Error; err != nil {
				log.Printf("⚠️ Failed to update TotalNFTs for DID %s: %v", did, err)
			}
		}
	}

	return nil
}

func StoreSCInfoInDB(SCs []SC) error {
	didCount := make(map[string]int)
	for _, sc := range SCs {
		scmodel := models.SmartContract{
			ContractID:  sc.SmartContractHash,
			BlockHash:   sc.BlockHash,
			DeployerDID: sc.Deployer,
			TxnId:       sc.TransactionID,
		}

		if err := database.DB.FirstOrCreate(&scmodel, models.SmartContract{ContractID: sc.SmartContractHash}).Error; err != nil {
			log.Printf("⚠️ Failed to insert SC %s: %v", sc.SmartContractHash, err)
			continue
		}
		log.Printf("✅ SC inserted or exists: %s", sc.SmartContractHash)

		tokenType := models.TokenType{
			TokenID:     sc.SmartContractHash,
			TokenType:   SCType,
			LastUpdated: time.Now(),
		}
		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: sc.SmartContractHash}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for SC %s: %v", sc.SmartContractHash, err)
		}
		didCount[sc.Deployer]++
	}

	for did, count := range didCount {
		var existing models.DIDs
		if err := database.DB.First(&existing, "did = ?", did).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newDID := models.DIDs{
					DID:       did,
					CreatedAt: time.Now(),
					TotalSC:   int64(count),
				}
				if err := database.DB.Create(&newDID).Error; err != nil {
					log.Printf("⚠️ Failed to create DID %s: %v", did, err)
				}
			}
		} else {
			existing.TotalSC += int64(count)
			if err := database.DB.Save(&existing).Error; err != nil {
				log.Printf("⚠️ Failed to update TotalSC for DID %s: %v", did, err)
			}
		}
	}

	return nil
}

// FetchAllTokenChainFromFullNode iterates over all tokens and stores transfer blocks
func FetchAllTokenChainFromFullNode() error {
	var tokens []models.TokenType

	// Fetch all tokens from DB
	if err := database.DB.Find(&tokens).Error; err != nil {
		log.Fatalf("❌ Failed to fetch tokens from DB: %v", err)
		return err
	}

	for _, token := range tokens {
		apiURL := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
			config.RubixNodeURL, token.TokenID, token.TokenType)

		resp, err := http.Get(apiURL)
		if err != nil {
			log.Printf("❌ Error fetching chain for %s: %v", token.TokenID, err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("❌ Error reading response for %s: %v", token.TokenID, err)
			continue
		}

		var chainData map[string]interface{}
		if err := json.Unmarshal(body, &chainData); err != nil {
			log.Printf("❌ Error decoding JSON for %s: %v", token.TokenID, err)
			continue
		}

		// ✅ Correct key for blocks
		var blocks []interface{}
		if b, ok := chainData["TokenChainData"].([]interface{}); ok {
			blocks = b
		} else if b, ok := chainData["blocks"].([]interface{}); ok {
			blocks = b
		} else {
			log.Printf("⚠️ No blocks found for token %s", token.TokenID)
			continue
		}

		for _, blk := range blocks {
			blockMap, ok := blk.(map[string]interface{})
			if !ok {
				continue
			}

			// Only store transfer-type blocks
			transType, ok := blockMap["TCTransTypeKey"].(string)
			if !ok || (transType != "02" && transType != "2") {
				continue
			}

			// --- Parse subkeys safely ---
			transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
			tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

			// Marshal child maps to JSON
			tokensJSON, _ := json.Marshal(tokensKey)
			pledgeMapJSON, _ := json.Marshal(blockMap["TCPledgeDetailsKey"])

			// Parse amount & epoch safely
			amount := float64Ptr(blockMap["TCTokenValueKey"])
			epoch := int64Ptr(blockMap["TCEpoch"])

			// Build DB entry
			tb := models.TransferBlocks{
				BlockHash:          fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
				PrevBlockID:        stringPtr(getNested(transInfo, "TTPreviousBlockIDKey")),
				SenderDID:          stringPtr(getNested(transInfo, "TISenderDIDKey")),
				ReceiverDID:        stringPtr(getNested(transInfo, "TIReceiverDIDKey")),
				TxnType:            stringPtr(transType),
				TxnID:              stringPtr(getNested(transInfo, "TITIDKey")),
				Amount:             amount,
				Epoch:              epoch,
				Tokens:             datatypes.JSON(tokensJSON),
				ValidatorPledgeMap: datatypes.JSON(pledgeMapJSON),
			}

			// Upsert into DB
			if err := database.DB.Clauses(clause.OnConflict{
				UpdateAll: true,
			}).Create(&tb).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
				log.Printf("❌ Failed to store transfer block %v: %v", tb.BlockHash, err)
			}
		}

		time.Sleep(100 * time.Millisecond) // avoid hammering full node
	}

	log.Println("✅ Finished fetching all token chains and storing transfer blocks")
	return nil
}

// Safe string pointer
func stringPtr(v interface{}) *string {
	if v == nil {
		return nil
	}
	str := fmt.Sprintf("%v", v)
	return &str
}

// Safe float64 pointer
func float64Ptr(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		return &val
	case int:
		f := float64(val)
		return &f
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return &f
	default:
		return nil
	}
}

// Safe int64 pointer
func int64Ptr(v interface{}) *int64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		i := int64(val)
		return &i
	case int64:
		return &val
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return &i
	default:
		return nil
	}
}

// getNested safely fetches a nested map key
func getNested(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}

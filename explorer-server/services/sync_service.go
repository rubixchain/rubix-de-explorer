package services

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/database/models"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RBTType  = "RBT"
	PartType = "PART"
	FTType   = "FT"
	NFTType  = "NFT"
	SCType   = "SC"
)

// These structs are ONLY for receiving API responses

// RBT - All PascalCase
type RBT struct {
	TokenID       string  `json:"TokenID"`
	TokenValue    float64 `json:"TokenValue"`
	OwnerDID      string  `json:"OwnerDID"`
	PublisherDID  string  `json:"PublisherDID"`
	TransactionID string  `json:"TransactionID"`
	BlockHash     string  `json:"BlockHash"`
	BlockHeight   uint64  `json:"BlockHeight"`
	SyncStaus     int     `json:"SyncStaus"` // Note: typo in API
	TokenStatus   int     `json:"TokenStatus"`
}

// FT - All PascalCase
type FT struct {
	TokenID       string  `json:"TokenID"`
	FTName        string  `json:"FTName"`
	OwnerDID      string  `json:"OwnerDID"`
	CreatorDID    string  `json:"CreatorDID"`
	PublisherDID  string  `json:"PublisherDID"`
	TokenValue    float64 `json:"TokenValue"`
	TransactionID string  `json:"TransactionID"`
	BlockHash     string  `json:"BlockHash"`
	BlockHeight   uint64  `json:"BlockHeight"`
	SyncStatus    int     `json:"SyncStatus"`
	TokenStatus   int     `json:"TokenStatus"`
}

// NFT - Mix of snake_case and PascalCase
type NFT struct {
	TokenID       string  `json:"token_id"`
	TokenValue    float64 `json:"token_value"`
	OwnerDID      string  `json:"OwnerDID"`
	PublisherDID  string  `json:"PublisherDID"`
	TransactionID string  `json:"TransactionID"`
	BlockHash     string  `json:"BlockHash"`
	BlockHeight   uint64  `json:"BlockHeight"`
	SyncStatus    int     `json:"SyncStatus"`
	TokenStatus   int     `json:"TokenStatus"`
}

// SC - Mix of snake_case and lowercase/PascalCase
type SC struct {
	SmartContractHash string `json:"smart_contract_hash"`
	Deployer          string `json:"deployer"`
	PublisherDID      string `json:"PublisherDID"`
	TransactionID     string `json:"TransactionID"`
	BlockHash         string `json:"BlockHash"`
	BlockHeight       uint64 `json:"BlockHeight"`
	SyncStatus        int    `json:"SyncStatus"`
	TokenStatus       int    `json:"TokenStatus"`
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

var insecureHTTPClient = &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true,
        },
        // Disable HTTP/2 (Go otherwise forces TLS verification again)
        ForceAttemptHTTP2: false,
        TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
    },
}

// FetchAndStoreAllRBTsFromFullNodeDB fetches RBTs from full node API and stores them
func FetchAndStoreAllRBTsFromFullNodeDB() error {
	apiURL := config.RubixNodeURL + "/api/de-exp/get-rbt-list"

	log.Println("📡 Fetching RBT list from:", apiURL)

	// 🔥 Disable TLS verification for this request ONLY
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("failed to call get-rbt-list API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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

	resp, err := insecureHTTPClient.Get(apiURL)
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

	resp, err := insecureHTTPClient.Get(apiURL)
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

	resp, err := insecureHTTPClient.Get(apiURL)
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
	didValueSum := make(map[string]float64)
	for _, rbt := range RBTs {
		rbtModel := models.RBT{
			TokenID:     rbt.TokenID,
			TokenValue:  rbt.TokenValue,
			OwnerDID:    rbt.OwnerDID,
			BlockID:     rbt.BlockHash,
			BlockHeight: fmt.Sprintf("%d", rbt.BlockHeight),
			TokenStatus: rbt.TokenStatus,
		}

		var existingRBT models.RBT
		err := database.DB.Where("rbt_id = ?", rbt.TokenID).First(&existingRBT).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.DB.Create(&rbtModel).Error; err != nil {
				log.Printf("⚠️ Failed to insert RBT %s: %v", rbt.TokenID, err)
				continue
			}
			log.Printf("✅ RBT inserted: %s", rbt.TokenID)

			// Only add to sum if TokenStatus is 0 (free)
			if rbt.TokenStatus == 0 {
				didValueSum[rbt.OwnerDID] += rbt.TokenValue
			}
		} else if err != nil {
			log.Printf("⚠️ Error checking RBT %s: %v", rbt.TokenID, err)
			continue
		} else {
			log.Printf("ℹ️ RBT already exists, skipping: %s", rbt.TokenID)
		}

		tokenType := models.TokenType{
			TokenID:     rbt.TokenID,
			TokenType:   RBTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: rbt.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", rbt.TokenID, err)
		}
	}

	for did, valueSum := range didValueSum {
		roundedValue := math.Round(valueSum*1000) / 1000

		var existing models.DIDs
		if err := database.DB.First(&existing, "did = ?", did).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newDID := models.DIDs{
					DID:       did,
					CreatedAt: time.Now(),
					TotalRBTs: roundedValue,
				}
				if err := database.DB.Create(&newDID).Error; err != nil {
					log.Printf("⚠️ Failed to create DID %s: %v", did, err)
				}
			}
		} else {
			existing.TotalRBTs += roundedValue
			existing.TotalRBTs = math.Round(existing.TotalRBTs*1000) / 1000
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
		err := database.DB.Where("ft_id = ?", ft.TokenID).First(&existingFT).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.DB.Create(&ftmodel).Error; err != nil {
				log.Printf("⚠️ Failed to insert FT %s: %v", ft.TokenID, err)
				continue
			}
			log.Printf("✅ FT inserted: %s", ft.TokenID)

			// Only increment count if TokenStatus is 0 (free)
			if ft.TokenStatus == 0 {
				didCount[ft.OwnerDID]++
			}
		} else if err != nil {
			log.Printf("⚠️ Error checking FT %s: %v", ft.TokenID, err)
			continue
		} else {
			log.Printf("ℹ️ FT already exists, skipping: %s", ft.TokenID)
		}

		tokenType := models.TokenType{
			TokenID:     ft.TokenID,
			TokenType:   FTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: ft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", ft.TokenID, err)
		}
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
			TokenID:     nft.TokenID,
			TokenValue:  fmt.Sprintf("%f", nft.TokenValue),
			OwnerDID:    nft.OwnerDID,
			BlockHash:   nft.BlockHash,
			Txn_ID:      nft.TransactionID,
			BlockHeight: nft.BlockHeight,
			TokenStatus: nft.TokenStatus,
		}

		var existingNFT models.NFT
		err := database.DB.Where("nft_id = ?", nft.TokenID).First(&existingNFT).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.DB.Create(&nftmodel).Error; err != nil {
				log.Printf("⚠️ Failed to insert NFT %s: %v", nft.TokenID, err)
				continue
			}
			log.Printf("✅ NFT inserted: %s", nft.TokenID)

			didCount[nft.OwnerDID]++
		} else if err != nil {
			log.Printf("⚠️ Error checking NFT %s: %v", nft.TokenID, err)
			continue
		} else {
			log.Printf("NFT already exists, skipping: %s", nft.TokenID)
		}

		tokenType := models.TokenType{
			TokenID:     nft.TokenID,
			TokenType:   NFTType,
			LastUpdated: time.Now(),
		}

		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: nft.TokenID}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for %s: %v", nft.TokenID, err)
		}
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
			BlockHeight: sc.BlockHeight,
			TokenStatus: sc.TokenStatus,
		}

		var existingSC models.SmartContract
		err := database.DB.Where("contract_id = ?", sc.SmartContractHash).First(&existingSC).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.DB.Create(&scmodel).Error; err != nil {
				log.Printf("⚠️ Failed to insert SC %s: %v", sc.SmartContractHash, err)
				continue
			}
			log.Printf("✅ SC inserted: %s", sc.SmartContractHash)

			didCount[sc.Deployer]++
		} else if err != nil {
			log.Printf("⚠️ Error checking SC %s: %v", sc.SmartContractHash, err)
			continue
		} else {
			log.Printf("ℹ️ SC already exists, skipping: %s", sc.SmartContractHash)
		}

		tokenType := models.TokenType{
			TokenID:     sc.SmartContractHash,
			TokenType:   SCType,
			LastUpdated: time.Now(),
		}
		if err := database.DB.FirstOrCreate(&tokenType, models.TokenType{TokenID: sc.SmartContractHash}).Error; err != nil {
			log.Printf("⚠️ Failed to insert token_type for SC %s: %v", sc.SmartContractHash, err)
		}
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

// // FetchAllTokenChainFromFullNode iterates over all tokens and stores different blocks
// func FetchAllTokenChainFromFullNode() error {
// 	var tokens []models.TokenType

// 	// Fetch all tokens from DB
// 	if err := database.DB.Find(&tokens).Error; err != nil {
// 		log.Fatalf("❌ Failed to fetch tokens from DB: %v", err)
// 		return err
// 	}

// 	for _, token := range tokens {
// 		apiURL := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
// 			config.RubixNodeURL, token.TokenID, token.TokenType)

// 		resp, err := http.Get(apiURL)
// 		if err != nil {
// 			log.Printf("❌ Error fetching chain for %s: %v", token.TokenID, err)
// 			continue
// 		}
// 		defer resp.Body.Close()

// 		body, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			log.Printf("❌ Error reading response for %s: %v", token.TokenID, err)
// 			continue
// 		}

// 		var chainData map[string]interface{}
// 		if err := json.Unmarshal(body, &chainData); err != nil {
// 			log.Printf("❌ Error decoding JSON for %s: %v", token.TokenID, err)
// 			continue
// 		}

// 		var blocks []interface{}
// 		if b, ok := chainData["TokenChainData"].([]interface{}); ok {
// 			blocks = b
// 		} else if b, ok := chainData["blocks"].([]interface{}); ok {
// 			blocks = b
// 		} else {
// 			log.Printf("⚠️ No blocks found for token %s", token.TokenID)
// 			continue
// 		}

// 		for _, blk := range blocks {
// 			blockMap, ok := blk.(map[string]interface{})
// 			if !ok {
// 				continue
// 			}

// 			// Always store in AllBlocks first
// 			StoreBlockInAllBlocks(blockMap)

// 			// Extract transaction type
// 			transType, _ := blockMap["TCTransTypeKey"].(string)

// 			// Smart Contract (Deploy or Execute)
// 			if token.TokenType == "SC" {
// 				switch transType {
// 				case "09", "9":
// 					StoreSCDeployBlock(blockMap)
// 				case "10":
// 					StoreSCExecuteBlock(blockMap)
// 				default:
// 					log.Printf("⚠️ Ignoring non-SC block type %s for token %s", transType, token.TokenID)
// 				}
// 				continue
// 			}

// 			// Regular tokens (FT, NFT, RBT)
// 			switch transType {
// 			case "02", "2":
// 				StoreTransferBlock(blockMap)
// 			case "08", "13":
// 				StoreBurntBlock(blockMap)
// 			default:
// 				log.Printf("⚠️ Unknown block type %s for token %s", transType, token.TokenID)
// 			}
// 		}

// 		// Small delay between tokens to avoid overloading node
// 		time.Sleep(100 * time.Millisecond)
// 	}

// 	log.Println("✅ Finished fetching all token chains and storing block data")
// 	return nil
// }

// StoreTransferBlock handles inserting a single transfer-type block into DB
func StoreTransferBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	tokensJSON, _ := json.Marshal(tokensKey)
	pledgeMapJSON, _ := json.Marshal(blockMap["TCPledgeDetailsKey"])

	amount := float64Ptr(blockMap["TCTokenValueKey"])
	epoch := int64Ptr(blockMap["TCEpoch"])

	tb := models.TransferBlocks{
		BlockHash:          fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		PrevBlockID:        stringPtr(getNested(transInfo, "TTPreviousBlockIDKey")),
		SenderDID:          stringPtr(getNested(transInfo, "TISenderDIDKey")),
		ReceiverDID:        stringPtr(getNested(transInfo, "TIReceiverDIDKey")),
		TxnType:            stringPtr(getNested(blockMap, "TCTransTypeKey")),
		TxnID:              stringPtr(getNested(transInfo, "TITIDKey")),
		Amount:             amount,
		Epoch:              epoch,
		Tokens:             datatypes.JSON(tokensJSON),
		ValidatorPledgeMap: datatypes.JSON(pledgeMapJSON),
	}

	if err := database.DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&tb).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		log.Printf("❌ Failed to store transfer block %v: %v", tb.BlockHash, err)
	}
	// time.Sleep(100 * time.Millisecond) // avoid hammering full node
	log.Println("Transfer block stored")
}

// StoreBurntBlock handles inserting a single burnt-type block into DB
func StoreBurntBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	tokensJSON, _ := json.Marshal(tokensKey)
	childTokensJSON, _ := json.Marshal(blockMap["TCChildTokensKey"])

	// Extract epoch from comment (e.g. "Token burnt at : 2025-10-09 15:31:14")
	var epoch *int64
	if comment, ok := transInfo["TICommentKey"].(string); ok {
		re := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
		if match := re.FindString(comment); match != "" {
			// Load IST timezone (UTC+5:30)
			ist, err := time.LoadLocation("Asia/Kolkata")
			if err != nil {
				log.Printf("⚠️ Failed to load IST timezone: %v", err)
				ist = time.FixedZone("IST", 5*3600+30*60) // fallback to fixed offset
			}

			// Parse the time as IST, then convert to Unix epoch (which is always UTC)
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", match, ist); err == nil {
				val := t.Unix()
				epoch = &val
			}
		}
	}

	// Normalize transaction type
	txnType := fmt.Sprintf("%v", blockMap["TCTransTypeKey"])
	var txnTypeStr string
	switch txnType {
	case "13":
		txnTypeStr = "Burnt for FT"
	case "08":
		txnTypeStr = "Burnt"
	default:
		txnTypeStr = "Unknown"
	}

	bb := models.BurntBlocks{
		BlockHash:   fmt.Sprintf("%v", blockMap["TCBlockHashKey"]),
		ChildTokens: datatypes.JSON(childTokensJSON),
		TxnType:     &txnTypeStr,
		OwnerDID:    fmt.Sprintf("%v", blockMap["TCTokenOwnerKey"]),
		Epoch:       epoch,
		Tokens:      datatypes.JSON(tokensJSON),
	}

	if err := database.DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&bb).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		log.Printf("❌ Failed to store burnt block %v: %v", bb.BlockHash, err)
	}

	log.Println("✅ Burnt block stored:", bb.BlockHash)
}

// StoreSCDeployBlock handles inserting a smart contract deploy block into DB
func StoreSCDeployBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	blockID := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])

	var contractID string
	var blockHeight int64
	for k, v := range tokensKey {
		contractID = k
		if tk, ok := v.(map[string]interface{}); ok {
			if bh, ok := tk["TTBlockNumberKey"].(string); ok {
				if vbh, err := strconv.ParseInt(bh, 10, 64); err == nil {
					blockHeight = vbh
				}
			}
		}
		break
	}

	// Parse epoch
	var epoch time.Time
	if e, ok := blockMap["TCEpoch"].(float64); ok {
		epoch = time.Unix(int64(e), 0)
	}

	// Owner_DID from TIDeployerDIDKey
	ownerDID := fmt.Sprintf("%v", getNested(transInfo, "TIDeployerDIDKey"))

	scBlock := models.SC_Block{
		Block_ID:     blockID,
		Contract_ID:  contractID,
		Block_Height: blockHeight,
		Epoch:        epoch,
		Owner_DID:    ownerDID,
	}

	// Insert or update (on conflict block_id)
	if err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "block_id"}},
		UpdateAll: true,
	}).Create(&scBlock).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		log.Printf("❌ Failed to store SC deploy block %v: %v", scBlock.Contract_ID, err)
		return
	}

	log.Println("SC Deploy block stored:", scBlock.Block_ID)
}

// StoreSCExecuteBlock handles inserting a smart contract execute block into DB
func StoreSCExecuteBlock(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})
	tokensKey, _ := transInfo["TITokensKey"].(map[string]interface{})

	blockID := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])

	var contractID string
	var blockHeight int64
	for k, v := range tokensKey {
		contractID = k
		if tk, ok := v.(map[string]interface{}); ok {
			if bh, ok := tk["TTBlockNumberKey"].(string); ok {
				if vbh, err := strconv.ParseInt(bh, 10, 64); err == nil {
					blockHeight = vbh
				}
			}
		}
		break
	}

	// Parse epoch (if available)
	var epoch time.Time
	if e, ok := blockMap["TCEpoch"].(float64); ok {
		epoch = time.Unix(int64(e), 0)
	}

	// Executor_DID from TIExecutorDIDKey
	execDidStr := getNested(transInfo, "TIExecutorDIDKey")
	execDidPtr := stringPtr(execDidStr)

	scBlock := models.SC_Block{
		Block_ID:     blockID,
		Contract_ID:  contractID,
		Executor_DID: execDidPtr,
		Block_Height: blockHeight,
		Epoch:        epoch,
	}

	// Insert or update (on conflict block_id)
	if err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "block_id"}},
		UpdateAll: true,
	}).Create(&scBlock).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		log.Printf("❌ Failed to store SC execute block %v: %v", scBlock.Contract_ID, err)
		return
	}

	log.Println("SC Execute block stored:", scBlock.Block_ID)
}

// StoreBlockInAllBlocks inserts a block entry into the AllBlocks table
func StoreBlockInAllBlocks(blockMap map[string]interface{}) {
	transInfo, _ := blockMap["TCTransInfoKey"].(map[string]interface{})

	blockHash := fmt.Sprintf("%v", blockMap["TCBlockHashKey"])
	txnID := fmt.Sprintf("%v", transInfo["TITIDKey"])

	// Determine readable block type
	var blockType string
	switch fmt.Sprintf("%v", blockMap["TCTransTypeKey"]) {
	case "02", "2":
		blockType = "transfer"
	case "08":
		blockType = "burnt"
	case "13":
		blockType = "burnt_for_ft"
	case "09", "9":
		blockType = "deploy"
	case "10":
		blockType = "execute"
	case "05", "5":
		blockType = "mint"
	default:
		blockType = "unknown"
	}

	// Parse epoch timestamp
	var epochTime time.Time
	switch v := blockMap["TCEpoch"].(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			epochTime = t
		} else {
			epochTime = time.Now()
		}
	case float64:
		epochTime = time.Unix(int64(v), 0)
	case int64:
		epochTime = time.Unix(v, 0)
	default:
		epochTime = time.Now()
	}

	record := models.AllBlocks{
		BlockHash: blockHash,
		BlockType: blockType,
		Epoch:     epochTime,
		TxnID:     txnID,
	}

	// Insert — if duplicate, skip (no update needed)
	if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
		log.Printf("❌ Failed to insert block into AllBlocks (%v): %v", blockHash, err)
		return
	}

	log.Printf("Stored block in AllBlocks: %v (type=%s)", blockHash, blockType)
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

func sliceStringPtr(v interface{}) *[]string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	strs := make([]string, 0, len(arr))
	for _, val := range arr {
		if s, ok := val.(string); ok {
			strs = append(strs, s)
		}
	}
	return &strs
}

/// sequential token chain fetching

// FetchAllTokenChainFromFullNode syncs token chains sequentially
func FetchAllTokenChainFromFullNode() error {
	var tokens []models.TokenType

	// Fetch all tokens from DB
	if err := database.DB.Find(&tokens).Error; err != nil {
		log.Fatalf("❌ Failed to fetch tokens from DB: %v", err)
		return err
	}

	if len(tokens) == 0 {
		log.Println("⚠️ No tokens found to sync")
		return nil
	}

	log.Printf("ℹ️ Starting sync for %d tokens", tokens)

	successCount := 0
	failureCount := 0
	var errs []error

	for idx, token := range tokens {
		fmt.Printf("🔄 Syncing token %d/%d: ID=%s, Type=%s\n",)
		if err := fetchAndStoreTokenChain(token); err != nil {
			errs = append(errs, err)
			failureCount++
		} else {
			successCount++
		}

		// Log progress every 1000 tokens
		if (idx+1)%1000 == 0 {
			log.Printf("📈 Progress: %d/%d tokens processed (Success: %d, Failures: %d)",
				idx+1, len(tokens), successCount, failureCount)
		}

		// Prevent hammering the node
		time.Sleep(5 * time.Millisecond)
	}

	if len(errs) > 0 {
		log.Printf("⚠️ Completed with %d errors out of %d tokens", len(errs), len(tokens))
	}

	log.Printf("✅ Finished fetching all token chains and storing block data (Success: %d, Failures: %d)",
		successCount, failureCount)

	return nil
}


// fetchAndStoreTokenChain handles individual token chain syncing with retry logic
func fetchAndStoreTokenChain(token models.TokenType) error {
	fmt.Println("tokenInfo for getting tokenchain is:", token)
	apiURL := fmt.Sprintf("%s/api/de-exp/get-token-chain?tokenID=%s&tokenType=%s",
		config.RubixNodeURL, token.TokenID, token.TokenType)


	// Retry logic with exponential backoff
	maxRetries := 3
	var resp *http.Response
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := insecureHTTPClient.Get(apiURL)
        print("testing-02:", resp)
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Printf("✅ Successfully fetched chain for %s on attempt %d\n", token.TokenID, attempt+1)
			break
		}

		if err != nil {
			log.Printf("❌ Attempt %d: Error fetching chain for %s: %v", attempt+1, token.TokenID, err)
		} else if resp.StatusCode != http.StatusOK {
			log.Printf("❌ Attempt %d: Bad status %d for token %s", attempt+1, resp.StatusCode, token.TokenID)
			resp.Body.Close()
		}

		if attempt < maxRetries-1 {
			backoffTime := time.Duration(1<<uint(attempt)) * 500 * time.Millisecond
			time.Sleep(backoffTime)
		}
	}

	if err != nil {
		return err
	}
	if resp == nil {
    return fmt.Errorf("nil response received from node for token %s", token.TokenID)
}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading response for %s: %v", token.TokenID, err)
		return err
	}

	var chainData map[string]interface{}
	if err := json.Unmarshal(body, &chainData); err != nil {
		log.Printf("❌ Error decoding JSON for %s: %v", token.TokenID, err)
		return err
	}

	// Extract blocks
	// var blocks []interface{}
	// if b, ok := chainData["TokenChainData"].([]interface{}); ok {
	// 	blocks = b
	// } else if b, ok := chainData["blocks"].([]interface{}); ok {
	// 	blocks = b
	// } else {
	// 	log.Printf("⚠️ No blocks found for token %s", token.TokenID)
	// 	return nil
	// }

	// // Process blocks sequentially
	// if err := processAndStoreBlocks(token, blocks); err != nil {
	// 	return err
	// }

	// Check if API returned an error
if status, ok := chainData["status"].(bool); ok && !status {
	log.Printf("❌ API returned error for token %s: %v", token.TokenID, chainData["message"])
	return nil
}

// Extract blocks safely
var blocks []interface{}
fmt.Printf("testing-1 is:%v", chainData)
if b, ok := chainData["TokenChainData"].([]interface{}); ok {
	blocks = b
} else if b, ok := chainData["blocks"].([]interface{}); ok {
	blocks = b
} else {
	log.Printf("⚠️ No block array found for token %s (keys: %v)", token.TokenID, reflect.ValueOf(chainData).MapKeys())
	return nil
}

// Defensive guard: prevent nil slice crashes
if blocks == nil || len(blocks) == 0 {
	log.Printf("⚠️ Empty or nil block list for token %s", token.TokenID)
	return nil
}

fmt.Printf("tested", len(blocks), token.TokenID, blocks)

// Process blocks sequentially
if err := processAndStoreBlocks(token, blocks); err != nil {
	return err
}


	return nil
}

// processAndStoreBlocks handles block classification and storage
func processAndStoreBlocks(token models.TokenType, blocks []interface{}) error {
	for _, blk := range blocks {
		blockMap, ok := blk.(map[string]interface{})
		if !ok {
			continue
		}

		// Always store in AllBlocks first
		StoreBlockInAllBlocks(blockMap)

		// Extract transaction type
		transType, _ := blockMap["TCTransTypeKey"].(string)

		// Smart Contract (Deploy or Execute)
		if token.TokenType == "SC" {
			switch transType {
			case "09", "9":
				StoreSCDeployBlock(blockMap)
			case "10":
				StoreSCExecuteBlock(blockMap)
			default:
				log.Printf("⚠️ Ignoring non-SC block type %s for token %s", transType, token.TokenID)
			}
			continue
		}

		// Regular tokens (FT, NFT, RBT)
		switch transType {
		case "02", "2":
			StoreTransferBlock(blockMap)
		case "08", "13":
			StoreBurntBlock(blockMap)
		default:
			log.Printf("⚠️ Unknown block type %s for token %s", transType, token.TokenID)
		}

		// Small delay between block writes to reduce DB contention
		time.Sleep(2 * time.Millisecond)
	}

	return nil
}

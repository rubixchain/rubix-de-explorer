package main

import (
	"explorer-server/database"
	"explorer-server/router"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	// "explorer-server/services"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	log.Println("🚀 Starting Explorer Server...")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using default values")
	}

	// Initialize PostgreSQL and drop & recreate tables
	log.Println("📦 Connecting to PostgreSQL...")
	database.ConnectAndMigrate(false) // pass true to drop tables

	// Insert dummy RBT data
	// insertDummyDIDs()
	// insertDummyBurntBlocks()

	// insertDummySCBlocks()

	// insertDummyRBTs()

	// insertDummyTransferBlocks()
	// insertDummyNFTs()
	// insertDummySmartContracts()
	// insertDummyFTs()
	// insertDummyAssetTypes() // 👈 Add this line

	// RBTfetchErr := services.FetchAndStoreAllRBTsFromFullNodeDB()
	// if RBTfetchErr != nil {
	// 	log.Printf("Failed to call `FetchAndStoreAllRBTsFromFullNodeDB`, err: %v", RBTfetchErr)
	// }

	// FTfetchErr := services.FetchAndStoreAllFTsFromFullNodeDB()
	// if FTfetchErr != nil {
	// 	log.Printf("Failed to call `FetchAndStoreAllFTsFromFullNodeDB`, err: %v", FTfetchErr)
	// }

	// NFTfetchErr := services.FetchAndStoreAllNFTsFromFullNodeDB()
	// if NFTfetchErr != nil {
	// 	log.Printf("Failed to call `FetchAndStoreAllNFTsFromFullNodeDB`, err: %v", NFTfetchErr)
	// }

	// SCfetchErr := services.FetchAndStoreAllSCsFromFullNodeDB()
	// if SCfetchErr != nil {
	// 	log.Printf("Failed to call `FetchAndStoreAllSCsFromFullNodeDB`, err: %v", SCfetchErr)
	// }

	// FetchTokenChainErr := services.FetchAllTokenChainFromFullNode()
	// if FetchTokenChainErr != nil {
	// 	log.Printf("Failed to call `FetchAllTokenChainFromFullNode`, err: %v", FetchTokenChainErr)
	// }

	// Setup router
	r := router.NewRouter()

	// Enable CORS
	handler := cors.Default().Handler(r)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Received shutdown signal...")
		database.CloseDB()
		log.Println("👋 Server shutdown complete")
		os.Exit(0)
	}()

	// Start server
	log.Printf("✅ Explorer server running on port :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), handler))
}

// Insert dummy RBTs
// func insertDummyRBTs() {
// 	dummyRBTs := []models.RBT{
// 		{TokenID: "qemrbt-0019", OwnerDID: "bafy1234abcd", BlockID: "block-001", BlockHeight: "1", TokenValue: 0.5},
// 		{TokenID: "rbt-0011", OwnerDID: "bafy1234abcd", BlockID: "block-002", BlockHeight: "2", TokenValue: 1.0},
// 	}

// 	for _, rbt := range dummyRBTs {
// 		if err := database.DB.FirstOrCreate(&rbt, models.RBT{TokenID: rbt.TokenID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy RBT %s: %v", rbt.TokenID, err)
// 		} else {
// 			log.Printf("✅ Dummy RBT inserted or exists: %s", rbt.TokenID)
// 		}
// 	}
// }

// Insert dummy TransferBlocks
// func insertDummyTransferBlocks() {
// 	now := time.Now().Unix() // epoch as int64

// 	dummyBlocks := []models.TransferBlocks{
// 		newDummyBlock("block-hash-007", "did:example:sender001", "did:example:receiver001", 150.25, "txn-001",
// 			[]string{"token1", "token2", "token3"},
// 			map[string][]string{"validator1": {"pledgeA", "pledgeB"}, "validator2": {"pledgeC"}},
// 			&now,
// 		),
// 		newDummyBlock("block-hash-008", "did:example:sender002", "did:example:receiver002", 320.75, "txn-002",
// 			[]string{"tokenX", "tokenY"},
// 			map[string][]string{"validator3": {"pledgeD", "pledgeE"}},
// 			&now,
// 		),
// 		newDummyBlock("block-hash-009", "did:example:sender003", "did:example:receiver003", 890.10, "txn-003",
// 			[]string{"tokenAlpha", "tokenBeta"},
// 			map[string][]string{"validator4": {"pledgeF"}, "validator5": {"pledgeG", "pledgeH"}},
// 			&now,
// 		),
// 	}

// 	for _, block := range dummyBlocks {
// 		if err := database.DB.Create(&block).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy block %s: %v", block.BlockHash, err)
// 		} else {
// 			log.Printf("✅ Dummy transfer block inserted: %s", block.BlockHash)
// 		}
// 	}
// }

// Helper to create a dummy TransferBlock with JSON fields marshaled correctly
// func newDummyBlock(
// 	blockHash, sender, receiver string,
// 	amount float64,
// 	txnID string,
// 	tokens []string,
// 	validatorMap map[string][]string,
// 	epoch *int64,
// ) models.TransferBlocks {
// 	// Marshal slice/map to JSON
// 	tokensJSON, _ := json.Marshal(tokens)
// 	validatorJSON, _ := json.Marshal(validatorMap)

// 	return models.TransferBlocks{
// 		BlockHash:          blockHash,
// 		SenderDID:          ptr(sender),
// 		ReceiverDID:        ptr(receiver),
// 		TxnType:            ptr("transfer"),
// 		Amount:             ptrFloat(amount),
// 		Epoch:              epoch,
// 		TxnID:              ptr(txnID),
// 		Tokens:             datatypes.JSON(tokensJSON), // JSON marshaled
// 		ValidatorPledgeMap: datatypes.JSON(validatorJSON),
// 	}
// }

// Insert dummy NFTs
// func insertDummyNFTs() {
// 	dummyNFTs := []models.NFT{
// 		{TokenID: "nft-001", OwnerDID: "did:example:owner001", TokenValue: "1.2", BlockHash: "block-010", Txn_ID: "10asfsadf"},
// 		{TokenID: "nft-002", OwnerDID: "did:example:owner001", TokenValue: "1.3", BlockHash: "block-010", Txn_ID: "10asfsadf"},
// 	}

// 	for _, nft := range dummyNFTs {
// 		if err := database.DB.FirstOrCreate(&nft, models.NFT{TokenID: nft.TokenID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy NFT %s: %v", nft.TokenID, err)
// 		} else {
// 			log.Printf("✅ Dummy NFT inserted or exists: %s", nft.TokenID)
// 		}
// 	}
// }

// // Insert dummy SmartContracts
// func insertDummySmartContracts() {
// 	dummyContracts := []models.SmartContract{
// 		{
// 			ContractID:  "sc-addr-001",
// 			DeployerDID: "did:example:dev001",
// 			TxnId:       "hash001",
// 			BlockHash:   "block-020",
// 		},
// 	}

// 	for _, sc := range dummyContracts {
// 		if err := database.DB.FirstOrCreate(&sc, models.SmartContract{ContractID: sc.ContractID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy SmartContract %s: %v", sc.ContractID, err)
// 		} else {
// 			log.Printf("✅ Dummy SmartContract inserted or exists: %s", sc.ContractID)
// 		}
// 	}
// }

// Insert dummy Fungible Tokens (FTs)
// func insertDummyFTs() {
// 	dummyFTs := []models.FT{
// 		{FtID: "qem-0101-asf", FTName: "USD Synthetic", TokenValue: 0.6, OwnerDID: "did:example:issuer001", CreatorDID: "did:example:issuer002", BlockID: "block-030", BlockHeight: "30"},
// 		{FtID: "qem-01023-asf", FTName: "USD Synthetic", TokenValue: 0.6, OwnerDID: "did:example:issuer001", CreatorDID: "did:example:issuer002", BlockID: "block-030", BlockHeight: "30"},
// 	}

// 	for _, ft := range dummyFTs {
// 		if err := database.DB.FirstOrCreate(&ft, models.FT{FtID: ft.FtID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy FT %s: %v", ft.FtID, err)
// 		} else {
// 			log.Printf("✅ Dummy FT inserted or exists: %s", ft.FtID)
// 		}
// 	}
// }

// Generic helpers
// func ptr[T any](v T) *T {
// 	return &v
// }

// func ptrFloat(v float64) *float64 {
// 	return &v
// }

// func insertDummyAssetTypes() {
// 	dummyAssetTypes := []models.TokenType{
// 		{TokenID: "qemrbt-001", TokenType: "RBT", LastUpdated: time.Now()},
// 		{TokenID: "rbt-002", TokenType: "NFT", LastUpdated: time.Now()},
// 		{TokenID: "nft-001", TokenType: "SmartContract", LastUpdated: time.Now()},
// 	}

// 	for _, asset := range dummyAssetTypes {
// 		if err := database.DB.FirstOrCreate(&asset, models.TokenType{TokenID: asset.TokenID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy asset type %s: %v", asset.TokenID, err)
// 		} else {
// 			log.Printf("✅ Dummy asset type inserted or exists: %s → %s", asset.TokenID, asset.TokenType)
// 		}
// 	}
// }

// // Insert dummy DIDs
// func insertDummyDIDs() {
// 	dummyDIDs := []models.DIDs{
// 		{
// 			DID:       "bafy1234abcd",
// 			CreatedAt: time.Now(),
// 			TotalRBTs: 12.5,
// 			TotalFTs:  34.8,
// 			TotalNFTs: 5,
// 			TotalSC:   2,
// 		},
// 		{
// 			DID:       "bafy5678efgh",
// 			CreatedAt: time.Now(),
// 			TotalRBTs: 9.0,
// 			TotalFTs:  15.2,
// 			TotalNFTs: 3,
// 			TotalSC:   1,
// 		},
// 		{
// 			DID:       "bafy9999ijkl",
// 			CreatedAt: time.Now(),
// 			TotalRBTs: 22.7,
// 			TotalFTs:  10.4,
// 			TotalNFTs: 7,
// 			TotalSC:   4,
// 		},
// 	}

// 	for _, did := range dummyDIDs {
// 		if err := database.DB.FirstOrCreate(&did, models.DIDs{DID: did.DID}).Error; err != nil {
// 			log.Printf("⚠️ Failed to insert dummy DID %s: %v", did.DID, err)
// 		} else {
// 			log.Printf("✅ Dummy DID inserted or exists: %s", did.DID)
// 		}
// 	}
// }

// // Insert dummy SC blocks (sc_blocks table)
// func insertDummySCBlocks() {
// 	now := time.Now()

// 	dummySCBlocks := []models.SC_Block{
// 		{
// 			Block_ID:     "1-sc-block",
// 			Contract_ID:  "sc-addr-001",
// 			Executor_DID: ptr("did:example:executor001"),
// 			Block_Height: 101,
// 			Epoch:        now,
// 			Owner_DID:    "did:example:owner001",
// 		},
// 		{
// 			Block_ID:     "2-sc-block",
// 			Contract_ID:  "sc-addr-001",
// 			Executor_DID: ptr("did:example:executor001"),
// 			Block_Height: 101,
// 			Epoch:        now,
// 			Owner_DID:    "did:example:owner001",
// 		},
// 		{
// 			Block_ID:     "4-sc-block",
// 			Contract_ID:  "sc-addr-001",
// 			Executor_DID: ptr("did:example:executor001"),
// 			Block_Height: 101,
// 			Epoch:        now,
// 			Owner_DID:    "did:example:owner001",
// 		},
// 	}

// 	for _, sc := range dummySCBlocks {
// 		if err := database.DB.FirstOrCreate(&sc, models.SC_Block{Block_ID: sc.Block_ID}).Error; err != nil {
// 			log.Printf("Failed to insert dummy SCBlock %s: %v", sc.Block_ID, err)
// 		} else {
// 			log.Printf("Dummy SCBlock inserted or exists: %s (height=%d)", sc.Block_ID)
// 		}
// 	}
// }

// // Insert dummy Burnt blocks (burnt_blocks table)
// func insertDummyBurntBlocks() {
// 	// now := time.Now()

// 	// Example child-tokens JSON structures
// 	childTokens1 := []string{"qemrbt-0019", "nft-007"}
// 	childTokens2 := []map[string]any{
// 		{"id": "qemrbt-0019", "amount": 45.5},
// 		{"id": "ft-456", "amount": 12.0},
// 	}
// 	childTokens3 := map[string]any{
// 		"tokens": []string{"tokenA", "tokenB"},
// 		"metadata": map[string]string{
// 			"burnReason": "upgrade",
// 		},
// 	}

// 	dummyBurntBlocks := []models.BurntBlocks{
// 		{
// 			BlockHash:   "burn-hash-005",
// 			ChildTokens: jsonb(childTokens1),
// 			TxnType:     ptr("burn"),
// 			// Epoch:       now,
// 			OwnerDID: "did:example:burner001",
// 		},
// 		{
// 			BlockHash:   "burn-hash-002",
// 			ChildTokens: jsonb(childTokens2),
// 			TxnType:     ptr("burn"),
// 			// Epoch:       now.Add(-3 * time.Minute),
// 			OwnerDID: "did:example:burner002",
// 		},
// 		{
// 			BlockHash:   "burn-hash-003",
// 			ChildTokens: jsonb(childTokens3),
// 			TxnType:     ptr("burn"),
// 			// Epoch:       now.Add(-8 * time.Minute),
// 			OwnerDID: "did:example:burner001",
// 		},
// 	}

// 	for _, bb := range dummyBurntBlocks {
// 		if err := database.DB.FirstOrCreate(&bb, models.BurntBlocks{BlockHash: bb.BlockHash}).Error; err != nil {
// 			log.Printf("Failed to insert dummy BurntBlock %s: %v", bb.BlockHash, err)
// 		} else {
// 			log.Printf("Dummy BurntBlock inserted or exists: %s", bb.BlockHash)
// 		}
// 	}
// }

// tiny helpers (already in your file, just re-exported for clarity)
// func ptr[T any](v T) *T               { return &v }
// func ptrInt64(v int64) *int64 { return &v }
// func jsonb(v any) datatypes.JSON {
// 	b, _ := json.Marshal(v)
// 	return datatypes.JSON(b)
// }

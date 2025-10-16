package services

import (
	"fmt"
	"log"
	"time"

	"explorer-server/client"
	"explorer-server/database"
	"explorer-server/model"
)

// SyncService handles synchronization between full node and database
type SyncService struct {
	rubixClient       *client.RubixClient
	tokenQueries      *database.TokenQueries
	blockQueries      *database.BlockQueries
	didQueries        *database.DIDQueries
	processingService *ProcessingService
	isRunning         bool
	stopChannel       chan bool
}

// SyncStats represents synchronization statistics
type SyncStats struct {
	LastSyncTime   time.Time `json:"last_sync_time"`
	TotalTokens    int64     `json:"total_tokens"`
	TotalBlocks    int64     `json:"total_blocks"`
	TotalDIDs      int64     `json:"total_dids"`
	SyncInProgress bool      `json:"sync_in_progress"`
	LastError      string    `json:"last_error,omitempty"`
	SyncDuration   string    `json:"sync_duration"`
}

// NewSyncService creates a new synchronization service
func NewSyncService(rubixClient *client.RubixClient) *SyncService {
	db := database.GetDB()

	return &SyncService{
		rubixClient:       rubixClient,
		tokenQueries:      database.NewTokenQueries(db),
		blockQueries:      database.NewBlockQueries(db),
		didQueries:        database.NewDIDQueries(db),
		processingService: NewProcessingService(),
		isRunning:         false,
		stopChannel:       make(chan bool),
	}
}

// StartPeriodicSync starts periodic synchronization with the full node
func (s *SyncService) StartPeriodicSync(interval time.Duration) {
	if s.isRunning {
		log.Println("🔄 Sync service is already running")
		return
	}
	s.isRunning = true
	log.Printf("🚀 Starting periodic sync every %v", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run initial sync
		s.performFullSync()

		for {
			select {
			case <-ticker.C:
				s.performFullSync()
			case <-s.stopChannel:
				log.Println("🛑 Stopping sync service")
				s.isRunning = false
				return
			}
		}
	}()
}

// StopPeriodicSync stops the periodic synchronization
func (s *SyncService) StopPeriodicSync() {
	if !s.isRunning {
		return
	}

	s.stopChannel <- true
}

// performFullSync performs a complete synchronization with the full node
func (s *SyncService) performFullSync() {
	log.Println("🔄 Starting full synchronization...")
	startTime := time.Now()

	// Sync tokens (RBT, FT, NFT, SC)
	if err := s.syncTokens(); err != nil {
		log.Printf("❌ Error syncing tokens: %v", err)
	}

	// Sync token chains for RBT tokens
	if err := s.syncTokenChains(); err != nil {
		log.Printf("❌ Error syncing token chains: %v", err)
	}
	// Sync blocks/transactions
	if err := s.syncBlocks(); err != nil {
		log.Printf("❌ Error syncing blocks: %v", err)
	}
	// Run server-side processing pipeline
	log.Println("🔄 Starting server-side data processing...")
	if err := s.processingService.ProcessAll(); err != nil {
		log.Printf("❌ Error in processing pipeline: %v", err)
	}
	// Update sync statistics
	duration := time.Since(startTime)
	log.Printf("✅ Full sync completed in %v", duration)
}

// syncTokens synchronizes token data from the full node
func (s *SyncService) syncTokens() error {
	log.Println("📦 Syncing tokens...")

	// Sync RBT tokens
	if err := s.syncRBTTokens(); err != nil {
		return fmt.Errorf("failed to sync RBT tokens: %v", err)
	}

	// Sync FT tokens
	if err := s.syncFTTokens(); err != nil {
		return fmt.Errorf("failed to sync FT tokens: %v", err)
	}

	// Note: Add NFT and SC sync when APIs are available
	log.Println("✅ Token sync completed")
	return nil
}

// syncRBTTokens synchronizes RBT tokens
func (s *SyncService) syncRBTTokens() error {
	log.Println("🔄 Syncing RBT tokens...")

	// Get RBT data from full node
	rbtResponse, err := s.rubixClient.GetFreeRBTs()
	if err != nil {
		return fmt.Errorf("failed to get RBT data: %v", err)
	}

	// Parse and store RBT tokens
	for _, rbtData := range rbtResponse.Result {
		token := &database.Token{
			TokenID:      rbtData.TokenID,
			TokenType:    "RBT",
			CurrentOwner: rbtData.DID,
			State:        determineTokenState(rbtData),
			CreatedAt:    time.Now(), // Use current time if not available from API
		}

		// Check if token already exists
		existingToken, err := s.tokenQueries.GetToken(token.TokenID)
		if err != nil {
			// Token doesn't exist, create new one
			if err := s.tokenQueries.CreateToken(token); err != nil {
				log.Printf("⚠️ Failed to create RBT token %s: %v", token.TokenID, err)
				continue
			}
			log.Printf("➕ Created RBT token: %s", token.TokenID)
		} else {
			// Token exists, update if needed
			if existingToken.CurrentOwner != token.CurrentOwner || existingToken.State != token.State {
				if err := s.tokenQueries.UpdateTokenOwner(token.TokenID, token.CurrentOwner); err != nil {
					log.Printf("⚠️ Failed to update RBT token owner %s: %v", token.TokenID, err)
				}
				if err := s.tokenQueries.UpdateTokenState(token.TokenID, token.State); err != nil {
					log.Printf("⚠️ Failed to update RBT token state %s: %v", token.TokenID, err)
				}
				log.Printf("🔄 Updated RBT token: %s", token.TokenID)
			}
		}

		// Update or create DID
		s.updateDID(rbtData.DID)
	}

	log.Printf("✅ Synced %d RBT tokens", len(rbtResponse.Result))
	return nil
}

// syncFTTokens synchronizes FT tokens
func (s *SyncService) syncFTTokens() error {
	log.Println("🔄 Syncing FT tokens...")

	// Get FT data from full node
	ftResponse, err := s.rubixClient.GetFTs()
	if err != nil {
		return fmt.Errorf("failed to get FT data: %v", err)
	}

	// Parse and store FT tokens
	for _, ftData := range ftResponse.Result {
		token := &database.Token{
			TokenID:      ftData.TokenID,
			TokenType:    "FT",
			CurrentOwner: ftData.DID,
			State:        determineFTTokenState(ftData),
			CreatedAt:    time.Now(),
		}

		// Check if token already exists
		existingToken, err := s.tokenQueries.GetToken(token.TokenID)
		if err != nil {
			// Token doesn't exist, create new one
			if err := s.tokenQueries.CreateToken(token); err != nil {
				log.Printf("⚠️ Failed to create FT token %s: %v", token.TokenID, err)
				continue
			}
			log.Printf("➕ Created FT token: %s", token.TokenID)
		} else {
			// Token exists, update if needed
			if existingToken.CurrentOwner != token.CurrentOwner || existingToken.State != token.State {
				if err := s.tokenQueries.UpdateTokenOwner(token.TokenID, token.CurrentOwner); err != nil {
					log.Printf("⚠️ Failed to update FT token owner %s: %v", token.TokenID, err)
				}
				if err := s.tokenQueries.UpdateTokenState(token.TokenID, token.State); err != nil {
					log.Printf("⚠️ Failed to update FT token state %s: %v", token.TokenID, err)
				}
				log.Printf("🔄 Updated FT token: %s", token.TokenID)
			}
		}

		// Update or create DID
		s.updateDID(ftData.DID)

		// Sync FT token chain if available
		s.syncFTTokenChain(ftData.TokenID)
	}

	log.Printf("✅ Synced %d FT tokens", len(ftResponse.Result))
	return nil
}

// syncFTTokenChain synchronizes FT token chain data
func (s *SyncService) syncFTTokenChain(tokenID string) error {
	// Get FT token chain from full node
	chainResponse, err := s.rubixClient.GetFTTokenchain(tokenID)
	if err != nil {
		log.Printf("⚠️ Failed to get token chain for %s: %v", tokenID, err)
		return err
	}

	// The chainResponse is a map[string]interface{}, so we need to parse it
	// For now, let's just log it since the structure may vary
	log.Printf("🔗 Token chain data for %s: %v", tokenID, chainResponse)

	// TODO: Parse the chainResponse structure and create database entries
	// This will depend on the actual structure returned by your full node
	return nil
}

// syncTokenChains synchronizes token chain data for RBT tokens
func (s *SyncService) syncTokenChains() error {
	log.Println("🔗 Syncing token chains...")

	// Get all RBT tokens from database
	tokens, err := s.tokenQueries.GetTokensByType("RBT")
	if err != nil {
		return fmt.Errorf("failed to get RBT tokens: %v", err)
	}

	log.Printf("📊 Found %d RBT tokens to sync chains for", len(tokens))

	// Sync token chains in batches to avoid overwhelming the full node
	batchSize := 10
	syncedCount := 0
	errorCount := 0

	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}

		batch := tokens[i:end]
		log.Printf("🔄 Processing token chain batch %d-%d", i+1, end)

		for _, token := range batch {
			if err := s.syncSingleTokenChain(token.TokenID); err != nil {
				log.Printf("⚠️ Failed to sync chain for token %s: %v", token.TokenID, err)
				errorCount++
			} else {
				syncedCount++
			}
		}

		// Add a small delay between batches to be respectful to the full node
		if end < len(tokens) {
			time.Sleep(1 * time.Second)
		}
	}

	log.Printf("✅ Token chain sync completed: %d synced, %d errors", syncedCount, errorCount)
	return nil
}

// syncSingleTokenChain syncs the chain data for a single token
func (s *SyncService) syncSingleTokenChain(tokenID string) error {
	// Check if we already have chain data for this token
	existingChain, err := s.blockQueries.GetTokenChain(tokenID)
	if err == nil && len(existingChain) > 0 {
		// We already have chain data, skip for now
		// In a more sophisticated implementation, we'd check for new blocks
		log.Printf("⏭️ Token %s already has chain data (%d blocks), skipping", tokenID, len(existingChain))
		return nil
	}

	// Fetch token chain from full node
	chainResponse, err := s.rubixClient.GetRBTTokenchain(tokenID)
	if err != nil {
		return fmt.Errorf("failed to get token chain from full node: %v", err)
	}

	if !chainResponse.Status {
		return fmt.Errorf("full node returned error: %s", chainResponse.Message)
	}

	// Process and store chain data
	if len(chainResponse.TokenChainData) == 0 {
		log.Printf("⚠️ No chain data for token %s", tokenID)
		return nil
	}

	// Store each block in the chain
	for i, chainData := range chainResponse.TokenChainData {
		if err := s.storeChainBlock(tokenID, i, &chainData); err != nil {
			return fmt.Errorf("failed to store block %d for token %s: %v", i, tokenID, err)
		}
	}

	log.Printf("✅ Synced %d blocks for token %s", len(chainResponse.TokenChainData), tokenID)
	return nil
}

// storeChainBlock stores a single block from the token chain
func (s *SyncService) storeChainBlock(tokenID string, blockIndex int, chainData *model.TokenChainData) error {
	// Create a block record
	txnType := "genesis"
	if blockIndex > 0 {
		txnType = "transfer"
	}
	amount := 1.0 // Default RBT amount

	block := &database.Block{
		BlockID:       chainData.TCBlockHashKey,
		BlockHash:     chainData.TCBlockHashKey,
		PrevBlockHash: nil, // Extract from genesis block if needed
		SenderDID:     nil, // Extract from chain data if available
		ReceiverDID:   &chainData.TCTokenOwnerKey,
		TxnType:       &txnType,
		Amount:        &amount,
		TxnTime:       time.Now(), // Use current time for now
		Epoch:         nil,
		TimeTakenMs:   nil,
	}

	// Check if block already exists
	existingBlock, err := s.blockQueries.GetBlock(block.BlockID)
	if err == nil && existingBlock != nil {
		// Block already exists, skip
		return nil
	}

	// Create the block
	if err := s.blockQueries.CreateBlock(block); err != nil {
		return fmt.Errorf("failed to create block: %v", err)
	}

	// Create token chain entry
	tokenChain := &database.TokenChain{
		TokenID:     tokenID,
		BlockHeight: int64(blockIndex),
		BlockID:     block.BlockID,
	}

	if err := s.blockQueries.CreateTokenChain(tokenChain); err != nil {
		return fmt.Errorf("failed to create token chain entry: %v", err)
	}

	return nil
}

// syncBlocks synchronizes recent blocks/transactions
func (s *SyncService) syncBlocks() error {
	log.Println("🔄 Syncing recent blocks...")

	// Note: Implement when you have a full node API to get recent blocks
	// This would typically involve:
	// 1. Get latest blocks from full node
	// 2. Compare with database
	// 3. Insert new blocks
	// 4. Update existing blocks if needed

	log.Println("✅ Block sync completed")
	return nil
}

// updateDID updates or creates a DID record
func (s *SyncService) updateDID(didID string) error {
	if didID == "" {
		return nil
	}

	// Check if DID exists
	existingDID, err := s.didQueries.GetDID(didID)
	if err != nil {
		// DID doesn't exist, create new one
		newDID := &database.DID{
			DIDID:         didID,
			LastActive:    &[]time.Time{time.Now()}[0],
			TotalBalance:  0, // Calculate from tokens
			PledgedAmount: 0, // Calculate from pledges
		}

		if err := s.didQueries.CreateDID(newDID); err != nil {
			log.Printf("⚠️ Failed to create DID %s: %v", didID, err)
			return err
		}
		log.Printf("➕ Created DID: %s", didID)
	} else {
		// DID exists, update activity
		if err := s.didQueries.UpdateDIDActivity(existingDID.DIDID); err != nil {
			log.Printf("⚠️ Failed to update DID activity %s: %v", didID, err)
		}
	}

	return nil
}

// GetSyncStats returns current synchronization statistics
func (s *SyncService) GetSyncStats() (*SyncStats, error) {
	stats := &SyncStats{
		LastSyncTime:   time.Now(), // You might want to store this
		SyncInProgress: s.isRunning,
	}

	// Get token statistics from processed data
	tokenStats, err := database.GetTokenStats()
	if err == nil && len(tokenStats) > 0 {
		for _, stat := range tokenStats {
			stats.TotalTokens += stat.TotalTokens
		}
	}

	// Get recent transaction analytics from processed data
	recentStats, err := database.GetRecentTransactionStats(24) // Last 24 hours
	if err == nil {
		stats.TotalBlocks = recentStats.TxnCount
	}

	// Get DID count from processed data
	processingStats, err := s.processingService.GetProcessingStats()
	if err == nil {
		if didCount, ok := processingStats["total_dids"].(int64); ok {
			stats.TotalDIDs = didCount
		}
	}

	return stats, nil
}

// Helper functions

func determineTokenState(rbtData model.Token) string {
	// Implement logic to determine token state based on RBT data
	// This might depend on your specific business logic
	switch rbtData.TokenStatus {
	case 1:
		return "active"
	case 0:
		return "inactive"
	default:
		return "unknown"
	}
}

func determineFTTokenState(ftData model.FTToken) string {
	// Implement logic to determine FT token state
	switch ftData.TokenStatus {
	case 1:
		return "active"
	case 0:
		return "inactive"
	default:
		return "unknown"
	}
}

func parseTimeFromBlock(timeStr string) time.Time {
	// Parse time from block data
	// Adjust format based on your full node's time format
	if timeStr == "" {
		return time.Now()
	}

	// Try common formats
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	log.Printf("⚠️ Failed to parse time: %s, using current time", timeStr)
	return time.Now()
}

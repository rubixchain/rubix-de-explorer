package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"explorer-server/database"
)

// DatabaseTokensHandler handles requests for tokens from database
func DatabaseTokensHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	tokenType := r.URL.Query().Get("type")
	ownerDID := r.URL.Query().Get("owner")
	state := r.URL.Query().Get("state")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Set defaults
	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Search tokens in database
	tokens, err := database.SearchTokens(tokenType, ownerDID, state, limit, offset)
	if err != nil {
		http.Error(w, "Failed to retrieve tokens", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"count":  len(tokens),
		"data":   tokens,
		"query": map[string]interface{}{
			"type":   tokenType,
			"owner":  ownerDID,
			"state":  state,
			"limit":  limit,
			"offset": offset,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseBlocksHandler handles requests for blocks from database
func DatabaseBlocksHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	senderDID := r.URL.Query().Get("sender")
	receiverDID := r.URL.Query().Get("receiver")
	txnType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Set defaults
	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Search blocks in database
	blocks, err := database.SearchBlocks(senderDID, receiverDID, txnType, limit, offset)
	if err != nil {
		http.Error(w, "Failed to retrieve blocks", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"count":  len(blocks),
		"data":   blocks,
		"query": map[string]interface{}{
			"sender":   senderDID,
			"receiver": receiverDID,
			"type":     txnType,
			"limit":    limit,
			"offset":   offset,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseTokenByIDHandler handles requests for a specific token
func DatabaseTokenByIDHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("id")
	if tokenID == "" {
		http.Error(w, "Token ID is required", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	tokenQueries := database.NewTokenQueries(db)

	token, err := tokenQueries.GetToken(tokenID)
	if err != nil {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseTokenChainHandler handles requests for token chain data
func DatabaseTokenChainHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("id")
	if tokenID == "" {
		http.Error(w, "Token ID is required", http.StatusBadRequest)
		return
	}

	// Get token chain from database
	tokenChain, err := database.GetTokenChain(tokenID)
	if err != nil {
		http.Error(w, "Failed to retrieve token chain", http.StatusInternalServerError)
		return
	}

	// Get block details for each chain entry
	var chainWithBlocks []map[string]interface{}
	db := database.GetDB()
	blockQueries := database.NewBlockQueries(db)

	for _, chain := range tokenChain {
		block, err := blockQueries.GetBlock(chain.BlockID)
		if err != nil {
			continue // Skip blocks that can't be found
		}

		chainEntry := map[string]interface{}{
			"token_id":     chain.TokenID,
			"block_height": chain.BlockHeight,
			"block_id":     chain.BlockID,
			"block":        block,
		}
		chainWithBlocks = append(chainWithBlocks, chainEntry)
	}

	response := map[string]interface{}{
		"status":    "success",
		"token_id":  tokenID,
		"count":     len(chainWithBlocks),
		"chain":     chainWithBlocks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseBlockByIDHandler handles requests for a specific block
func DatabaseBlockByIDHandler(w http.ResponseWriter, r *http.Request) {
	blockID := r.URL.Query().Get("id")
	if blockID == "" {
		http.Error(w, "Block ID is required", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	blockQueries := database.NewBlockQueries(db)

	block, err := blockQueries.GetBlock(blockID)
	if err != nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	// Get associated tokens
	blockTokens, err := database.GetBlockTokens(blockID)
	if err != nil {
		// Log error but don't fail the request
		blockTokens = []*database.BlockToken{}
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   block,
		"tokens": blockTokens,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseDIDHandler handles requests for DID information
func DatabaseDIDHandler(w http.ResponseWriter, r *http.Request) {
	didID := r.URL.Query().Get("id")
	if didID == "" {
		http.Error(w, "DID ID is required", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	didQueries := database.NewDIDQueries(db)
	tokenQueries := database.NewTokenQueries(db)
	blockQueries := database.NewBlockQueries(db)

	// Get DID info
	did, err := didQueries.GetDID(didID)
	if err != nil {
		http.Error(w, "DID not found", http.StatusNotFound)
		return
	}

	// Get tokens owned by this DID
	tokens, err := tokenQueries.GetTokensByOwner(didID)
	if err != nil {
		tokens = []*database.Token{} // Empty slice if error
	}

	// Get recent transactions for this DID
	blocks, err := blockQueries.GetBlocksByDID(didID)
	if err != nil {
		blocks = []*database.Block{} // Empty slice if error
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   did,
		"tokens": tokens,
		"recent_transactions": blocks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseAnalyticsHandler handles requests for analytics data
func DatabaseAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	// Get token statistics
	tokenStats, err := database.GetTokenStats()
	if err != nil {
		http.Error(w, "Failed to retrieve token statistics", http.StatusInternalServerError)
		return
	}

	// Get recent transaction analytics (last 24 hours)
	recentTxns, err := database.GetRecentTransactionStats(24)
	if err != nil {
		recentTxns = &database.TxnAnalytics{} // Empty if error
	}

	// Get recent blocks
	db := database.GetDB()
	blockQueries := database.NewBlockQueries(db)
	recentBlocks, err := blockQueries.GetLatestBlocks(10)
	if err != nil {
		recentBlocks = []*database.Block{} // Empty if error
	}

	response := map[string]interface{}{
		"status": "success",
		"analytics": map[string]interface{}{
			"token_statistics":     tokenStats,
			"recent_transactions":  recentTxns,
			"latest_blocks":        recentBlocks,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DatabaseSearchHandler handles general search requests
func DatabaseSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	searchType := r.URL.Query().Get("type") // token, block, did, or all

	results := map[string]interface{}{
		"status": "success",
		"query":  query,
		"type":   searchType,
		"results": map[string]interface{}{},
	}

	// Search tokens if query looks like a token ID or if type is token/all
	if searchType == "token" || searchType == "all" || searchType == "" {
		db := database.GetDB()
		tokenQueries := database.NewTokenQueries(db)

		// Try exact match first
		if token, err := tokenQueries.GetToken(query); err == nil {
			results["results"].(map[string]interface{})["token"] = token
		}
	}

	// Search blocks if query looks like a block ID or if type is block/all
	if searchType == "block" || searchType == "all" || searchType == "" {
		db := database.GetDB()
		blockQueries := database.NewBlockQueries(db)

		// Try exact match first
		if block, err := blockQueries.GetBlock(query); err == nil {
			results["results"].(map[string]interface{})["block"] = block
		} else if block, err := blockQueries.GetBlockByHash(query); err == nil {
			results["results"].(map[string]interface{})["block"] = block
		}
	}

	// Search DIDs if query looks like a DID or if type is did/all
	if searchType == "did" || searchType == "all" || searchType == "" {
		db := database.GetDB()
		didQueries := database.NewDIDQueries(db)

		// Try exact match first
		if did, err := didQueries.GetDID(query); err == nil {
			results["results"].(map[string]interface{})["did"] = did
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// DatabaseInterfaceHandler provides a simple interface to view database contents
func DatabaseInterfaceHandler(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	// Get counts from all tables
	interface_data := map[string]interface{}{
		"status": "success",
		"database_info": map[string]interface{}{},
		"tables": map[string]interface{}{},
	}

	// Get table counts
	tables := []string{"tokens", "blocks", "token_chains", "dids", "validators", "pledges", "block_tokens", "token_stats", "txn_analytics"}

	for _, table := range tables {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		err := db.QueryRow(query).Scan(&count)
		if err != nil {
			interface_data["tables"].(map[string]interface{})[table] = map[string]interface{}{
				"count": 0,
				"error": err.Error(),
			}
		} else {
			interface_data["tables"].(map[string]interface{})[table] = map[string]interface{}{
				"count": count,
			}
		}
	}

	// Get recent data samples
	samples := map[string]interface{}{}

	// Sample tokens
	tokenRows, err := db.Query("SELECT token_id, token_type, current_owner, state, created_at FROM tokens ORDER BY created_at DESC LIMIT 5")
	if err == nil {
		defer tokenRows.Close()
		var tokenSamples []map[string]interface{}
		for tokenRows.Next() {
			var tokenID, tokenType, currentOwner, state string
			var createdAt time.Time
			err := tokenRows.Scan(&tokenID, &tokenType, &currentOwner, &state, &createdAt)
			if err == nil {
				tokenSamples = append(tokenSamples, map[string]interface{}{
					"token_id": tokenID,
					"token_type": tokenType,
					"current_owner": currentOwner,
					"state": state,
					"created_at": createdAt,
				})
			}
		}
		samples["recent_tokens"] = tokenSamples
	}

	// Sample DIDs
	didRows, err := db.Query("SELECT did_id, total_balance, pledged_amount, last_active FROM dids ORDER BY total_balance DESC LIMIT 5")
	if err == nil {
		defer didRows.Close()
		var didSamples []map[string]interface{}
		for didRows.Next() {
			var didID string
			var totalBalance, pledgedAmount float64
			var lastActive *time.Time
			err := didRows.Scan(&didID, &totalBalance, &pledgedAmount, &lastActive)
			if err == nil {
				didSamples = append(didSamples, map[string]interface{}{
					"did_id": didID,
					"total_balance": totalBalance,
					"pledged_amount": pledgedAmount,
					"last_active": lastActive,
				})
			}
		}
		samples["top_dids"] = didSamples
	}

	// Sample token stats
	statsRows, err := db.Query("SELECT token_type, total_tokens, active_tokens, pledged_tokens, last_updated FROM token_stats")
	if err == nil {
		defer statsRows.Close()
		var statsSamples []map[string]interface{}
		for statsRows.Next() {
			var tokenType string
			var totalTokens, activeTokens, pledgedTokens int64
			var lastUpdated time.Time
			err := statsRows.Scan(&tokenType, &totalTokens, &activeTokens, &pledgedTokens, &lastUpdated)
			if err == nil {
				statsSamples = append(statsSamples, map[string]interface{}{
					"token_type": tokenType,
					"total_tokens": totalTokens,
					"active_tokens": activeTokens,
					"pledged_tokens": pledgedTokens,
					"last_updated": lastUpdated,
				})
			}
		}
		samples["token_statistics"] = statsSamples
	}

	interface_data["samples"] = samples
	interface_data["database_info"] = map[string]interface{}{
		"connected": database.IsHealthy(),
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interface_data)
}
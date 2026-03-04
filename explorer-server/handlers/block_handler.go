package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"net/http"
	"strconv"
	"time"
)

// ============================================================================
//  READ-ONLY PUBLIC API HANDLERS
// ============================================================================

func GetTxnsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetTxnsCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"all_block_count": count})
}

func GetTransferBlockListHandler(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit := 10
	page := 1

	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	response, err := services.GetTransferBlocksList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetBlockInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	txnHash := r.URL.Query().Get("hash")

	data, err := services.GetTransferBlockInfoFromTxnID(txnHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func GetBlockInfoFromBlockHash(w http.ResponseWriter, r *http.Request) {
	blockHash := r.URL.Query().Get("hash")

	data, err := services.GetTransferBlockInfoFromBlockHash(blockHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func GetBurntTxnInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	txnHash := r.URL.Query().Get("hash")

	data, err := services.GetBurntBlockInfo(txnHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func GetBurntBlockList(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit, _ := strconv.Atoi(limitStr)
	page, _ := strconv.Atoi(pageStr)

	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}

	data, err := services.GetBurntBlockList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// ============================================================================
//  BLOCK UPDATE (High Priority) → Worker Pool
// ============================================================================
//  Legacy UpdateBlocksHandler removed (Publishing is now handled via PubSub)
// ============================================================================
//  Optional Debug Endpoint — Worker Pool Status
// ============================================================================

func QueueStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if services.GlobalTxnProcessor == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "Dynamic processor not initialized",
		})
		return
	}

	stats := services.GlobalTxnProcessor.GetStats()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"workers":      stats["workers"],
		"queue_length": stats["queue_length"],
		"queue_cap":    stats["queue_cap"],
	})
}

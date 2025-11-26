package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"log"
	"net/http"
	"strconv"
	"time"
)

// ============================================================================
//  READ-ONLY PUBLIC API HANDLERS (unchanged)
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

func UpdateBlocksHandler(w http.ResponseWriter, r *http.Request) {
	var block map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Println("📥 Received block update — queueing to high-priority worker")

	// Use the ORIGINAL, already-correct update path, just executed in the pool.
	ok := services.EnqueueBlockUpdateTask(func() {
		// This function already knows how to:
		//   - decode the fullnode block map
		//   - store into AllBlocks with correct hash / type / txn_id
		//   - fan out into RBT/FT/NFT/SC etc.
		services.UpdateBlocks(block)
	})

	if !ok {
		log.Println("⚠️ Worker queue full — processing block inline (fallback)")
		services.UpdateBlocks(block)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"queued","message":"Block update accepted"}`))
}

// ============================================================================
//  Optional Debug Endpoint — Worker Pool Status
// ============================================================================

func QueueStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := services.GetWorkerPoolStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"workers":      status.Workers,
		"queue_length": status.QueueLen,
		"queue_cap":    status.QueueCap,
		"load_factor":  status.LoadFactor,
	})
}

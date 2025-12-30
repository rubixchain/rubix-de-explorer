package handlers

import (
	"encoding/json"
	"explorer-server/model"
	"explorer-server/services"
	"fmt"
	"log"
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

func UpdateBlocksHandler(w http.ResponseWriter, r *http.Request) {
	var info model.IncomingBlockInfo

	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Println("📥 Received block update — queueing to high-priority worker")
	fmt.Println("Block is:", info)

	if info.TxnBlock == nil {
		log.Println("❌ Incoming block missing block_map")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"missing block_map"}`))
		return
	}

	okTask := services.EnqueueBlockUpdateTask(func() {
		services.UpdateBlocks(info.TxnBlock, &info)
	})

	if !okTask {
		log.Println("⚠️ Worker queue full — processing inline")
		services.UpdateBlocks(info.TxnBlock, &info)
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

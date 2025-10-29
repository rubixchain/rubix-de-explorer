package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"explorer-server/services"
)

func GetTxnsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetTxnsCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]int64{"all_block_count": count}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetTransferBlockListHandler(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params
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

	// Fetch data
	response, err := services.GetTransferBlocksList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}

}

func GetBlockInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params
	txnHash := r.URL.Query().Get("hash")

	// Fetch data
	response, err := services.GetTransferBlockInfoFromTxnID(txnHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBlockInfoFromBlockHash(w http.ResponseWriter, r *http.Request) {
	// Parse pagination params
	blockHash := r.URL.Query().Get("hash")

	// Fetch data
	response, err := services.GetTransferBlockInfoFromBlockHash(blockHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBurntTxnInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	
	txnkHash := r.URL.Query().Get("hash")

	// Fetch data using service
	data, err := services.GetBurntBlockInfo(txnkHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}


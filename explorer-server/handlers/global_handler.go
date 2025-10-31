package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func GetInfo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	var (
		data      interface{}
		err       error
		assetType string
	)

	// Determine logic based on ID prefix
	if strings.HasPrefix(id, "Qm") {
		// Fetch asset type from DB
		assetType, err = services.GetAssetType(id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch asset type: %v", err), http.StatusInternalServerError)
			return
		}

		switch assetType {
		case "NFT":
			data, err = services.GetNFTInfoFromNFTID(id)
		case "RBT":
			data, err = services.GetRBTInfoFromRBTID(id)
		case "FT":
			data, err = services.GetFTInfoFromFTID(id)
		case "SmartContract":
			data, err = services.GetSCInfoFromSCID(id)
		case "DID":
			data, err = services.GetDIDInfoFromDID(id)
		case "TransferBlock":
			data, err = services.GetTransferBlockInfoFromTxnID(id)
		default:
			http.Error(w, fmt.Sprintf("Unknown asset type for ID: %s", id), http.StatusBadRequest)
			return
		}

	} else if strings.HasPrefix(id, "bafy") {
		assetType = "DID"
		data, err = services.GetDIDInfoFromDID(id)
	} else {
		assetType = "TransferBlock"
		data, err = services.GetTransferBlockInfoFromTxnID(id)
	}

	// Handle any service error
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch info: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle empty data
	if data == nil {
		http.Error(w, fmt.Sprintf("No data found for ID: %s", id), http.StatusNotFound)
		return
	}

	// Send successful response
	response := map[string]interface{}{
		"id":   id,
		"type": assetType,
		"data": data,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetTokenChainFromTokenID(w http.ResponseWriter, r *http.Request) {
	var chainData map[string]interface{}

	tokenID := r.URL.Query().Get("token_id")
	if tokenID == "" {
		http.Error(w, "Missing 'token_id' parameter", http.StatusBadRequest)
		return
	}

	chainData, err := services.GetTokenChainFromTokenID(tokenID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch token chain: %v", err), http.StatusInternalServerError)
		return
	}

	if chainData == nil {
		http.Error(w, fmt.Sprintf("No chain data found for Token ID: %s", tokenID), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chainData); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetTokenBlocksFromTokenID(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	tokenID := r.URL.Query().Get("tokenID")
	if tokenID == "" {
		http.Error(w, "Missing 'token_id' parameter", http.StatusBadRequest)
		return
	}

	// Parse pagination parameters
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

	// Fetch token chain data with pagination
	chainData, totalBlocks, err := services.GetTokenBlocksFromTokenID(tokenID, page, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch token chain: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate total pages
	totalPages := (totalBlocks + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1 // handle edge case of empty chain
	}

	// Prepare standard response
	response := map[string]interface{}{
		"page":         page,
		"limit":        limit,
		"total_blocks": totalBlocks,
		"total_pages":  totalPages,
		"data":         chainData,
	}

	// Add message for empty data (out of range or no blocks)
	if len(chainData) == 0 {
		response["message"] = "No data available for this page."
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetSCBlockInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "Missing transaction hash", http.StatusBadRequest)
		return
	}

	var response interface{}

	scBlockInfo, err := services.GetSCBlockInfoFromTxnId(hash)
	if err != nil {
		http.Error(w, "Failed to fetch SC block info", http.StatusInternalServerError)
		return
	}
	response = scBlockInfo

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}

}

package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"net/http"
	"strings"
	"fmt"

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
	if strings.HasPrefix(id, "qem") {
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
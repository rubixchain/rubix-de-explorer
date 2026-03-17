package handlers

import (
	"encoding/json"
	"explorer-server/database"
	"explorer-server/database/models"
	"net/http"
)

// TokenType constants (aligned with database/operations/token_operations.go)
const (
	TokenTypeRBT int16 = 1
	TokenTypeFT  int16 = 2
	TokenTypeNFT int16 = 3
	TokenTypeSC  int16 = 4
)

// ==========================================
//   Statistics Handlers (Counts)
// ==========================================

// GetRBTCountHandler returns the total number of RBT tokens
func GetRBTCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", TokenTypeRBT).Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_rbt_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetFTCountHandler returns the total number of FT tokens
func GetFTCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", TokenTypeFT).Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_ft_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetNFTsCountHandler returns the total number of NFT tokens
func GetNFTsCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", TokenTypeNFT).Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_nft_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetSCsCountHandler returns the total number of Smart Contract tokens
func GetSCsCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.Token{}).Where("token_type = ?", TokenTypeSC).Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_sc_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetTxnsCountHandler returns the total number of successful transactions
func GetTxnsCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.Transactions{}).Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_transaction_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetDIDCountHandler returns the total number of unique DIDs with balances
func GetDIDCountHandler(w http.ResponseWriter, r *http.Request) {
	var count int64
	if err := database.ReadDB.Model(&models.DIDBalance{}).Distinct("did").Count(&count).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_did_count": count}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

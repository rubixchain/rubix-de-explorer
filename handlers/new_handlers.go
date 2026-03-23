package handlers

import (
	"encoding/json"
	"explorer-server/api"
	"explorer-server/database/models"
	"explorer-server/model"
	"net/http"
	"strconv"
)

// ==========================================
//   1. Search Handler
// ==========================================

// GetInfo routes search queries to the appropriate table based on format
func GetInfo(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("id")
	if query == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	result, err := api.GetSearchInfo(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ==========================================
//   2. Statistics Handlers (Counts)
// ==========================================

// GetRBTCountHandler returns the total number of RBT tokens
func GetRBTCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetRBTCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_rbt_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetFTCountHandler returns the total number of FT tokens
func GetFTCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetFTCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_ft_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetNFTsCountHandler returns the total number of NFT tokens
func GetNFTsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetNFTCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_nft_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSCsCountHandler returns the total number of Smart Contract tokens
func GetSCsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetSCCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_sc_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTxnsCountHandler returns the total number of successful transactions
func GetTxnsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetTxnsCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_transaction_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDIDCountHandler returns the total number of unique DIDs with balances
func GetDIDCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetDIDCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]int64{"all_did_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ==========================================
//   3. Lists and Holders Handlers
// ==========================================

// GetLatestTransactionsListHandler returns a paginated list of latest transactions
func GetLatestTransactionsListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	transactions, err := api.GetTransactionInfoList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// GetDIDHoldersListHandler returns DIDs with the most RBT balances
func GetDIDHoldersListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	balances, err := api.GetDIDHoldersList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balances)
}

// GetRBTListHandler returns a paginated list of RBT tokens
func GetRBTListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, err := api.GetRBTList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.Token{}
	}
	json.NewEncoder(w).Encode(data)
}

// GetFTGroupListHandler returns FT tokens grouped by name and creator
func GetFTGroupListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, err := api.GetFTGroupList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []model.FTGroup{}
	}
	json.NewEncoder(w).Encode(data)
}

// GetFTListByFTNameHandler returns all FT tokens for a specific group
func GetFTListByFTNameHandler(w http.ResponseWriter, r *http.Request) {
	ftName := r.URL.Query().Get("ftName")
	creatorDID := r.URL.Query().Get("creatorDID")
	limit, page := getPagination(r)

	data, err := api.GetFTListByFTName(ftName, creatorDID, limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.Token{}
	}
	json.NewEncoder(w).Encode(data)
}

// GetSCListHandler returns a paginated list of Smart Contract tokens
func GetSCListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, err := api.GetSCList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.Token{}
	}
	json.NewEncoder(w).Encode(data)
}

// ==========================================
//   4. Specific Info and History Handlers
// ==========================================

// GetTransactionInfoHandler returns details for a single transaction
func GetTransactionInfoHandler(w http.ResponseWriter, r *http.Request) {
	txnID := r.URL.Query().Get("transactionID")
	data, err := api.GetTransactionInfo(txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetTransactionInfoListHandler returns full TransactionInfo for a specific token
func GetTransactionInfoListHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("tokenID")
	limit, page := getPagination(r)
	data, err := api.GetTransactionInfoListByToken(tokenID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.TransactionInfo{}
	}
	json.NewEncoder(w).Encode(data)
}

// GetTransactionIDListHandler returns TokenChain records for a specific token
func GetTransactionIDListHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("tokenID")
	limit, page := getPagination(r)
	data, err := api.GetTransactionIDList(tokenID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.TokenChain{}
	}
	json.NewEncoder(w).Encode(data)
}

// GetTokenInfoHandler returns generic token details
func GetTokenInfoHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("tokenID")
	data, err := api.GetTokenInfo(tokenID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// -------------------------------------------------------------------
// Shared Helper Functions
// -------------------------------------------------------------------

func getPagination(r *http.Request) (int, int) {
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
	return limit, page
}

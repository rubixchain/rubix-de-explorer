package handlers

import (
	"encoding/json"
	"explorer-server/api"
	"explorer-server/database/models"
	"explorer-server/model"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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
	data, total, err := api.GetTransactionInfoList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.TransactionInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// HideMintTxnsHandler returns latest transactions excluding RBT mint transactions
// (where initiator == owner and only RBT tokens are involved).
func HideMintTxnsHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, total, err := api.GetTransactionInfoListNoMint(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.TransactionInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetDIDHoldersListHandler returns DIDs with the most RBT balances
func GetDIDHoldersListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, total, err := api.GetDIDHoldersList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.DIDBalance{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetRBTListHandler returns a paginated list of RBT tokens
func GetRBTListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, total, err := api.GetRBTList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.Token{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetFTGroupListHandler returns FT tokens grouped by name and creator
func GetFTGroupListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, total, err := api.GetFTGroupList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []model.FTGroup{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetFTListByFTNameHandler returns all FT tokens for a specific group
func GetFTListByFTNameHandler(w http.ResponseWriter, r *http.Request) {
	ftName := r.URL.Query().Get("ftName")
	creatorDID := r.URL.Query().Get("creatorDID")
	limit, page := getPagination(r)

	data, total, err := api.GetFTListByFTName(ftName, creatorDID, limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.Token{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetSCListHandler returns a paginated list of Smart Contract tokens
func GetSCListHandler(w http.ResponseWriter, r *http.Request) {
	limit, page := getPagination(r)
	data, total, err := api.GetSCList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.Token{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// ==========================================
//   DID Balance Handler
// ==========================================

// GetDIDBalanceHandler returns all balances for a specific DID
func GetDIDBalanceHandler(w http.ResponseWriter, r *http.Request) {
	did := r.URL.Query().Get("did")
	if did == "" {
		http.Error(w, `{"error":"did parameter is required"}`, http.StatusBadRequest)
		return
	}
	data, err := api.GetDIDBalance(did)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		data = []models.DIDBalance{}
	}
	json.NewEncoder(w).Encode(data)
}

// ==========================================
//   4. Specific Info and History Handlers
// ==========================================

// GetDAGTxnHandler returns an anchor transaction and its ancestors up to 100 levels deep.
// Path param: txnID. Optional query param: depth (default/max 100).
// For infinite scroll: pass the oldest transaction from the previous response as the new txnID.
func GetDAGTxnHandler(w http.ResponseWriter, r *http.Request) {
	txnID := mux.Vars(r)["txnID"]
	if txnID == "" {
		http.Error(w, "Missing txnID", http.StatusBadRequest)
		return
	}
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	data, err := api.GetDAGFromTxn(txnID, depth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetDAGTransactionsHandler builds a DAG of up to 500 nodes.
// Fetches latest transactions in batches of 50, walks ancestors up to depth 7,
// and keeps adding batches until 500 total nodes are collected.
func GetDAGTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	data, err := api.GetDAGTransactions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetTransactionInfoHandler returns details for a single transaction
func GetTxnsByDIDHandler(w http.ResponseWriter, r *http.Request) {
	did := r.URL.Query().Get("did")
	if did == "" {
		http.Error(w, `{"error":"did parameter is required"}`, http.StatusBadRequest)
		return
	}
	limit, page := getPagination(r)
	data, total, err := api.GetTxnsByDID(did, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.TransactionInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

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
	data, total, err := api.GetTransactionInfoListByToken(tokenID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.TransactionInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
}

// GetTransactionIDListHandler returns TokenChain records for a specific token
func GetTransactionIDListHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("tokenID")
	limit, page := getPagination(r)
	data, total, err := api.GetTransactionIDList(tokenID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []models.TokenChain{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.NewPaginated(data, total, page, limit))
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

// GetRBTSuggestionsHandler returns autocomplete suggestions for RBT token IDs.
// Query params: query (required), limit (optional)
func GetRBTSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("query")
	if prefix == "" {
		http.Error(w, "Missing 'query' parameter", http.StatusBadRequest)
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	suggestions, err := api.SearchRBTSuggestions(prefix, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if suggestions == nil {
		suggestions = []model.RBTSuggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

// GetRBTInfoHandler returns owner and value details for a single RBT token.
// Query params: tokenId (required)
func GetRBTInfoHandler(w http.ResponseWriter, r *http.Request) {
	tokenID := r.URL.Query().Get("tokenId")
	if tokenID == "" {
		http.Error(w, "Missing 'tokenId' parameter", http.StatusBadRequest)
		return
	}
	info, err := api.GetRBTInfo(tokenID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// GetFTInfoHandler returns aggregate details for a specific FT.
// Query params: ftName (required), creatorDID (required)
func GetFTInfoHandler(w http.ResponseWriter, r *http.Request) {
	ftName := r.URL.Query().Get("ftName")
	creatorDID := r.URL.Query().Get("creatorDID")
	if ftName == "" || creatorDID == "" {
		http.Error(w, "Missing required parameters: ftName, creatorDID", http.StatusBadRequest)
		return
	}
	info, err := api.GetFTInfo(ftName, creatorDID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// GetFTTopHoldersHandler returns the top holders of a specific FT, paginated.
// Query params: ftName (required), creatorDID (required), limit, page
func GetFTTopHoldersHandler(w http.ResponseWriter, r *http.Request) {
	ftName := r.URL.Query().Get("ftName")
	creatorDID := r.URL.Query().Get("creatorDID")
	if ftName == "" || creatorDID == "" {
		http.Error(w, "Missing required parameters: ftName, creatorDID", http.StatusBadRequest)
		return
	}
	limit, page := getPagination(r)
	data, err := api.GetFTTopHolders(ftName, creatorDID, limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetFTSuggestionsHandler returns autocomplete suggestions for FT names.
// Query param: query (required), limit (optional, default 10, max 20)
// Example: GET /api/search-ft-suggestions?query=app&limit=10
// Response: [{"ft_name":"apple","creator_did":"bafybm..."},...]
func GetFTSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("query")
	if prefix == "" {
		http.Error(w, "Missing 'query' parameter", http.StatusBadRequest)
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	suggestions, err := api.SearchFTSuggestions(prefix, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if suggestions == nil {
		suggestions = []model.FTSuggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
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

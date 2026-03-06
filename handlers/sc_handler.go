package handlers

import (
	"encoding/json"
	"net/http"
    "strconv"
	"explorer-server/api"
	
)


func GetSCsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := api.GetSCCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]int64{"all_sc_count": count}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetSmartContractInfoFromSCID(w http.ResponseWriter, r *http.Request) {
	scid := r.URL.Query().Get("scid")
	println("SCID:", scid)
	scInfo, err := api.GetSCInfoFromSCID(scid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"sc_info": scInfo}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetSCListHandler(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	data, err := api.GetSCList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetSCBlockList(w http.ResponseWriter, r *http.Request) {
// Parse query parameters: ?limit=10&page=2
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10 // default limit
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default page
	}

	// Fetch data using service
	data, err := api.GetSCBlockList(limit, page)
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





package handlers

import (
	"encoding/json"
	"explorer-server/api"
	"net/http"
	"strconv"
)

// func GetRBTCountHandler(w http.ResponseWriter, r *http.Request) {
// 	count, err := api.GetRBTCount()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
//
// 	response := map[string]int64{"all_rbt_count": count}
//
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

func GetRBTInfoFromRBTID(w http.ResponseWriter, r *http.Request) {
	rbtId := r.URL.Query().Get("rbtid")
	println("RBT ID:", rbtId)
	rbtInfo, err := api.GetRBTInfoFromRBTID(rbtId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"rbt_info": rbtInfo}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetRBTListHandler(w http.ResponseWriter, r *http.Request) {
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
	data, err := api.GetRBTList(limit, page)
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

// func GetRBTInfoFromRBTIDHandler(w http.ResponseWriter, r *http.Request)  {
// 	rbtId:= r.URL.Query().Get("rbtId")
// 	println("RBT ID:", rbtId)
// 	count, err := api.GetRBTInfoFromRBTID()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	response := map[string]int64{"all_rbt_count": count}

// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

// func GetRBTListHandler(w http.ResponseWriter, r *http.Request) {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		data, err := service.FetchFreeRBTs()
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		if err := json.NewEncoder(w).Encode(data); err != nil {
// 			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 		}
// 	}
// }

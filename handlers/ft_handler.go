package handlers

import (
	"encoding/json"
	"explorer-server/api"
	"net/http"
)

// func GetFTCountHandler(w http.ResponseWriter, r *http.Request) {
// 	count, err := api.GetFTCount()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
//
// 	response := map[string]int64{"all_ft_count": count}
//
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

func GetFTInfoFromFTID(w http.ResponseWriter, r *http.Request) {
	ftId := r.URL.Query().Get("ftid")
	println("FT ID:", ftId)
	ftInfo, err := api.GetFTInfoFromFTID(ftId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"ft_info": ftInfo}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}


func GetFtHoldingList(w http.ResponseWriter, r *http.Request) {
   did := r.URL.Query().Get("did")

   ftInfo, err := api.GetFTListFromDID(did)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"ft_info": ftInfo}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

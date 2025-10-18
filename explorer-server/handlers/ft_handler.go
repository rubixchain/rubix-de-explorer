package handlers

import (
	"encoding/json"
	"net/http"
	"explorer-server/services"
)

func GetFTCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetFTCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]int64{"all_ft_count": count}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetFTInfoFromFTID(w http.ResponseWriter, r *http.Request) {
	ftId := r.URL.Query().Get("ftId")
	println("FT ID:", ftId)
	ftInfo, err := services.GetFTInfoFromFTID(ftId)
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


// func GetRBTInfoFromRBTIDHandler(w http.ResponseWriter, r *http.Request)  {
// 	rbtId:= r.URL.Query().Get("rbtId")
// 	println("RBT ID:", rbtId)
// 	count, err := services.GetRBTInfoFromRBTID()
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

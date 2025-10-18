package handlers

import (
	"encoding/json"
	"net/http"

	"explorer-server/services"
)


func GetNFTsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetNFTCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]int64{"all_nft_count": count}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}

}

func GetNFTInfoFromNFTID(w http.ResponseWriter, r *http.Request) {
	nftId := r.URL.Query().Get("nftId")
	println("NFT ID:", nftId)
	nftInfo, err := services.GetNFTInfoFromNFTID(nftId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"nft_info": nftInfo}

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



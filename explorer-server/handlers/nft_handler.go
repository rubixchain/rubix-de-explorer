package handlers

// import (
// 	"encoding/json"
// 	"net/http"

// 	"explorer-server/services"
// )

// func GetNFTCountHandler(service *services.RBTService) http.HandlerFunc {
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

// func GetNFTInfoFromNFTIDHandler(service *services.RBTService) http.HandlerFunc {
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

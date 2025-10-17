package handlers

// import (
// 	"encoding/json"
// 	"net/http"

// 	"explorer-server/services"
// )

// // return did_count as response
// func GetDIDCountHandler(service *services.FTService) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		result, err := service.GetFTTokens()
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(result)
// 	}
// }

// // there will be DID param send in the rquest query
// func GetDIDInfoFromDIDHandler(service *services.FTService) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		result, err := service.GetFTTokens()
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(result)
// 	}
// }


// // there will be limit and page params send in the rquest query
// func GetDIDListHavingHighTokenCountHandler(service *services.FTService) http.HandlerFunc {

// }
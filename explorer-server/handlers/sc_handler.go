package handlers

import (
	"encoding/json"
	"net/http"

	"explorer-server/services"
)


func GetSCsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetSCCount()
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
	scInfo, err := services.GetSCInfoFromSCID(scid)
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


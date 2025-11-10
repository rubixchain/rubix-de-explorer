package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"log"
	"net/http"
)

func UpdateTokensHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Table     string      `json:"table"`
		Data      interface{} `json:"data"`
		Operation string      `json:"operation"` // NEW: CREATE, UPDATE, DELETE
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("✅ Received token %s from fullnode (table: %s)", payload.Operation, payload.Table)
	services.UpdateTokens(payload.Table, payload.Data, payload.Operation)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Token update processed successfully"}`))
}

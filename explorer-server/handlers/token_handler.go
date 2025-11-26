package handlers

import (
	"encoding/json"
	"explorer-server/services"
	"log"
	"net/http"
)

// TOKEN UPDATE (High Priority) → Worker Pool
func UpdateTokensHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Table     string      `json:"table"`
		Data      interface{} `json:"data"`
		Operation string      `json:"operation"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Received token %s for table %s — queueing", payload.Operation, payload.Table)

	ok := services.EnqueueTokenUpdateTask(payload.Table, payload.Data, payload.Operation)
	if !ok {
		log.Println("⚠️ Token worker queue full — executing token update inline")
		services.UpdateTokens(payload.Table, payload.Data, payload.Operation)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"queued","message":"Token update accepted"}`))
}

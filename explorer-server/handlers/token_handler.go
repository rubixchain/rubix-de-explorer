package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// UpdateTokensHandler - ASYNC (queues instead of blocking)
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

	log.Printf("📝 Received token %s from fullnode (table: %s), queueing for processing", payload.Operation, payload.Table)

	queue := GetQueue()
	if err := queue.EnqueueTokenUpdate(payload.Table, payload.Data, payload.Operation); err != nil {
		http.Error(w, fmt.Sprintf("Queue error: %v", err), http.StatusServiceUnavailable)
		log.Printf("❌ Failed to enqueue token: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "queued", "message": "Token update accepted for processing"}`))
}

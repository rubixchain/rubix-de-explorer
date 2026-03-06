package handlers

// import (
// 	"encoding/json"
// 	"net/http"

// 	"explorer-server/database"
// )

// // HealthHandler handles health check requests
// func HealthHandler(w http.ResponseWriter, r *http.Request) {
// 	health := database.DatabaseHealth{
// 		IsConnected: database.IsHealthy(),
// 		Status:      "ok",
// 		Message:     "Database connection is healthy",
// 	}

// 	if !health.IsConnected {
// 		health.Status = "error"
// 		health.Message = "Database connection failed"
// 		w.WriteHeader(http.StatusServiceUnavailable)
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(health)
// }

// // DatabaseStatsHandler handles database statistics requests
// func DatabaseStatsHandler(w http.ResponseWriter, r *http.Request) {
// 	if !database.IsHealthy() {
// 		http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
// 		return
// 	}

// 	stats, err := database.GetTokenStats()
// 	if err != nil {
// 		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"status":      "success",
// 		"token_stats": stats,
// 	})
// }
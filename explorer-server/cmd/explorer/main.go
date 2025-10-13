package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"explorer-server/client"
	"explorer-server/config"
	"explorer-server/database"
	"explorer-server/router"
	"explorer-server/services"

	"github.com/rs/cors"
)

func main() {
	log.Println("🚀 Starting Explorer Server...")

	// Initialize PostgreSQL database
	log.Println("📦 Initializing database...")
	if err := database.Initialize(); err != nil {
		log.Fatalf("❌ failed to start PostgreSQL server %v", err)
	}

	// Initialize Rubix client for data synchronization
	log.Println("🔗 Initializing Rubix client...")
	rubixClient := client.NewRubixClient(config.RubixNodeURL)

	// Initialize sync service
	log.Println("🔄 Initializing sync service...")
	syncService := services.NewSyncService(rubixClient)

	// Start periodic sync (every 5 minutes)
	syncInterval := 5 * time.Minute
	syncService.StartPeriodicSync(syncInterval)
	log.Printf("⏰ Periodic sync started (every %v)", syncInterval)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Received shutdown signal...")

		log.Println("🔄 Stopping sync service...")
		syncService.StopPeriodicSync()

		if err := database.Close(); err != nil {
			log.Printf("❌ Error during database shutdown: %v", err)
		}

		log.Println("👋 Server shutdown complete")
		os.Exit(0)
	}()

	// Setup router
	r := router.NewRouter()

	// Add sync monitoring route
	r.HandleFunc("/api/sync/status", func(w http.ResponseWriter, req *http.Request) {
		stats, err := syncService.GetSyncStats()
		if err != nil {
			http.Error(w, "Failed to get sync statistics", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"status": "success",
			"sync":   stats,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}).Methods(http.MethodGet)

	// Wrap with CORS
	handler := cors.Default().Handler(r)

	// ✅ Use port from config instead of hardcoded value
	port := config.ExplorerPort
	if port == "" {
		port = "8081" // fallback if not provided
	}

	log.Printf("✅ Explorer server running on :%s with CORS enabled", port)
	log.Println("📊 Database ready for connections")

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), handler))
}

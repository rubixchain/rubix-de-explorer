package main

import (
	"encoding/json"
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
		log.Fatalf("❌ Failed to initialize database: %v", err)
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

	// Set sync service for handlers
	log.Println("🔧 Configuring handlers...")
	// We'll need to update the router to pass the sync service

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Received shutdown signal...")

		// Stop sync service
		log.Println("🔄 Stopping sync service...")
		syncService.StopPeriodicSync()

		// Close database connections and stop PostgreSQL
		if err := database.Close(); err != nil {
			log.Printf("❌ Error during database shutdown: %v", err)
		}

		log.Println("👋 Server shutdown complete")
		os.Exit(0)
	}()

	// Use the mux router from router.go
	r := router.NewRouter()

	// Add sync monitoring routes
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

	log.Println("✅ Explorer server running on :8081 with CORS enabled")
	log.Println("📊 Database ready for connections")
	log.Fatal(http.ListenAndServe(":8081", handler))
}

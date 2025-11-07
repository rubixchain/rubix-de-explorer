package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"explorer-server/database"
	"explorer-server/router"
	"explorer-server/services"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	startTime := time.Now()
	log.Printf("Starting Explorer Server at %s\n", startTime.Format(time.RFC1123))

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default values")
	} else {
		log.Println(".env file loaded successfully")
	}

	// Initialize PostgreSQL
	log.Println("Connecting to PostgreSQL...")
	database.ConnectAndMigrate(false)
	log.Println("PostgreSQL connected and migrated")

	// Setup router
	r := router.NewRouter()
	handler := cors.Default().Handler(r)

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: handler,
	}

	// Start server IMMEDIATELY in goroutine
	go func() {
		serverStart := time.Now()
		log.Printf("Explorer server STARTED on port :%s at %s\n", port, serverStart.Format(time.RFC1123))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start initial sync IN BACKGROUND (non-blocking)
	go func() {
		time.Sleep(2 * time.Second) // Let server boot first
		syncData("Initial Sync (Startup)")
	}()

	// Start periodic sync scheduler
	go startPeriodicSync()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownStart := time.Now()
	log.Printf("Shutdown signal received at %s\n", shutdownStart.Format(time.RFC1123))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("HTTP server stopped gracefully")
	}

	database.CloseDB()
	log.Printf("Database connection closed\n")
	log.Printf("Server shutdown complete in %s\n", time.Since(shutdownStart).Round(time.Millisecond))
	log.Printf("Total uptime: %s\n", time.Since(startTime).Round(time.Second))
}

// startPeriodicSync runs syncData every 3 hours in background
func startPeriodicSync() {
	log.Println("Periodic sync scheduler started (every 3 hours)")

	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()

	for t := range ticker.C {
		go func(triggerTime time.Time) {
			log.Printf("Scheduled sync triggered at %s\n", triggerTime.Format(time.RFC1123))
			syncData("Scheduled Sync")
		}(t)
	}
}

// syncData performs all data fetches with detailed timing and error logging
func syncData(syncType string) {
	syncStart := time.Now()
	log.Printf("=== %s STARTED at %s ===\n", syncType, syncStart.Format(time.RFC1123))

	var errCount int

	// Helper to log each fetch with timing
	fetchWithLog := func(name string, fn func() error) {
		start := time.Now()
		err := fn()
		duration := time.Since(start)

		if err != nil {
			log.Printf("  [Failed] %s | Duration: %s | Error: %v\n", name, duration.Round(time.Millisecond), err)
			errCount++
		} else {
			log.Printf("  [Success] %s | Duration: %s\n", name, duration.Round(time.Millisecond))
		}
	}

	// Run all syncs (still sequential, but in background)
	fetchWithLog("FetchAndStoreAllRBTsFromFullNodeDB", services.FetchAndStoreAllRBTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllFTsFromFullNodeDB", services.FetchAndStoreAllFTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllNFTsFromFullNodeDB", services.FetchAndStoreAllNFTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllSCsFromFullNodeDB", services.FetchAndStoreAllSCsFromFullNodeDB)
	fetchWithLog("FetchAllTokenChainFromFullNode", services.FetchAllTokenChainFromFullNode)

	totalDuration := time.Since(syncStart)
	log.Printf("=== %s COMPLETED in %s | Failed: %d ===\n",
		syncType, totalDuration.Round(time.Millisecond), errCount)

	if errCount == 0 {
		log.Println("All data synced successfully!")
	} else {
		log.Printf("Sync completed with %d error(s). Check logs above.\n", errCount)
	}
}

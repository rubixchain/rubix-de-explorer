package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"explorer-server/database"
	"explorer-server/handlers"
	"explorer-server/router"
	"explorer-server/services"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	startTime := time.Now()

	// Get total CPU cores
	totalCores := runtime.NumCPU()
	log.Printf("Detected %d CPU cores\n", totalCores)

	// Reserve 1 core for server, rest for syncing
	syncCores := totalCores - 1
	if syncCores < 1 {
		syncCores = 1 // Minimum 1 core for syncing
	}

	// Set GOMAXPROCS to use all cores
	runtime.GOMAXPROCS(totalCores)
	log.Printf("Using %d cores total: 1 for server, %d for data syncing\n", totalCores, syncCores)
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

	// Initialize notification queue BEFORE starting server
	notificationQueue := handlers.InitNotificationQueue(8)
	log.Println("✅ Notification queue initialized with 8 workers")

	// Setup router
	r := router.NewRouter()
	handler := cors.Default().Handler(r)

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:           "0.0.0.0:" + port,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server IMMEDIATELY in goroutine
	go func() {
		serverStart := time.Now()
		log.Printf("Explorer server STARTED on port :%s at %s\n", port, serverStart.Format(time.RFC1123))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start initial sync IN BACKGROUND (non-blocking) with parallel processing
	go func() {
		time.Sleep(2 * time.Second) // Let server boot first
		syncData("Initial Sync (Startup)", syncCores)
	}()

	// Start periodic sync scheduler
	go startPeriodicSync(syncCores)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownStart := time.Now()
	log.Printf("Shutdown signal received at %s\n", shutdownStart.Format(time.RFC1123))

	// Shutdown sequence: HTTP -> Queue -> Database

	// 1. Stop accepting new HTTP requests (10 second timeout)
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer httpCancel()

	if err := srv.Shutdown(httpCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("✅ HTTP server stopped gracefully")
	}

	// 2. Drain notification queue (60 second timeout for pending tasks)
	log.Println("🛑 Draining notification queue...")
	queueCtx, queueCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer queueCancel()
	notificationQueue.Shutdown(queueCtx)
	log.Println("✅ Notification queue drained")

	// 3. Close database connections
	database.CloseDB()
	log.Println("✅ Database connection closed")

	log.Printf("Server shutdown complete in %s\n", time.Since(shutdownStart).Round(time.Millisecond))
	log.Printf("Total uptime: %s\n", time.Since(startTime).Round(time.Second))
}

// startPeriodicSync runs syncData every 12 hours in background
func startPeriodicSync(maxWorkers int) {
	log.Println("Periodic sync scheduler started (every 12 hours)")

	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for t := range ticker.C {
		go func(triggerTime time.Time) {
			log.Printf("Scheduled sync triggered at %s\n", triggerTime.Format(time.RFC1123))
			syncData("Scheduled Sync", maxWorkers)
		}(t)
	}
}

// syncData performs all data fetches in parallel using available cores
func syncData(syncType string, maxWorkers int) {
	syncStart := time.Now()
	log.Printf("=== %s STARTED at %s (using %d workers) ===\n", syncType, syncStart.Format(time.RFC1123), maxWorkers)

	var errCount int
	var mu sync.Mutex // Protect errCount
	var wg sync.WaitGroup

	// Semaphore to limit concurrent workers
	semaphore := make(chan struct{}, maxWorkers)

	// Helper to log each fetch with timing (parallel with worker limit)
	fetchWithLog := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore (blocks if maxWorkers already running)
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			start := time.Now()
			err := fn()
			duration := time.Since(start)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("  [Failed] %s | Duration: %s | Error: %v\n", name, duration.Round(time.Millisecond), err)
				errCount++
			} else {
				log.Printf("  [Success] %s | Duration: %s\n", name, duration.Round(time.Millisecond))
			}
		}()
	}

	// Run all syncs IN PARALLEL (limited by maxWorkers)
	fetchWithLog("FetchAndStoreAllRBTsFromFullNodeDB", services.FetchAndStoreAllRBTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllFTsFromFullNodeDB", services.FetchAndStoreAllFTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllNFTsFromFullNodeDB", services.FetchAndStoreAllNFTsFromFullNodeDB)
	fetchWithLog("FetchAndStoreAllSCsFromFullNodeDB", services.FetchAndStoreAllSCsFromFullNodeDB)
	fetchWithLog("FetchAllTokenChainFromFullNode", services.FetchAllTokenChainFromFullNode)

	// Wait for all fetches to complete
	wg.Wait()

	totalDuration := time.Since(syncStart)
	log.Printf("=== %s COMPLETED in %s | Failed: %d ===\n",
		syncType, totalDuration.Round(time.Millisecond), errCount)

	if errCount == 0 {
		log.Println("✅ All data synced successfully!")
	} else {
		log.Printf("⚠️ Sync completed with %d error(s). Check logs above.\n", errCount)
	}
}

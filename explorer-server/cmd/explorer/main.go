package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
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

	// Detect CPU cores
	totalCores := runtime.NumCPU()
	log.Printf("Detected %d CPU cores\n", totalCores)

	// Reserve 1 core for HTTP server, rest can be used for background workers
	syncCores := totalCores - 1
	if syncCores < 1 {
		syncCores = 1
	}

	// Use all cores
	runtime.GOMAXPROCS(totalCores)
	log.Printf("Using %d cores total: 1 for server, %d for background work\n", totalCores, syncCores)
	log.Printf("Starting Explorer Server at %s\n", startTime.Format(time.RFC1123))

	// Load .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default values")
	} else {
		log.Println(".env file loaded successfully")
	}

	// Initialize PostgreSQL
	log.Println("Connecting to PostgreSQL...")
	database.ConnectAndMigrate(false)
	log.Println("PostgreSQL connected and migrated")

	// --------------------------------------------------
	// Initialize worker pools (block / token / sync)
	// --------------------------------------------------
	services.InitWorkerPools(totalCores)
	log.Println("✅ Worker pools initialized")

	// --------------------------------------------------
	// Start continuous background sync (Option C)
	// --------------------------------------------------
	// This uses the dedicated sync worker pool internally.
	// services.StartContinuousSync(syncCores)

	// --------------------------------------------------
	// HTTP router + CORS
	// --------------------------------------------------
	r := router.NewRouter()
	handler := cors.Default().Handler(r)

	// Port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	// HTTP server
	srv := &http.Server{
		Addr:           "0.0.0.0:" + port,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start HTTP server
	go func() {
		serverStart := time.Now()
		log.Printf("Explorer server STARTED on port :%s at %s\n", port, serverStart.Format(time.RFC1123))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// --------------------------------------------------
	// Graceful shutdown (HTTP + DB)
	// --------------------------------------------------
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownStart := time.Now()
	log.Printf("Shutdown signal received at %s\n", shutdownStart.Format(time.RFC1123))

	// 1) Stop accepting new HTTP requests
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer httpCancel()

	if err := srv.Shutdown(httpCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("✅ HTTP server stopped gracefully")
	}

	// 2) Close database connection
	database.CloseDB()
	log.Println("✅ Database connection closed")

	log.Printf("Server shutdown complete in %s\n", time.Since(shutdownStart).Round(time.Millisecond))
	log.Printf("Total uptime: %s\n", time.Since(startTime).Round(time.Second))
}

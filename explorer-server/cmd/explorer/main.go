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

	// Detect CPU cores and initialize worker pool
	totalCores := runtime.NumCPU()
	runtime.GOMAXPROCS(totalCores)
	services.InitWorkerPools(totalCores)

	// Load .env if present
	if err := godotenv.Load(); err == nil {
		log.Println("✅ Environment configuration loaded")
	}

	// Initialize PostgreSQL
	database.ConnectAndMigrate(false)
	log.Printf("✅ Explorer Server initialized with %d cores\n", totalCores)

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
		log.Printf("🚀 Explorer Server listening on port :%s\n", port)
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

package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"explorer-server/database"
	"explorer-server/pubsub"
	"explorer-server/router"
	"explorer-server/services"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// CLI Flags
	testNet := flag.Bool("testnet", false, "Connect to Rubix TestNet (default: MainNet)")
	flag.Parse()

	startTime := time.Now()

	// Log which network we're connecting to
	if *testNet {
		log.Println("🌐 Network: TestNet")
	} else {
		log.Println("🌐 Network: MainNet")
	}

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
	// Initialize IPFS PubSub Listener & Daemon
	// --------------------------------------------------
	ipfsManager := services.NewIPFSManager()

	if err := ipfsManager.EnsureInitialized(*testNet); err != nil {
		log.Fatalf("❌ Failed to initialize IPFS node: %v\n", err)
	}

	if err := ipfsManager.StartDaemon(); err != nil {
		log.Fatalf("❌ Failed to start IPFS daemon: %v\n", err)
	}

	ipfsHost := os.Getenv("IPFS_HOST")
	if ipfsHost == "" {
		ipfsHost = "localhost:5001"
	}

	psClient, err := pubsub.NewPubSub(ipfsHost)
	if err != nil {
		log.Printf("⚠️ Failed to initialize PubSub client: %v\n", err)
	} else {
		topic := "rubix_txns" // Same topic as regular nodes publish to
		err = psClient.SubscribeTopic(topic, services.TxnCallBack)
		if err != nil {
			log.Printf("⚠️ Failed to subscribe to PubSub topic %s: %v\n", topic, err)
		}
	}

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

	// 2) Stop IPFS Daemon
	ipfsManager.Stop()
	log.Println("✅ IPFS Daemon stopped gracefully")

	// 3) Close database connection
	database.CloseDB()
	log.Println("✅ Database connection closed")

	log.Printf("Server shutdown complete in %s\n", time.Since(shutdownStart).Round(time.Millisecond))
	log.Printf("Total uptime: %s\n", time.Since(startTime).Round(time.Second))
}

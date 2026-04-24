package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/database/operations"
	"explorer-server/ipfs"
	"explorer-server/processor"
	"explorer-server/pubsub"
	"explorer-server/router"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// 1. Setup Logging (Unique File per Restart + Console)
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		_ = os.Mkdir(logDir, 0755)
	}
	
	// Create a unique filename based on the current time (e.g., explorer_2026-01-02_15-04-05.log)
	startTime := time.Now()
	logFileName := fmt.Sprintf("explorer_%s.log", startTime.Format("2006-01-02_15-04-05"))
	
	logFile, err := os.OpenFile(filepath.Join(logDir, logFileName), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: Could not open log file: %v", err)
	} else {
		// MultiWriter redirects log output to both Stdout and the log file
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
	}

	log.Println("----------------------------------------------------------------")
	log.Printf(">>> EXPLORER RESTART - %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Println("----------------------------------------------------------------")

	// CLI Flags
	testNet := flag.Bool("testnet", false, "Connect to Rubix TestNet (default: MainNet)")
	swarmKeyPath := flag.String("swarmkey", "", "Path to a custom swarm.key file (overrides built-in keys)")
	publishDummy := flag.Bool("publish-dummy", false, "TODO: DELETE LATER - Publish a dummy transaction for testing")
	flag.Parse()

	startTime = time.Now()

	// Log which network we're connecting to
	if *swarmKeyPath != "" {
		log.Printf("Network: Custom (swarm key: %s)", *swarmKeyPath)
	} else if *testNet {
		log.Println("Network: TestNet")
	} else {
		log.Println("Network: MainNet")
	}

	// Detect CPU cores
	totalCores := runtime.NumCPU()
	runtime.GOMAXPROCS(totalCores)

	// Load .env if present
	if err := godotenv.Load(); err == nil {
		log.Println("Environment configuration loaded")
	}

	// Initialize PostgreSQL
	database.ConnectAndMigrate(false)
	log.Println("Database connection established and migrated")

	// One-time pledged balance sync
	log.Println("Migration: Starting one-time PledgedBalance synchronization...")
	if err := operations.SyncPledgedBalances(database.WriteDB); err != nil {
		log.Printf("Migration Warning: Failed to sync pledged balances: %v\n", err)
	} else {
		log.Println("Migration: PledgedBalance synchronization complete")
	}

	log.Printf("Explorer Server initialized with %d cores\n", totalCores)

	// --------------------------------------------------
	// Initialize IPFS PubSub Listener & Daemon
	// --------------------------------------------------

	// Initialize the Dynamic Worker Pool for PubSub messages
	processor.InitDynamicWorkerPool()

	ipfsManager := ipfs.NewIPFSManager()

	if err := ipfsManager.EnsureInitialized(*testNet, *swarmKeyPath); err != nil {
		log.Fatalf("Failed to initialize IPFS node: %v\n", err)
	}

	if err := ipfsManager.StartDaemon(); err != nil {
		log.Fatalf("Failed to start IPFS daemon: %v\n", err)
	}

	ipfsHost := os.Getenv("IPFS_HOST")
	if ipfsHost == "" {
		ipfsHost = "localhost:5001"
	}

	psClient, err := pubsub.NewPubSub(ipfsHost)
	if err != nil {
		log.Printf("Warning: Failed to initialize PubSub client: %v\n", err)
	} else {
		topicTxn := models.Event_RubixTxns
		err = psClient.SubscribeTopic(topicTxn, processor.TxnCallBack)
		if err != nil {
			log.Printf("Warning: Failed to subscribe to PubSub topic %s: %v\n", topicTxn, err)
		}

		topicDID := models.Event_RubixDID
		err = psClient.SubscribeTopic(topicDID, processor.TxnCallBack)
		if err != nil {
			log.Printf("Warning: Failed to subscribe to PubSub topic %s: %v\n", topicDID, err)
		} else if *publishDummy {
			// TODO: DELETE LATER - Dummy Publisher for testing the new EventTransaction struct
			go func() {
				time.Sleep(5 * time.Second) // Delay to ensure explorer is fully ready
				processor.PublishDummyTransaction(psClient)
			}()
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
		log.Printf("Explorer Server listening on port :%s\n", port)
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
		log.Println("HTTP server stopped gracefully")
	}

	// 2) Stop IPFS Daemon & Worker Pool
	if processor.GlobalWorkerPool != nil {
		processor.GlobalWorkerPool.Shutdown()
	}
	ipfsManager.Stop()
	log.Println("IPFS Daemon & TxnProcessor stopped gracefully")

	// 3) Close database connection
	database.CloseDB()
	log.Println("Database connection closed")

	log.Printf("Server shutdown complete in %s\n", time.Since(shutdownStart).Round(time.Millisecond))
	log.Printf("Total uptime: %s\n", time.Since(startTime).Round(time.Second))
}

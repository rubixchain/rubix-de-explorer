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
	tokensync "explorer-server/sync"
	"explorer-server/util"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// 1. Setup Logging (Unique File per Restart + Console)
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		_ = os.Mkdir(logDir, 0755)
	}
	
	// Create a unique filename based on the current time
	startTime := time.Now()
	logFileName := fmt.Sprintf("explorer_%s.log", startTime.Format("2006-01-02_15-04-05"))
	
	logFile, err := os.OpenFile(filepath.Join(logDir, logFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not open log file: %v\n", err)
	} else {
		// Use MultiWriter for the standard log package (Go's log.Printf etc)
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)

		// On Linux, also redirect the OS-level file descriptors 1 (stdout) and 2 (stderr).
		// This ensures that "fatal error: concurrent map write" or panics are caught in the log file.
		util.RedirectStderr(logFile)
	}

	log.Println("----------------------------------------------------------------")
	log.Printf(">>> EXPLORER RESTART - %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Println("----------------------------------------------------------------")

	// CLI Flags
	testNet := flag.Bool("testnet", false, "Connect to Rubix TestNet (default: MainNet)")
	swarmKeyPath := flag.String("swarmkey", "", "Path to a custom swarm.key file (overrides built-in keys)")
	runSync := flag.Bool("sync", false, "Run one token-chain sync cycle 10s after startup (manual test mode; no periodic sync)")
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

	// One-time balance sync
	log.Println("Migration: Starting one-time Balance synchronization...")
	if err := operations.SyncAllBalances(database.WriteDB); err != nil {
		log.Printf("Migration Warning: Failed to sync balances: %v\n", err)
	} else {
		log.Println("Migration: Balance synchronization complete")
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
		}

		topicUnpledge := models.Event_RubixUnpledge
		err = psClient.SubscribeTopic(topicUnpledge, processor.TxnCallBack)
		if err != nil {
			log.Printf("Warning: Failed to subscribe to PubSub topic %s: %v\n", topicUnpledge, err)
		}
	}

	// --------------------------------------------------
	// Token chain sync (manual one-shot via -sync flag)
	// --------------------------------------------------
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()
	if *runSync {
		if tokenSyncSvc := tokensync.NewTokenSyncServiceFromEnv(*testNet); tokenSyncSvc != nil {
			go func() {
				log.Println("[TokenSync] -sync flag set; first cycle will run 10s after startup")
				select {
				case <-syncCtx.Done():
					return
				case <-time.After(10 * time.Second):
				}
				log.Println("[TokenSync] Starting one-shot sync cycle")
				tokenSyncSvc.RunOnce(syncCtx)
				log.Println("[TokenSync] One-shot sync cycle finished; explorer continues serving (no further syncs this run)")
			}()
		} else {
			log.Println("[TokenSync] -sync flag set but fullnode peer not configured (need TOKEN_SYNC_FULLNODE_PEER_ID)")
		}
	} else {
		log.Println("[TokenSync] Skipped (start with -sync to trigger one cycle)")
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

	syncCancel()

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

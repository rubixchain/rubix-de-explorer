package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/router"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	log.Println("🚀 Starting Explorer Server...")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using default values")
	}

	// Initialize PostgreSQL and auto-migrate tables
	log.Println("📦 Connecting to PostgreSQL and running migrations...")
	database.ConnectAndMigrate()

	// Insert dummy RBT data (if not exists)
	dummyRBT := models.RBT{
		TokenID:     "rbt-001",
		TokenValue:  100.5,
		OwnerDID:    "did:example:123",
		BlockID:     "block-001",
		BlockHeight: "1",
	}

	if err := database.DB.FirstOrCreate(&dummyRBT, models.RBT{TokenID: dummyRBT.TokenID}).Error; err != nil {
		log.Printf("⚠️ Failed to insert dummy RBT: %v", err)
	} else {
		log.Println("✅ Dummy RBT inserted")
	}

	// Setup router
	r := router.NewRouter()

	// Enable CORS
	handler := cors.Default().Handler(r)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Received shutdown signal...")
		database.CloseDB()
		log.Println("👋 Server shutdown complete")
		os.Exit(0)
	}()

	// Start server
	log.Printf("✅ Explorer server running on port :%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), handler))
}

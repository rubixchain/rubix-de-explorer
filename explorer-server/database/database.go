package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"explorer-server/database/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectAndMigrate initializes PostgreSQL with GORM and auto-migrates tables
func ConnectAndMigrate(drop bool) {
	// Build DSN
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		getEnv("PG_HOST", "localhost"),
		getEnv("PG_USER", "postgres"),
		getEnv("PG_PASSWORD", "postgres123"),
		getEnv("PG_DB", "explorer"),
		getEnv("PG_PORT", "5432"),
	)

	// Connect to PostgreSQL
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to PostgreSQL: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get sql.DB from GORM: %v", err)
	}

	// Connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("✅ Connected to PostgreSQL successfully")

	// Drop tables if requested
	if drop {
		log.Println("⚠️ Dropping existing tables...")
		dropTables()
		log.Println("✅ Tables dropped successfully")
	}

	// Auto-migrate tables
	err = DB.AutoMigrate(
		&models.RBT{},
		&models.FT{},
		&models.NFT{},
		&models.SmartContract{},
		&models.DIDs{},
		&models.TokenType{},
		&models.AllBlocks{},
		&models.TransferBlocks{},
		&models.TxnAnalytics{},
		&models.BurntBlocks{}, 
		&models.SC_Block{},
	)
	if err != nil {
		log.Fatalf("❌ Failed to migrate tables: %v", err)
	}

	log.Println("✅ Tables auto-migrated successfully")
}

// dropTables drops only the TransferBlocks table
func dropTables() {
	if DB.Migrator().HasTable(&models.TransferBlocks{}) {
		if err := DB.Migrator().DropTable(&models.TransferBlocks{}); err != nil {
			log.Fatalf("❌ Failed to drop TransferBlocks table: %v", err)
		}
	}
}

// getEnv fetches environment variable or returns fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// CloseDB closes PostgreSQL connection
func CloseDB() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			log.Printf("❌ Failed to get sql.DB for closing: %v", err)
			return
		}
		sqlDB.Close()
		log.Println("✅ PostgreSQL connection closed")
	}
}

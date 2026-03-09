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

// WriteDB is the connection pool for block ingestion (writes).
var WriteDB *gorm.DB

// ReadDB is the connection pool for API queries (reads).
var ReadDB *gorm.DB

// DB is kept for backward compatibility — points to WriteDB.
var DB *gorm.DB

// ConnectAndMigrate initializes PostgreSQL with GORM and auto-migrates tables
func ConnectAndMigrate(drop bool) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		getEnv("PG_HOST", "localhost"),
		getEnv("PG_USER", "postgres"),
		getEnv("PG_PASSWORD", "postgres123"),
		getEnv("PG_DB", "explorer"),
		getEnv("PG_PORT", "5432"),
	)

	// ── Write Pool (block ingestion) ──
	var err error
	WriteDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect WriteDB: %v", err)
	}
	if sqlDB, err := WriteDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// ── Read Pool (API queries) ──
	ReadDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect ReadDB: %v", err)
	}
	if sqlDB, err := ReadDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(30)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// Backward compat
	DB = WriteDB

	log.Println("Connected to PostgreSQL (WriteDB=20, ReadDB=30)")

	if drop {
		log.Println("Warning: Dropping existing tables...")
		dropTables()
		log.Println("Tables dropped successfully")
	}

	allModels := []interface{}{
		&models.RBT{},
		&models.FT{},
		&models.NFT{},
		&models.SC{},
		&models.DIDs{},
		&models.AllTokens{},
		&models.AllBlocks{},
		&models.TransactionBlocks{},
		&models.BurntBlocks{},
		&models.SCBlocks{},
		&models.MintBlocks{},
	}

	err = WriteDB.AutoMigrate(allModels...)
	if err != nil {
		log.Fatalf("Failed to migrate tables: %v", err)
	}

	log.Println("Tables auto-migrated successfully")

	// Ensure unique constraints (manual fix for SQLSTATE 42P10)
	ensureUniqueConstraints(WriteDB, allModels)
}

// ensureUniqueConstraints adds primary keys if they are missing, dynamically extracted from models
func ensureUniqueConstraints(db *gorm.DB, models []interface{}) {
	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			log.Printf("Warning: Could not parse model %T: %v", model, err)
			continue
		}

		tableName := stmt.Schema.Table
		pkFields := stmt.Schema.PrimaryFieldDBNames
		if len(pkFields) == 0 {
			log.Printf("Skipping %s: No primary key defined in model", tableName)
			continue
		}

		// Currently support single-column PKs for this migration
		pk := pkFields[0]

		query := fmt.Sprintf(`
			DO $$ 
			BEGIN 
				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint 
					WHERE conname = '%[1]s_pkey' 
				) THEN 
					ALTER TABLE "%[1]s" ADD PRIMARY KEY (%[2]s); 
				END IF; 
			END $$;`, tableName, pk)

		if err := db.Exec(query).Error; err != nil {
			log.Printf("Warning: Could not ensure primary key on %s: %v", tableName, err)
		}
	}
	log.Println("All primary key constraints verified")
}

// dropTables drops only the TransferBlocks table
func dropTables() {
	if WriteDB.Migrator().HasTable(&models.TransactionBlocks{}) {
		if err := WriteDB.Migrator().DropTable(&models.TransactionBlocks{}); err != nil {
			log.Fatalf("Failed to drop TransferBlocks table: %v", err)
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

// CloseDB closes both PostgreSQL connections
func CloseDB() {
	if WriteDB != nil {
		if sqlDB, err := WriteDB.DB(); err == nil {
			sqlDB.Close()
		}
	}
	if ReadDB != nil {
		if sqlDB, err := ReadDB.DB(); err == nil {
			sqlDB.Close()
		}
	}
	log.Println("PostgreSQL connections closed")
}

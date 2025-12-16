package database

// import (
// 	"fmt"
// 	"log"
// 	"os"
// 	"time"

// 	"gorm.io/driver/postgres"
// 	"gorm.io/gorm"
// 	"gorm.io/gorm/logger"
// )

// var DB *gorm.DB

// // ConnectDB initializes and connects to PostgreSQL using GORM
// func ConnectDB() {
// 	host := os.Getenv("DB_HOST")
// 	user := os.Getenv("DB_USER")
// 	password := os.Getenv("DB_PASSWORD")
// 	dbname := os.Getenv("DB_NAME")
// 	port := os.Getenv("DB_PORT")

// 	if host == "" {
// 		host = "localhost"
// 	}
// 	if port == "" {
// 		port = "5432"
// 	}
// 	if user == "" {
// 		user = "postgres"
// 	}
// 	if password == "" {
// 		password = "postgres"
// 	}
// 	if dbname == "" {
// 		dbname = "explorer"
// 	}

// 	dsn := fmt.Sprintf(
// 		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Kolkata",
// 		host, user, password, dbname, port,
// 	)

// 	var err error
// 	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
// 		Logger: logger.Default.LogMode(logger.Info), // shows SQL logs
// 	})
// 	if err != nil {
// 		log.Fatalf("❌ Failed to connect to database: %v", err)
// 	}

// 	sqlDB, err := DB.DB()
// 	if err != nil {
// 		log.Fatalf("❌ Failed to get generic DB object: %v", err)
// 	}

// 	// Set connection pool limits
// 	sqlDB.SetMaxIdleConns(10)
// 	sqlDB.SetMaxOpenConns(100)
// 	sqlDB.SetConnMaxLifetime(time.Hour)

// 	log.Println("✅ Connected to PostgreSQL database successfully.")
// }

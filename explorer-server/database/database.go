package database

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	DB     *sql.DB
	config *DatabaseConfig
)

// Initialize sets up the PostgreSQL database
func Initialize() error {
	config = GetDefaultConfig()

	log.Println("🔧 Initializing PostgreSQL database...")

	// Check if PostgreSQL binaries exist
	if !checkPostgreSQLBinaries() {
		return fmt.Errorf("PostgreSQL binaries not found. Please add them to: %s", config.BinDir)
	}

	// Initialize database if not exists
	if err := initializeDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	// Start PostgreSQL server
	if err := startPostgreSQLServer(); err != nil {
		return fmt.Errorf("failed to start PostgreSQL server: %v", err)
	}

	// Connect to database
	if err := connectToDatabase(); err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	// Create database if not exists
	if err := createDatabaseIfNotExists(); err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}

	// Reconnect to the specific database
	if err := connectToExplorerDatabase(); err != nil {
		return fmt.Errorf("failed to connect to explorer database: %v", err)
	}

	// Run schema migrations
	if err := runSchemaMigrations(); err != nil {
		return fmt.Errorf("failed to run schema migrations: %v", err)
	}

	log.Println("✅ PostgreSQL database initialized successfully")
	return nil
}

// checkPostgreSQLBinaries checks if required PostgreSQL binaries exist
func checkPostgreSQLBinaries() bool {
	requiredBinaries := []string{
		config.GetPostgresExecutable(),
		config.GetPgCtlExecutable(),
		config.GetInitDBExecutable(),
		config.GetCreateDBExecutable(),
		config.GetPsqlExecutable(),
	}

	missingBinaries := []string{}
	for _, binary := range requiredBinaries {
		if _, err := os.Stat(binary); os.IsNotExist(err) {
			missingBinaries = append(missingBinaries, binary)
		}
	}

	if len(missingBinaries) > 0 {
		log.Printf("❌ Missing PostgreSQL binaries:")
		for _, binary := range missingBinaries {
			log.Printf("   - %s", binary)
		}
		log.Printf("")
		log.Printf("🔧 To download PostgreSQL binaries:")
		log.Printf("   Windows: .\\database\\download-all-postgres.ps1")
		log.Printf("   Linux:   ./database/download-linux-postgres.sh")
		log.Printf("   macOS:   ./database/download-macos-postgres.sh")
		log.Printf("   All:     ./database/setup-postgres-all-platforms.sh")
		log.Printf("")
		log.Printf("📖 See database/README.md for detailed instructions")
		return false
	}

	log.Println("✅ All PostgreSQL binaries found")
	return true
}

// initializeDatabase initializes the PostgreSQL data directory
func initializeDatabase() error {
	// Check if data directory already exists and is initialized
	if isDataDirectoryInitialized() {
		log.Println("📁 PostgreSQL data directory already initialized")
		return nil
	}

	log.Println("🏗️ Initializing PostgreSQL data directory...")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Create password file for Windows compatibility
	pwFile := filepath.Join(config.DataDir, "..","pgpass.tmp")
	if err := ioutil.WriteFile(pwFile, []byte(config.Password), 0600); err != nil {
		return fmt.Errorf("failed to create password file: %v", err)
	}
	defer os.Remove(pwFile) // Clean up password file

	// Run initdb command with Windows-compatible password file
	cmd := exec.Command(config.GetInitDBExecutable(),
		"-D", config.DataDir,
		"-U", config.Username,
		"--pwfile="+pwFile,
		"--auth-local=md5",
		"--auth-host=md5",
	)

	// Set environment variables
	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, fmt.Sprintf("PATH=%s;%s", config.BinDir, os.Getenv("PATH")))
	} else {
		env = append(env, fmt.Sprintf("PATH=%s:%s", config.BinDir, os.Getenv("PATH")))
	}
	cmd.Env = env

	// Provide password via stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start initdb: %v", err)
	}

	// Write password to stdin
	fmt.Fprintln(stdin, config.Password)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("initdb failed: %v", err)
	}

	// Update postgresql.conf for our settings
	if err := updatePostgreSQLConfig(); err != nil {
		return fmt.Errorf("failed to update PostgreSQL config: %v", err)
	}

	log.Println("✅ PostgreSQL data directory initialized")
	return nil
}

// isDataDirectoryInitialized checks if PostgreSQL data directory is already initialized
func isDataDirectoryInitialized() bool {
	pgVersionFile := filepath.Join(config.DataDir, "PG_VERSION")
	_, err := os.Stat(pgVersionFile)
	return !os.IsNotExist(err)
}

// updatePostgreSQLConfig updates postgresql.conf with custom settings
func updatePostgreSQLConfig() error {
	configFile := filepath.Join(config.DataDir, "postgresql.conf")

	// Read current config
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read postgresql.conf: %v", err)
	}

	configStr := string(content)

	// Update settings
	settings := map[string]string{
		"port":                             strconv.Itoa(config.Port),
		"listen_addresses":                 "'localhost'",
		"max_connections":                  "100",
		"shared_buffers":                   "'128MB'",
		"effective_cache_size":             "'512MB'",
		"maintenance_work_mem":             "'64MB'",
		"checkpoint_completion_target":     "0.9",
		"wal_buffers":                      "'16MB'",
		"default_statistics_target":        "100",
		"random_page_cost":                 "1.1",
		"effective_io_concurrency":         "200",
		"work_mem":                         "'4MB'",
		"min_wal_size":                     "'1GB'",
		"max_wal_size":                     "'4GB'",
		"max_worker_processes":             "8",
		"max_parallel_workers_per_gather":  "2",
		"max_parallel_workers":             "8",
		"max_parallel_maintenance_workers": "2",
	}

	for key, value := range settings {
		// Handle both commented and uncommented lines
		commentedPattern := fmt.Sprintf("#%s = ", key)
		replacement := fmt.Sprintf("%s = %s", key, value)

		// Replace commented lines first
		configStr = strings.Replace(configStr, commentedPattern, replacement, 1)
		// Also replace any existing active lines to avoid duplication
		lines := strings.Split(configStr, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), key+" = ") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				lines[i] = replacement
				break
			}
		}
		configStr = strings.Join(lines, "\n")
	}

	// Write updated config
	if err := ioutil.WriteFile(configFile, []byte(configStr), 0644); err != nil {
		return fmt.Errorf("failed to write postgresql.conf: %v", err)
	}

	return nil
}

// startPostgreSQLServer starts the PostgreSQL server
func startPostgreSQLServer() error {
	if isPostgreSQLRunning() {
		log.Println("🟢 PostgreSQL server is already running")
		return nil
	}

	log.Println("🚀 Starting PostgreSQL server...")

	cmd := exec.Command(config.GetPgCtlExecutable(),
		"-D", config.DataDir,
		"-l", filepath.Join(config.DataDir, "postgres.log"),
		"start",
	)

	// Set environment variables
	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, fmt.Sprintf("PATH=%s;%s", config.BinDir, os.Getenv("PATH")))
	} else {
		env = append(env, fmt.Sprintf("PATH=%s:%s", config.BinDir, os.Getenv("PATH")))
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %v", err)
	}

	// Wait for server to be ready
	log.Println("⏳ Waiting for PostgreSQL server to be ready...")
	for i := 0; i < 30; i++ {
		if isPostgreSQLRunning() {
			log.Println("✅ PostgreSQL server started successfully")
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("PostgreSQL server failed to start within 30 seconds")
}

// isPostgreSQLRunning checks if PostgreSQL server is running
func isPostgreSQLRunning() bool {
	cmd := exec.Command(config.GetPgCtlExecutable(),
		"-D", config.DataDir,
		"status",
	)

	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, fmt.Sprintf("PATH=%s;%s", config.BinDir, os.Getenv("PATH")))
	} else {
		env = append(env, fmt.Sprintf("PATH=%s:%s", config.BinDir, os.Getenv("PATH")))
	}
	cmd.Env = env

	return cmd.Run() == nil
}

// connectToDatabase connects to PostgreSQL (without specific database)
func connectToDatabase() error {
	log.Println("🔌 Connecting to PostgreSQL...")

	connStr := config.GetConnectionStringWithoutDB()

	var err error
	for i := 0; i < 10; i++ {
		DB, err = sql.Open("postgres", connStr)
		if err == nil {
			if err = DB.Ping(); err == nil {
				log.Println("✅ Connected to PostgreSQL")
				return nil
			}
		}
		log.Printf("⏳ Connection attempt %d failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("failed to connect to PostgreSQL after 10 attempts: %v", err)
}

// createDatabaseIfNotExists creates the explorer database if it doesn't exist
func createDatabaseIfNotExists() error {
	log.Println("🏗️ Creating explorer database if not exists...")

	// Check if database exists
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", config.DatabaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %v", err)
	}

	if exists {
		log.Println("📦 Explorer database already exists")
		return nil
	}

	// Create database
	_, err = DB.Exec(fmt.Sprintf("CREATE DATABASE %s", config.DatabaseName))
	if err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}

	log.Println("✅ Explorer database created successfully")
	return nil
}

// connectToExplorerDatabase connects to the specific explorer database
func connectToExplorerDatabase() error {
	log.Println("🔌 Connecting to explorer database...")

	if DB != nil {
		DB.Close()
	}

	connStr := config.GetConnectionString()

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection to explorer database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping explorer database: %v", err)
	}

	log.Println("✅ Connected to explorer database")
	return nil
}

// runSchemaMigrations runs the database schema migrations
func runSchemaMigrations() error {
	log.Println("📋 Running database schema migrations...")

	// Read schema file
	baseDir := getProjectRoot()
	schemaFile := filepath.Join(baseDir, "database", "schema.sql")
	content, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	// Split schema into individual statements
	statements := strings.Split(string(content), ";")
	log.Printf("📊 Found %d total SQL statements in schema", len(statements))

	// Execute each statement
	for i, statement := range statements {
		statement = strings.TrimSpace(statement)

		if statement == "" {
			log.Printf("⏭️ Skipping statement %d (empty)", i+1)
			continue
		}

		// Skip statements that are only comments (check if contains SQL commands)
		hasSQL := strings.Contains(strings.ToUpper(statement), "CREATE") ||
				  strings.Contains(strings.ToUpper(statement), "INSERT") ||
				  strings.Contains(strings.ToUpper(statement), "UPDATE") ||
				  strings.Contains(strings.ToUpper(statement), "DELETE") ||
				  strings.Contains(strings.ToUpper(statement), "DROP") ||
				  strings.Contains(strings.ToUpper(statement), "ALTER")

		if !hasSQL {
			log.Printf("⏭️ Skipping statement %d (no SQL commands)", i+1)
			continue
		}

		// Skip empty statements after trimming
		if len(statement) == 0 {
			log.Printf("⏭️ Skipping statement %d (empty after trim)", i+1)
			continue
		}

		preview := statement
		if len(statement) > 100 {
			preview = statement[:100] + "..."
		}
		log.Printf("🔧 Executing SQL: %s", preview)
		_, err := DB.Exec(statement)
		if err != nil {
			log.Printf("⚠️ Schema statement failed: %v", err)
			log.Printf("📝 Statement was: %s", statement)
			// Don't return error for schema statements as some may already exist
		} else {
			log.Printf("✅ SQL executed successfully")
		}
	}

	log.Println("✅ Database schema migrations completed")
	return nil
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	return DB
}

// Close closes the database connection and stops the PostgreSQL server
func Close() error {
	log.Println("🔌 Closing database connection...")

	if DB != nil {
		if err := DB.Close(); err != nil {
			log.Printf("❌ Error closing database connection: %v", err)
		}
	}

	log.Println("🛑 Stopping PostgreSQL server...")

	cmd := exec.Command(config.GetPgCtlExecutable(),
		"-D", config.DataDir,
		"stop",
		"-m", "fast",
	)

	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, fmt.Sprintf("PATH=%s;%s", config.BinDir, os.Getenv("PATH")))
	} else {
		env = append(env, fmt.Sprintf("PATH=%s:%s", config.BinDir, os.Getenv("PATH")))
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		log.Printf("❌ Error stopping PostgreSQL server: %v", err)
		return err
	}

	log.Println("✅ PostgreSQL server stopped successfully")
	return nil
}

// IsHealthy checks if the database connection is healthy
func IsHealthy() bool {
	if DB == nil {
		return false
	}

	err := DB.Ping()
	return err == nil
}
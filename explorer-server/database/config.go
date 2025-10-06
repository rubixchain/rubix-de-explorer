package database

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DatabaseConfig holds the database configuration
type DatabaseConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	SSLMode      string
	DataDir      string
	BinDir       string
}

// GetDefaultConfig returns the default database configuration
func GetDefaultConfig() *DatabaseConfig {
	baseDir := getProjectRoot()
	pgBinDir := getPlatformBinDir(baseDir)
	dataDir := getPlatformDataDir(baseDir)

	return &DatabaseConfig{
		Host:         "localhost",
		Port:         5432,
		Username:     "explorer",
		Password:     "explorer_password",
		DatabaseName: "explorer_db",
		SSLMode:      "disable",
		DataDir:      dataDir,
		BinDir:       pgBinDir,
	}
}

// GetConnectionString returns the PostgreSQL connection string
func (c *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.DatabaseName, c.SSLMode)
}

// GetConnectionStringWithoutDB returns connection string without database name
func (c *DatabaseConfig) GetConnectionStringWithoutDB() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.SSLMode)
}

// getPlatformBinDir returns the platform-specific binary directory
func getPlatformBinDir(baseDir string) string {
	platform := runtime.GOOS
	switch platform {
	case "windows":
		return filepath.Join(baseDir, "database", "postgres-bin", "windows", "bin")
	case "linux":
		return filepath.Join(baseDir, "database", "postgres-bin", "linux", "bin")
	case "darwin":
		return filepath.Join(baseDir, "database", "postgres-bin", "macos", "bin")
	default:
		return filepath.Join(baseDir, "database", "postgres-bin", "linux", "bin")
	}
}

// getPlatformDataDir returns the platform-specific data directory
func getPlatformDataDir(baseDir string) string {
	platform := runtime.GOOS
	switch platform {
	case "windows":
		return filepath.Join(baseDir, "database", "postgres-bin", "windows", "data")
	case "linux":
		return filepath.Join(baseDir, "database", "postgres-bin", "linux", "data")
	case "darwin":
		return filepath.Join(baseDir, "database", "postgres-bin", "macos", "data")
	default:
		return filepath.Join(baseDir, "database", "postgres-bin", "linux", "data")
	}
}

// GetPostgresExecutable returns the path to postgres executable
func (c *DatabaseConfig) GetPostgresExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.BinDir, "postgres.exe")
	}
	return filepath.Join(c.BinDir, "postgres")
}

// GetPgCtlExecutable returns the path to pg_ctl executable
func (c *DatabaseConfig) GetPgCtlExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.BinDir, "pg_ctl.exe")
	}
	return filepath.Join(c.BinDir, "pg_ctl")
}

// GetInitDBExecutable returns the path to initdb executable
func (c *DatabaseConfig) GetInitDBExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.BinDir, "initdb.exe")
	}
	return filepath.Join(c.BinDir, "initdb")
}

// GetCreateDBExecutable returns the path to createdb executable
func (c *DatabaseConfig) GetCreateDBExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.BinDir, "createdb.exe")
	}
	return filepath.Join(c.BinDir, "createdb")
}

// GetPsqlExecutable returns the path to psql executable
func (c *DatabaseConfig) GetPsqlExecutable() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(c.BinDir, "psql.exe")
	}
	return filepath.Join(c.BinDir, "psql")
}

// getProjectRoot finds the project root directory by looking for go.mod file
func getProjectRoot() string {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}

	// Walk up the directory tree to find go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found go.mod, this is the project root
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root of the filesystem without finding go.mod
			// Fall back to current directory
			currentDir, _ := os.Getwd()
			return currentDir
		}
		dir = parent
	}
}
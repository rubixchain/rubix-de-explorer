package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"explorer-server/database"
)

// ProcessingService handles server-side data processing and aggregation
type ProcessingService struct {
	db            *sql.DB
	tokenQueries  *database.TokenQueries
	didQueries    *database.DIDQueries
	blockQueries  *database.BlockQueries
}

// NewProcessingService creates a new processing service
func NewProcessingService() *ProcessingService {
	return &ProcessingService{
		db:           database.GetDB(),
		tokenQueries: database.NewTokenQueries(database.GetDB()),
		didQueries:   database.NewDIDQueries(database.GetDB()),
		blockQueries: database.NewBlockQueries(database.GetDB()),
	}
}

// ProcessAll runs all processing tasks
func (ps *ProcessingService) ProcessAll() error {
	log.Println("🔄 Starting data processing pipeline...")
	startTime := time.Now()

	// Process in order: DIDs → Token Stats → Transaction Analytics
	if err := ps.ProcessDIDBalances(); err != nil {
		log.Printf("❌ Error processing DID balances: %v", err)
		return err
	}

	if err := ps.ProcessTokenStats(); err != nil {
		log.Printf("❌ Error processing token stats: %v", err)
		return err
	}

	if err := ps.ProcessTransactionAnalytics(); err != nil {
		log.Printf("❌ Error processing transaction analytics: %v", err)
		return err
	}

	duration := time.Since(startTime)
	log.Printf("✅ Data processing completed in %v", duration)
	return nil
}

// ProcessDIDBalances calculates and updates DID balances from token ownership
func (ps *ProcessingService) ProcessDIDBalances() error {
	log.Println("👤 Processing DID balances...")

	// Get token ownership counts and balances per DID
	query := `
		SELECT
			current_owner as did_id,
			COUNT(*) as total_balance,
			COUNT(CASE WHEN state = 'pledged' THEN 1 END) as pledged_amount,
			MAX(created_at) as last_active
		FROM tokens
		WHERE current_owner IS NOT NULL AND current_owner != ''
		GROUP BY current_owner
	`

	rows, err := ps.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query token balances: %v", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var didID string
		var totalBalance, pledgedAmount int64
		var lastActive time.Time

		err := rows.Scan(&didID, &totalBalance, &pledgedAmount, &lastActive)
		if err != nil {
			log.Printf("⚠️ Error scanning DID data: %v", err)
			continue
		}

		// Upsert DID record
		if err := ps.upsertDID(didID, float64(totalBalance), float64(pledgedAmount), lastActive); err != nil {
			log.Printf("⚠️ Failed to upsert DID %s: %v", didID, err)
			continue
		}

		processed++
	}

	log.Printf("✅ Processed %d DID balances", processed)
	return nil
}

// ProcessTokenStats aggregates token statistics by type
func (ps *ProcessingService) ProcessTokenStats() error {
	log.Println("📊 Processing token statistics...")

	// Get token counts by type and state
	query := `
		SELECT
			token_type,
			COUNT(*) as total_tokens,
			COUNT(CASE WHEN state = 'active' THEN 1 END) as active_tokens,
			COUNT(CASE WHEN state = 'pledged' THEN 1 END) as pledged_tokens
		FROM tokens
		WHERE token_type IS NOT NULL
		GROUP BY token_type
	`

	rows, err := ps.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query token stats: %v", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var tokenType string
		var totalTokens, activeTokens, pledgedTokens int64

		err := rows.Scan(&tokenType, &totalTokens, &activeTokens, &pledgedTokens)
		if err != nil {
			log.Printf("⚠️ Error scanning token stats: %v", err)
			continue
		}

		// Upsert token stats
		if err := ps.upsertTokenStats(tokenType, totalTokens, activeTokens, pledgedTokens); err != nil {
			log.Printf("⚠️ Failed to upsert token stats for %s: %v", tokenType, err)
			continue
		}

		processed++
		log.Printf("📈 %s: %d total, %d active, %d pledged", tokenType, totalTokens, activeTokens, pledgedTokens)
	}

	log.Printf("✅ Processed %d token type statistics", processed)
	return nil
}

// ProcessTransactionAnalytics aggregates transaction data by time intervals
func (ps *ProcessingService) ProcessTransactionAnalytics() error {
	log.Println("📈 Processing transaction analytics...")

	// Process hourly analytics for the last 24 hours
	now := time.Now()
	for i := 23; i >= 0; i-- {
		intervalStart := now.Add(-time.Duration(i+1) * time.Hour).Truncate(time.Hour)
		intervalEnd := intervalStart.Add(time.Hour)

		if err := ps.processTimeInterval(intervalStart, intervalEnd); err != nil {
			log.Printf("⚠️ Failed to process interval %v-%v: %v", intervalStart, intervalEnd, err)
			continue
		}
	}

	log.Printf("✅ Processed transaction analytics for last 24 hours")
	return nil
}

// Helper functions

func (ps *ProcessingService) upsertDID(didID string, totalBalance, pledgedAmount float64, lastActive time.Time) error {
	query := `
		INSERT INTO dids (did_id, created_at, last_active, total_balance, pledged_amount)
		VALUES ($1, NOW(), $2, $3, $4)
		ON CONFLICT (did_id) DO UPDATE SET
			last_active = EXCLUDED.last_active,
			total_balance = EXCLUDED.total_balance,
			pledged_amount = EXCLUDED.pledged_amount
	`
	_, err := ps.db.Exec(query, didID, lastActive, totalBalance, pledgedAmount)
	return err
}

func (ps *ProcessingService) upsertTokenStats(tokenType string, totalTokens, activeTokens, pledgedTokens int64) error {
	query := `
		INSERT INTO token_stats (token_type, total_tokens, active_tokens, pledged_tokens, last_updated)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (token_type) DO UPDATE SET
			total_tokens = EXCLUDED.total_tokens,
			active_tokens = EXCLUDED.active_tokens,
			pledged_tokens = EXCLUDED.pledged_tokens,
			last_updated = EXCLUDED.last_updated
	`
	_, err := ps.db.Exec(query, tokenType, totalTokens, activeTokens, pledgedTokens)
	return err
}

func (ps *ProcessingService) processTimeInterval(intervalStart, intervalEnd time.Time) error {
	// Get transaction counts by token type for this interval
	query := `
		SELECT
			COALESCE(t.token_type, 'unknown') as token_type,
			COUNT(b.block_id) as txn_count,
			COALESCE(SUM(b.amount), 0) as total_value
		FROM blocks b
		LEFT JOIN block_tokens bt ON b.block_id = bt.block_id
		LEFT JOIN tokens t ON bt.token_id = t.token_id
		WHERE b.txn_time >= $1 AND b.txn_time < $2
		GROUP BY t.token_type
	`

	rows, err := ps.db.Query(query, intervalStart, intervalEnd)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tokenType string
		var txnCount int64
		var totalValue float64

		err := rows.Scan(&tokenType, &txnCount, &totalValue)
		if err != nil {
			continue
		}

		// Upsert analytics data
		analyticsQuery := `
			INSERT INTO txn_analytics (interval_start, interval_end, txn_count, total_value, token_type)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (interval_start, interval_end, token_type) DO UPDATE SET
				txn_count = EXCLUDED.txn_count,
				total_value = EXCLUDED.total_value
		`
		_, err = ps.db.Exec(analyticsQuery, intervalStart, intervalEnd, txnCount, totalValue, tokenType)
		if err != nil {
			log.Printf("⚠️ Failed to upsert analytics for %s: %v", tokenType, err)
		}
	}

	return nil
}

// GetProcessingStats returns statistics about the processing service
func (ps *ProcessingService) GetProcessingStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count processed DIDs
	var didCount int64
	ps.db.QueryRow("SELECT COUNT(*) FROM dids").Scan(&didCount)
	stats["total_dids"] = didCount

	// Count token stats entries
	var tokenStatsCount int64
	ps.db.QueryRow("SELECT COUNT(*) FROM token_stats").Scan(&tokenStatsCount)
	stats["token_types_processed"] = tokenStatsCount

	// Count analytics entries
	var analyticsCount int64
	ps.db.QueryRow("SELECT COUNT(*) FROM txn_analytics").Scan(&analyticsCount)
	stats["analytics_intervals"] = analyticsCount

	// Last processing time
	var lastUpdated *time.Time
	ps.db.QueryRow("SELECT MAX(last_updated) FROM token_stats").Scan(&lastUpdated)
	if lastUpdated != nil {
		stats["last_processed"] = lastUpdated
	}

	return stats, nil
}
package operations

import (
	"encoding/json"
	"fmt"

	"explorer-server/database"
	"explorer-server/database/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Persistence helpers for the sync-txn-info-chain mirror. Two tables are
// written:
//
//   - SyncTransactions: one row per transaction_id (PK), info blob as JSONB.
//   - SyncTokenChain:   one row per (token_id, position) (composite PK).
//
// Both inserts use ON CONFLICT DO NOTHING so retries and overlapping pages
// are idempotent. UI does not read either table.

// KnownPosition is the per-token "I already have up to here" cursor sent to
// the fullnode in the next request.
type KnownPosition struct {
	Position      int64
	TransactionID string
}

// InsertSyncChainEntry writes one (token_id, position) row plus its info
// blob through the supplied gorm.DB (or the global WriteDB if nil). Both
// inserts use DO NOTHING — if either row already exists we leave it alone.
//
// rawInfo is the raw JSON bytes from the fullnode response. Passing the
// bytes through verbatim avoids re-marshalling and keeps the stored blob
// byte-identical to what the server emitted (useful for forensic debugging
// against a divergent fullnode).
func InsertSyncChainEntry(
	db *gorm.DB,
	tokenID string,
	position int64,
	txnID, prevTxnID string,
	role int16,
	rawInfo json.RawMessage,
) error {
	if db == nil {
		db = database.WriteDB
	}
	if tokenID == "" {
		return fmt.Errorf("InsertSyncChainEntry: empty tokenID")
	}
	if txnID == "" {
		return fmt.Errorf("InsertSyncChainEntry: empty txnID for token %s pos %d", tokenID, position)
	}

	// 1. Transaction row — keyed by transaction_id.
	txnRow := &models.SyncTransaction{
		TransactionID: txnID,
		Info:          rawInfo,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(txnRow).Error; err != nil {
		return fmt.Errorf("insert SyncTransaction %s: %w", txnID, err)
	}

	// 2. Chain row — keyed by (token_id, position).
	chainRow := &models.SyncTokenChain{
		TokenID:               tokenID,
		Position:              position,
		TransactionID:         txnID,
		Role:                  role,
		PreviousTransactionID: prevTxnID,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(chainRow).Error; err != nil {
		return fmt.Errorf("insert SyncTokenChain (%s, %d): %w", tokenID, position, err)
	}
	return nil
}

// PurgeSyncChainForToken removes every row in SyncTokenChain for tokenID.
// Used when the fullnode marks a token divergent: the prior local mirror is
// declared stale and the server is sending the chain from position 0.
//
// We deliberately do NOT delete from SyncTransactions — transaction rows are
// keyed by ID independently of which token chain references them, and a
// transaction may legitimately be referenced by multiple chains (e.g. a
// pledge entry shared across the pledger's and the pledgee's chain views).
// Re-inserting on the next page with ON CONFLICT DO NOTHING is safe.
func PurgeSyncChainForToken(db *gorm.DB, tokenID string) error {
	if db == nil {
		db = database.WriteDB
	}
	if tokenID == "" {
		return fmt.Errorf("PurgeSyncChainForToken: empty tokenID")
	}
	return db.Where("token_id = ?", tokenID).Delete(&models.SyncTokenChain{}).Error
}

// GetKnownPositions returns, for each token in tokenIDs, the highest stored
// position and the transaction_id at that position — exactly the shape the
// fullnode expects in the request's known_positions map. Tokens with no
// local rows are omitted from the result map; the caller leaves them out of
// the request (server defaults them to "full chain from position 0").
//
// Implemented as GROUP BY max(position) + self-join so the query runs on
// both Postgres (production) and sqlite (tests). The self-join only matches
// (token_id, position) pairs whose position equals the per-token max, which
// is exactly one row per token thanks to the composite PK.
func GetKnownPositions(db *gorm.DB, tokenIDs []string) (map[string]KnownPosition, error) {
	if db == nil {
		db = database.ReadDB
	}
	out := make(map[string]KnownPosition, len(tokenIDs))
	if len(tokenIDs) == 0 {
		return out, nil
	}

	type row struct {
		TokenID       string
		Position      int64
		TransactionID string
	}
	var rows []row
	err := db.Table(`SyncTokenChain AS tc`).
		Select(`tc.token_id, tc.position, tc.transaction_id`).
		Joins(`INNER JOIN (
			SELECT token_id, MAX(position) AS max_pos
			FROM SyncTokenChain
			WHERE token_id IN ?
			GROUP BY token_id
		) m ON m.token_id = tc.token_id AND m.max_pos = tc.position`, tokenIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query known positions: %w", err)
	}
	for _, r := range rows {
		out[r.TokenID] = KnownPosition{Position: r.Position, TransactionID: r.TransactionID}
	}
	return out, nil
}

// FlagTokenNeedsSync sets Tokens.needs_sync = true for the given token.
// Called by the sync structural validator when the chain linkage returned by
// the fullnode doesn't match what the explorer expected — already-inserted
// rows in SyncTokenChain are kept untouched; the flag is informational, an
// alarm bell for operators and a re-sync hint for the next cycle.
//
// No-op (and no error) if the token isn't in the Tokens table — the new sync
// flow lists tokens from Tokens itself, so a missing row here can only
// happen if the row was deleted between ListAllTokenIDs and this call.
func FlagTokenNeedsSync(db *gorm.DB, tokenID string) error {
	if db == nil {
		db = database.WriteDB
	}
	if tokenID == "" {
		return fmt.Errorf("FlagTokenNeedsSync: empty tokenID")
	}
	return db.Model(&models.Token{}).
		Where("token_id = ?", tokenID).
		Update("needs_sync", true).Error
}

// ListAllTokenIDs returns every token_id known to the explorer (from the
// existing Tokens table — populated by the pubsub path). The sync mirror's
// own list is bootstrapped from here on first run.
func ListAllTokenIDs(db *gorm.DB) ([]string, error) {
	if db == nil {
		db = database.ReadDB
	}
	var ids []string
	err := db.Model(&models.Token{}).
		Order("token_id ASC").
		Pluck("token_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list token IDs: %w", err)
	}
	return ids, nil
}

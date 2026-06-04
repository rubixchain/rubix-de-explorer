package models

import (
	"encoding/json"
	"time"
)

// SyncTransaction stores the full info blob fetched from the fullnode via the
// sync-txn-info-chain API. Sync-only mirror: nothing else writes to this
// table, and the public-facing UI does not read from it.
//
// Why `json` and not `jsonb`:
// The fullnode serves info bytes verbatim out of its own
// `fullnode_transactions.info` column — no unmarshal/remarshal cycle. The
// explorer must preserve those bytes exactly so any signed-digest invariant
// over info stays intact (and so a forensic compare against the fullnode
// side is byte-identical). Postgres `jsonb` parses to a normalized binary
// form, which reorders keys, drops whitespace, deduplicates duplicate keys,
// and can normalize numeric precision — all of which break byte equality.
// `json` (text) preserves the original bytes verbatim while still imposing
// JSON-validity at the column level.
type SyncTransaction struct {
	TransactionID string          `json:"transaction_id" gorm:"primaryKey;column:transaction_id"`
	Info          json.RawMessage `json:"info" gorm:"column:info;type:json"`
	CreatedAt     time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (SyncTransaction) TableName() string { return "SyncTransactions" }

// SyncTokenChain stores one chain entry per (token_id, position) — the
// authoritative ordering on the fullnode side. Inserts are idempotent
// (ON CONFLICT (token_id, position) DO NOTHING), so re-fetching pages is
// safe.
//
// Note on the composite primary key: gorm's AutoMigrate respects the two
// `primaryKey` tags and creates the table with PRIMARY KEY (token_id,
// position). The DB-level constraint fix-up in database.go only handles
// single-column PKs; if it ever runs against this table (it shouldn't,
// because AutoMigrate creates the constraint at table-create time), the
// IF NOT EXISTS gate keeps it from doing harm.
type SyncTokenChain struct {
	TokenID               string    `json:"token_id" gorm:"primaryKey;column:token_id"`
	Position              int64     `json:"position" gorm:"primaryKey;column:position;autoIncrement:false"`
	TransactionID         string    `json:"transaction_id" gorm:"column:transaction_id;index"`
	Role                  int16     `json:"role" gorm:"column:role"`
	PreviousTransactionID string    `json:"previous_transaction_id" gorm:"column:previous_transaction_id;index"`
	CreatedAt             time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (SyncTokenChain) TableName() string { return "SyncTokenChain" }

package operations

import (
	"encoding/json"
	"strconv"
	"testing"

	"explorer-server/database/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Token{},
		&models.SyncTransaction{},
		&models.SyncTokenChain{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestInsertSyncChainEntry_IdempotentInsert(t *testing.T) {
	db := newSyncTestDB(t)
	info := json.RawMessage(`{"initiator":"did1"}`)

	// First insert.
	if err := InsertSyncChainEntry(db, "tokA", 1, "tx1", "", 1, info); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Second insert with different prev_id — should NOT overwrite.
	if err := InsertSyncChainEntry(db, "tokA", 1, "tx1", "DIFFERENT", 99, info); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	var row models.SyncTokenChain
	if err := db.Where("token_id = ? AND position = ?", "tokA", 1).First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if row.PreviousTransactionID != "" {
		t.Errorf("ON CONFLICT DO NOTHING violated: prev_tx_id=%q (should be original empty string)", row.PreviousTransactionID)
	}
	if row.Role != 1 {
		t.Errorf("ON CONFLICT DO NOTHING violated: role=%d (should be original 1)", row.Role)
	}
}

func TestInsertSyncChainEntry_RejectsEmptyTokenAndTxn(t *testing.T) {
	db := newSyncTestDB(t)
	if err := InsertSyncChainEntry(db, "", 1, "tx1", "", 1, nil); err == nil {
		t.Error("expected error for empty tokenID")
	}
	if err := InsertSyncChainEntry(db, "tok", 1, "", "", 1, nil); err == nil {
		t.Error("expected error for empty txnID")
	}
}

func TestGetKnownPositions_ReturnsLatestPositionPerToken(t *testing.T) {
	db := newSyncTestDB(t)

	for pos := int64(1); pos <= 3; pos++ {
		if err := InsertSyncChainEntry(db, "tokA", pos, "A-"+strconv.Itoa(int(pos)), "", 1, json.RawMessage(`{}`)); err != nil {
			t.Fatalf("seed A pos %d: %v", pos, err)
		}
	}
	for pos := int64(10); pos <= 12; pos++ {
		if err := InsertSyncChainEntry(db, "tokB", pos, "B-"+strconv.Itoa(int(pos)), "", 1, json.RawMessage(`{}`)); err != nil {
			t.Fatalf("seed B pos %d: %v", pos, err)
		}
	}

	got, err := GetKnownPositions(db, []string{"tokA", "tokB", "tokMissing"})
	if err != nil {
		t.Fatalf("GetKnownPositions: %v", err)
	}
	if a, ok := got["tokA"]; !ok || a.Position != 3 || a.TransactionID != "A-3" {
		t.Errorf("tokA: want pos=3 tx=A-3 got %+v ok=%v", a, ok)
	}
	if b, ok := got["tokB"]; !ok || b.Position != 12 || b.TransactionID != "B-12" {
		t.Errorf("tokB: want pos=12 tx=B-12 got %+v ok=%v", b, ok)
	}
	if _, ok := got["tokMissing"]; ok {
		t.Error("tokMissing should not be in the result map (no rows)")
	}
}

func TestGetKnownPositions_EmptyInput(t *testing.T) {
	db := newSyncTestDB(t)
	got, err := GetKnownPositions(db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestPurgeSyncChainForToken_RemovesOnlyMatchingRows(t *testing.T) {
	db := newSyncTestDB(t)
	_ = InsertSyncChainEntry(db, "keep", 1, "K1", "", 1, json.RawMessage(`{}`))
	_ = InsertSyncChainEntry(db, "purge", 1, "P1", "", 1, json.RawMessage(`{}`))
	_ = InsertSyncChainEntry(db, "purge", 2, "P2", "P1", 1, json.RawMessage(`{}`))

	if err := PurgeSyncChainForToken(db, "purge"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var n int64
	db.Model(&models.SyncTokenChain{}).Where("token_id = ?", "purge").Count(&n)
	if n != 0 {
		t.Errorf("purge token rows remaining: %d", n)
	}
	db.Model(&models.SyncTokenChain{}).Where("token_id = ?", "keep").Count(&n)
	if n != 1 {
		t.Errorf("keep token should still have 1 row, got %d", n)
	}
	// Transactions table is intentionally NOT purged.
	var txn int64
	db.Model(&models.SyncTransaction{}).Count(&txn)
	if txn != 3 {
		t.Errorf("SyncTransactions count should stay at 3 (purge doesn't touch txns), got %d", txn)
	}
}

func TestListAllTokenIDs_OrderedAndDistinct(t *testing.T) {
	db := newSyncTestDB(t)
	for _, id := range []string{"tokC", "tokA", "tokB"} {
		if err := db.Create(&models.Token{TokenID: id, DID: "did", TokenType: 1}).Error; err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	got, err := ListAllTokenIDs(db)
	if err != nil {
		t.Fatalf("ListAllTokenIDs: %v", err)
	}
	want := []string{"tokA", "tokB", "tokC"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("idx %d: want %s got %s", i, want[i], got[i])
		}
	}
}

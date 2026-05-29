package operations

import (
	"bytes"
	"encoding/json"
	"errors"
	"explorer-server/database/models"
	"explorer-server/model"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- test helpers ----------

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Token{},
		&models.Transactions{},
		&models.EventTransaction{},
		&models.TransactionInfo{},
		&models.FailedTransactionInfo{},
		&models.TokenChain{},
		&models.TokenChainArray{},
		&models.DIDBalance{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// minimalInfo returns a TransactionInfo with all nil token slices so
// ProcessTransactionAssets short-circuits (no balance/state side-effects).
// The Initiator string is just a marker so per-entry rows look distinct.
func minimalInfo(initiator string) *model.TransactionInfo {
	return &model.TransactionInfo{Initiator: initiator}
}

// preInsertRawTxn writes a Transactions row directly with the given JSON bytes,
// bypassing the sync path. Used to simulate a row that PubSub already wrote.
func preInsertRawTxn(t *testing.T, db *gorm.DB, id string, infoBytes []byte) {
	t.Helper()
	row := &models.Transactions{
		ID:        id,
		Info:      json.RawMessage(infoBytes),
		Signature: json.RawMessage(`{}`),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("preInsertRawTxn %s: %v", id, err)
	}
}

// preInsertToken creates a Token row in the ChainSyncIssue state so we can
// verify MarkTokenSyncedWithStatus updates it correctly.
func preInsertToken(t *testing.T, db *gorm.DB, tokenID string) {
	t.Helper()
	row := &models.Token{
		TokenID:       tokenID,
		DID:           "didOwner",
		TokenType:     1, // RBT
		TokenStatus:   models.TokenStatus_ChainSyncIssue,
		TransactionID: "head-before-sync",
		NeedsSync:     true,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("preInsertToken: %v", err)
	}
}

func loadToken(t *testing.T, db *gorm.DB, tokenID string) models.Token {
	t.Helper()
	var tok models.Token
	if err := db.Where("token_id = ?", tokenID).First(&tok).Error; err != nil {
		t.Fatalf("load token %s: %v", tokenID, err)
	}
	return tok
}

func loadTxnInfoBytes(t *testing.T, db *gorm.DB, id string) []byte {
	t.Helper()
	var row models.Transactions
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("load txn %s: %v", id, err)
	}
	return []byte(row.Info)
}

func countTxns(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.Transactions{}).Count(&n).Error; err != nil {
		t.Fatalf("count txns: %v", err)
	}
	return n
}

func mkEntry(id string, role int16, prev string, initiator string) model.SyncedTxn {
	return model.SyncedTxn{
		ID:                    id,
		Role:                  role,
		PreviousTransactionID: prev,
		Info:                  minimalInfo(initiator),
	}
}

// ---------- scenarios ----------

func TestIngestTokenChain_HappyPath_StatusFromLastRole(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-happy"
	preInsertToken(t, db, tokenID)

	chain := []model.SyncedTxn{
		mkEntry("A", models.TokenRole_Mint, "", "init-A"),
		mkEntry("B", models.TokenRole_Transfer, "A", "init-B"),
		mkEntry("C", models.TokenRole_Transfer, "B", "init-C"),
	}

	if err := IngestTokenChain(db, tokenID, chain); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if n := countTxns(t, db); n != 3 {
		t.Errorf("expected 3 Transactions rows, got %d", n)
	}
	tok := loadToken(t, db, tokenID)
	if tok.NeedsSync {
		t.Error("needs_sync should be false after full sync")
	}
	if tok.TokenStatus != models.TokenStatus_Free {
		t.Errorf("expected token_status=Free (Transfer→Free), got %d", tok.TokenStatus)
	}
}

func TestIngestTokenChain_Resumption_ExistingRowsByteIdentical(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-resume"
	preInsertToken(t, db, tokenID)

	// PubSub already inserted A and B with specific Info content.
	aOriginal := []byte(`{"initiator":"pubsub-A","memo":"orig-A"}`)
	bOriginal := []byte(`{"initiator":"pubsub-B","memo":"orig-B"}`)
	preInsertRawTxn(t, db, "A", aOriginal)
	preInsertRawTxn(t, db, "B", bOriginal)

	// Sync returns the full chain from genesis [A, B, C].
	chain := []model.SyncedTxn{
		mkEntry("A", models.TokenRole_Mint, "", "sync-A-different"),
		mkEntry("B", models.TokenRole_Transfer, "A", "sync-B-different"),
		mkEntry("C", models.TokenRole_Transfer, "B", "sync-C"),
	}

	// Snapshot existing rows.
	aBefore := loadTxnInfoBytes(t, db, "A")
	bBefore := loadTxnInfoBytes(t, db, "B")

	if err := IngestTokenChain(db, tokenID, chain); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// A and B must be byte-identical (sync NEVER touches existing rows).
	aAfter := loadTxnInfoBytes(t, db, "A")
	bAfter := loadTxnInfoBytes(t, db, "B")
	if !bytes.Equal(aBefore, aAfter) {
		t.Errorf("Transactions[A].info was altered:\n  before=%s\n  after =%s", aBefore, aAfter)
	}
	if !bytes.Equal(bBefore, bAfter) {
		t.Errorf("Transactions[B].info was altered:\n  before=%s\n  after =%s", bBefore, bAfter)
	}

	// Only C should be new — total = 3.
	if n := countTxns(t, db); n != 3 {
		t.Errorf("expected 3 total Transactions rows (A+B preserved + C new), got %d", n)
	}

	// Validation passed because chain[0].prev = "" is the fullnode's genesis,
	// regardless of what the explorer already had. MarkTokenSyncedWithStatus ran.
	tok := loadToken(t, db, tokenID)
	if tok.NeedsSync {
		t.Error("needs_sync should be false after successful resumption")
	}
	if tok.TokenStatus != models.TokenStatus_Free {
		t.Errorf("expected Free from chain[2].Role=Transfer, got %d", tok.TokenStatus)
	}
}

func TestIngestTokenChain_StructuralGap_PartialCommitAndSentinel(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-gap"
	preInsertToken(t, db, tokenID)

	chain := []model.SyncedTxn{
		mkEntry("A", models.TokenRole_Mint, "", "init-A"),
		mkEntry("B", models.TokenRole_Transfer, "A", "init-B"),
		mkEntry("C", models.TokenRole_Transfer, "WRONG", "init-C"), // structural break
		mkEntry("D", models.TokenRole_Transfer, "C", "init-D"),
	}

	err := IngestTokenChain(db, tokenID, chain)
	if err == nil {
		t.Fatal("expected ErrChainValidationFailed")
	}
	if !errors.Is(err, ErrChainValidationFailed) {
		t.Errorf("expected ErrChainValidationFailed, got %v", err)
	}

	// A and B inserted; C and D skipped.
	if n := countTxns(t, db); n != 2 {
		t.Errorf("expected 2 inserts before break, got %d", n)
	}

	// Token stays flagged so the next cycle retries.
	tok := loadToken(t, db, tokenID)
	if !tok.NeedsSync {
		t.Error("needs_sync should remain true after partial sync")
	}
	if tok.TokenStatus != models.TokenStatus_ChainSyncIssue {
		t.Errorf("token_status should be unchanged (ChainSyncIssue), got %d", tok.TokenStatus)
	}
}

func TestIngestTokenChain_UnpledgeLast_StatusFree(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-unpledge"
	preInsertToken(t, db, tokenID)

	chain := []model.SyncedTxn{
		mkEntry("M", models.TokenRole_Mint, "", "i1"),
		mkEntry("P", models.TokenRole_Pledge, "M", "i2"),
		mkEntry("U", models.TokenRole_Unpledge, "P", "i3"), // last entry: Unpledge → Free
	}

	if err := IngestTokenChain(db, tokenID, chain); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	tok := loadToken(t, db, tokenID)
	if tok.TokenStatus != models.TokenStatus_Free {
		t.Errorf("Unpledge as last role should yield Free (0), got %d", tok.TokenStatus)
	}
	if tok.NeedsSync {
		t.Error("needs_sync should be false")
	}
}

func TestIngestTokenChain_PledgeLast_StatusPledged(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-pledge"
	preInsertToken(t, db, tokenID)

	chain := []model.SyncedTxn{
		mkEntry("M", models.TokenRole_Mint, "", "i1"),
		mkEntry("T", models.TokenRole_Transfer, "M", "i2"),
		mkEntry("P", models.TokenRole_Pledge, "T", "i3"), // last entry: Pledge → Pledged
	}

	if err := IngestTokenChain(db, tokenID, chain); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	tok := loadToken(t, db, tokenID)
	if tok.TokenStatus != models.TokenStatus_Pledged {
		t.Errorf("Pledge as last role should yield Pledged (6), got %d", tok.TokenStatus)
	}
}

func TestIngestTokenChain_AllExisting_NoInsertsButStillMarks(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-all-exist"
	preInsertToken(t, db, tokenID)

	// All three already exist with specific content.
	aOriginal := []byte(`{"initiator":"orig-A"}`)
	bOriginal := []byte(`{"initiator":"orig-B"}`)
	cOriginal := []byte(`{"initiator":"orig-C"}`)
	preInsertRawTxn(t, db, "A", aOriginal)
	preInsertRawTxn(t, db, "B", bOriginal)
	preInsertRawTxn(t, db, "C", cOriginal)

	chain := []model.SyncedTxn{
		mkEntry("A", models.TokenRole_Mint, "", "sync-A"),
		mkEntry("B", models.TokenRole_Transfer, "A", "sync-B"),
		mkEntry("C", models.TokenRole_Pledge, "B", "sync-C"), // last entry: Pledge → Pledged
	}

	aBefore := loadTxnInfoBytes(t, db, "A")
	bBefore := loadTxnInfoBytes(t, db, "B")
	cBefore := loadTxnInfoBytes(t, db, "C")

	if err := IngestTokenChain(db, tokenID, chain); err != nil {
		t.Fatalf("expected success even for all-existing chain, got: %v", err)
	}

	// No new rows.
	if n := countTxns(t, db); n != 3 {
		t.Errorf("expected count to stay at 3 (no new inserts), got %d", n)
	}

	// All existing rows byte-identical.
	if !bytes.Equal(aBefore, loadTxnInfoBytes(t, db, "A")) ||
		!bytes.Equal(bBefore, loadTxnInfoBytes(t, db, "B")) ||
		!bytes.Equal(cBefore, loadTxnInfoBytes(t, db, "C")) {
		t.Error("at least one existing Transactions.info was altered")
	}

	// MarkTokenSyncedWithStatus still ran — status re-derived from last role.
	tok := loadToken(t, db, tokenID)
	if tok.NeedsSync {
		t.Error("needs_sync should be cleared even when no entries were inserted")
	}
	if tok.TokenStatus != models.TokenStatus_Pledged {
		t.Errorf("expected Pledged from chain[2].Role=Pledge, got %d", tok.TokenStatus)
	}
}

func TestIngestTokenChain_EmptyChain_NoOp(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-empty"
	preInsertToken(t, db, tokenID)

	if err := IngestTokenChain(db, tokenID, nil); err != nil {
		t.Fatalf("nil chain should be no-op, got: %v", err)
	}
	if err := IngestTokenChain(db, tokenID, []model.SyncedTxn{}); err != nil {
		t.Fatalf("empty chain should be no-op, got: %v", err)
	}

	// No inserts, no token flag changes.
	if n := countTxns(t, db); n != 0 {
		t.Errorf("expected 0 txns inserted for empty chain, got %d", n)
	}
	tok := loadToken(t, db, tokenID)
	if !tok.NeedsSync {
		t.Error("needs_sync should remain true (no evidence explorer is up to date)")
	}
	if tok.TokenStatus != models.TokenStatus_ChainSyncIssue {
		t.Errorf("token_status should be unchanged, got %d", tok.TokenStatus)
	}
}

// Bonus: Uncommit/unknown role should leave token_status unchanged.
func TestMarkTokenSyncedWithStatus_UnknownRoleKeepsStatus(t *testing.T) {
	db := newTestDB(t)
	const tokenID = "tok-uncommit"
	preInsertToken(t, db, tokenID) // starts with ChainSyncIssue

	if err := MarkTokenSyncedWithStatus(db, tokenID, models.TokenRole_Uncommit); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tok := loadToken(t, db, tokenID)
	if tok.NeedsSync {
		t.Error("needs_sync should still be cleared for Uncommit role")
	}
	if tok.TokenStatus != models.TokenStatus_ChainSyncIssue {
		t.Errorf("Uncommit should leave token_status unchanged (ChainSyncIssue=14), got %d", tok.TokenStatus)
	}
}

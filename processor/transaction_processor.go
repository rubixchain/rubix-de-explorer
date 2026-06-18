package processor

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"explorer-server/database"
	"explorer-server/database/models"
	"explorer-server/database/operations"
	"explorer-server/model"
	"explorer-server/util"

	"gorm.io/gorm"
)

// HandleIncomingTxn orchestrates the unmarshaling, validation, and enqueuing of a transaction
func HandleIncomingTxn(newEvent *model.EventTransaction) {
	if newEvent.Transaction == nil {
		if txnID := getEventTxnID(newEvent); txnID != "" {
			_ = operations.SaveEventTransaction(nil, txnID, false, "missing transaction payload")
			_ = operations.SaveFailedTransactionReason(nil, txnID, "missing transaction payload")
			log.Printf("Warning: Received event with nil transaction, stored transaction %s as failed", txnID)
		} else {
			log.Printf("Warning: Received event with nil transaction and no transaction_id, skipping")
		}
		return
	}

	// 1. Format Validation
	if ok, reason := ValidateTransactionFormatWithReason(newEvent); !ok {
		newEvent.Status = false
		newEvent.Message = reason
	} else if info, err := newEvent.Transaction.ParseInfo(); err == nil && info != nil {
		if ok, reason := ValidateAllowlist(info); !ok {
			newEvent.Status = false
			newEvent.Message = reason
		} else if ok, reason := CheckNoDoubleMint(info); !ok {
			newEvent.Status = false
			newEvent.Message = reason
		}
	}

	// 2. Enqueue for processing
	if GlobalWorkerPool != nil {
		GlobalWorkerPool.EnqueueTransaction(newEvent)
	} else {
		txnID := getEventTxnID(newEvent)
		_ = operations.SaveEventTransaction(nil, txnID, false, "worker pool not initialized")
		_ = operations.SaveTransaction(nil, &models.Transactions{ID: txnID, Info: newEvent.Transaction.Info, Signature: newEvent.Transaction.Signature})
		_ = operations.SaveFailedTransactionReason(nil, txnID, "worker pool not initialized")
		log.Printf("Warning: GlobalWorkerPool not initialized, storing transaction %s as failed", txnID)
	}
}

func getEventTxnID(newEvent *model.EventTransaction) string {
	if newEvent.Transaction != nil && newEvent.Transaction.ID != "" {
		return newEvent.Transaction.ID
	}
	return newEvent.TransactionID
}

// ValidateTransactionFormat checks the DIDs and TokenIDs against Rubix format rules
func ValidateTransactionFormat(newEvent *model.EventTransaction) bool {
	ok, _ := ValidateTransactionFormatWithReason(newEvent)
	return ok
}

func ValidateTransactionFormatWithReason(newEvent *model.EventTransaction) (bool, string) {
	info, err := newEvent.Transaction.ParseInfo()
	if err != nil {
		log.Printf("Warning: Failed to parse transaction info for validation: %v", err)
		return false, fmt.Sprintf("failed to parse transaction info: %v", err)
	}
	if info == nil {
		return true, "" // No info to validate
	}

	// DID Validation
	if info.Initiator != "" && !util.IsValidDID(info.Initiator) {
		log.Printf("ID-FORMAT-ERR: Invalid Initiator DID format: %s", info.Initiator)
		return false, fmt.Sprintf("invalid initiator DID format: %s", info.Initiator)
	}
	if info.Owner != "" && !util.IsValidDID(info.Owner) {
		log.Printf("ID-FORMAT-ERR: Invalid Owner DID format: %s", info.Owner)
		return false, fmt.Sprintf("invalid owner DID format: %s", info.Owner)
	}

	// TokenID Validation
	if info.Tokens != nil {
		for _, t := range info.Tokens.RBT {
			if !util.IsValidRBT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid RBT TokenID format: %s", t.TokenID)
				return false, fmt.Sprintf("invalid RBT token ID format: %s", t.TokenID)
			}
		}
		for _, t := range info.Tokens.FT {
			if !util.IsValidFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid FT TokenID format: %s", t.TokenID)
				return false, fmt.Sprintf("invalid FT token ID format: %s", t.TokenID)
			}
		}
		for _, t := range info.Tokens.NFT {
			if !util.IsValidNFT(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid NFT TokenID format: %s", t.TokenID)
				return false, fmt.Sprintf("invalid NFT token ID format: %s", t.TokenID)
			}
		}
		for _, t := range info.Tokens.SmartContract {
			if !util.IsValidSC(t.TokenID) {
				log.Printf("ID-FORMAT-ERR: Invalid SC TokenID format: %s", t.TokenID)
				return false, fmt.Sprintf("invalid smart contract token ID format: %s", t.TokenID)
			}
		}
	}

	// Quorum DID and Token Validation
	for _, q := range info.Quorums {
		if !util.IsValidDID(q.Did) {
			log.Printf("ID-FORMAT-ERR: Invalid Quorum DID format: %s", q.Did)
			return false, fmt.Sprintf("invalid quorum DID format: %s", q.Did)
		}
		if q.Tokens != nil {
			for _, t := range q.Tokens {
				if !util.IsValidRBT(t.TokenID) {
					log.Printf("ID-FORMAT-ERR: Invalid Quorum RBT TokenID format: %s", t.TokenID)
					return false, fmt.Sprintf("invalid quorum RBT token ID format: %s", t.TokenID)
				}
			}
		}
	}

	// Quorum Signature DID Validation
	sig, err := newEvent.Transaction.ParseSignature()
	if err == nil && sig != nil {
		for _, q := range sig.Quorums {
			if !util.IsValidDID(q.Did) {
				log.Printf("ID-FORMAT-ERR: Invalid Quorum Signature DID format: %s", q.Did)
				return false, fmt.Sprintf("invalid quorum signature DID format: %s", q.Did)
			}
		}
	}

	// Committed Tokens Validation (Must be RBTs)
	for _, t := range info.CommittedTokens {
		if !util.IsValidRBT(t.TokenID) {
			log.Printf("ID-FORMAT-ERR: Invalid Committed RBT TokenID format: %s", t.TokenID)
			return false, fmt.Sprintf("invalid committed RBT token ID format: %s", t.TokenID)
		}
	}

	return true, ""
}

// ProcessDBTransaction handles the actual logic of logging/inserting to DB
// Called by workers in the DynamicWorkerPool
func ProcessDBTransaction(newEvent *model.EventTransaction, workerID int) {
	txnID := getEventTxnID(newEvent)

	// 1. Save EventTransaction (captures consensus result)
	if err := operations.SaveEventTransaction(nil, txnID, newEvent.Status, newEvent.Message); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save event: %v", workerID, txnID, err)
	}

	// 2. Save Raw Transaction
	if err := operations.SaveTransaction(nil, &models.Transactions{ID: txnID, Info: newEvent.Transaction.Info, Signature: newEvent.Transaction.Signature}); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save raw txn: %v", workerID, txnID, err)
	}

	// Parse Info
	txnInfo, err := newEvent.Transaction.ParseInfo()
	if err != nil || txnInfo == nil {
		reason := newEvent.Message
		if reason == "" {
			reason = fmt.Sprintf("failed to parse transaction info: %v", err)
		}
		if err := operations.SaveFailedTransactionReason(nil, txnID, reason); err != nil {
			log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save failed details: %v", workerID, txnID, err)
		}
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to parse info: %v", workerID, txnID, err)
		return
	}

	// 3. Save Flattened Details
	if err := operations.SaveTransactionDetails(nil, txnID, txnInfo, newEvent.Status, newEvent.Message); err != nil {
		log.Printf("[Worker %d] ERROR: Transaction %s - Failed to save details: %v", workerID, txnID, err)
	}

	// 4. Atomic Asset Processing (only for successful consensus)
	if newEvent.Status {
		err = database.WriteDB.Transaction(func(tx *gorm.DB) error {
			return operations.ProcessTransactionAssets(tx, txnInfo, txnID)
		})
		if err != nil {
			log.Printf("[Worker %d] ERROR: Transaction %s - Asset processing failed: %v", workerID, txnID, err)
			return
		}
	}

	// ONE SUMMARY LOG: Success
	log.Printf("[Worker %d] SUCCESS: Transaction %s processed (Status: %v)", workerID, txnID, newEvent.Status)
}

// ---------- DID + token-level allowlist ----------

var (
	explorerNetworkMu sync.RWMutex
	explorerNetwork   string
)

func SetExplorerNetwork(testnet bool) {
	explorerNetworkMu.Lock()
	defer explorerNetworkMu.Unlock()
	if testnet {
		explorerNetwork = util.NetworkTestnet
	} else {
		explorerNetwork = util.NetworkMainnet
	}
}

func GetExplorerNetwork() string {
	explorerNetworkMu.RLock()
	defer explorerNetworkMu.RUnlock()
	return explorerNetwork
}

func resetExplorerNetworkForTesting() {
	explorerNetworkMu.Lock()
	defer explorerNetworkMu.Unlock()
	explorerNetwork = ""
}

func networkMatches(infoNetwork, configured string) bool {
	if configured == "" {
		return false
	}
	if strings.TrimSpace(infoNetwork) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(infoNetwork), configured)
}

// Three checks in order: network match, per-token range, mainnet/testnet
// mint DID partition. Returns the first failure with a reason suitable for
// FailedTransactionInfo.failure_reason.
func ValidateAllowlist(info *model.TransactionInfo) (bool, string) {
	if info == nil {
		return true, ""
	}

	configured := GetExplorerNetwork()
	if !networkMatches(info.Network, configured) {
		return false, fmt.Sprintf(
			"network mismatch: explorer=%q transaction=%q",
			configured, info.Network,
		)
	}

	if info.Tokens == nil || len(info.Tokens.RBT) == 0 {
		return true, ""
	}

	isMint := isRBTMintTransaction(info)
	network := util.NormalizeNetwork(info.Network)

	for _, t := range info.Tokens.RBT {
		if t == nil {
			continue
		}
		elems, err := util.ParseRbtTokenID(t.TokenID)
		if err != nil {
			return false, fmt.Sprintf(
				"unparseable RBT token ID for allowlist check: %s",
				t.TokenID,
			)
		}

		if !util.IsAuthorizedTokenRange(network, elems.Level, elems.TokenNumber) {
			return false, fmt.Sprintf(
				"token %s outside allowed range for %s: level=%d number=%d",
				t.TokenID, network, elems.Level, elems.TokenNumber,
			)
		}

		if isMint && !util.IsAuthorizedMint(network, info.Initiator, elems.Level, elems.TokenNumber) {
			return false, fmt.Sprintf(
				"unauthorized mint on %s: initiator=%s token=%s level=%d number=%d",
				network, info.Initiator, t.TokenID, elems.Level, elems.TokenNumber,
			)
		}
	}

	return true, ""
}

// A transaction is a mint when every RBT token has empty previous_transaction_id
// — same classifier the pubsub ProcessTransactionAssets path uses.
func isRBTMintTransaction(info *model.TransactionInfo) bool {
	if info == nil || info.Tokens == nil || len(info.Tokens.RBT) == 0 {
		return false
	}
	for _, t := range info.Tokens.RBT {
		if t == nil {
			continue
		}
		// A non-empty previous transaction ID means a transfer, not a mint.
		if t.PreviousTransactionID != "" {
			return false
		}
		// Split outputs are freshly created sub-tokens (PartIndex > 0) of an
		// already-minted parent. They carry an empty previous_transaction_id
		// but are NOT mints — only whole tokens (PartIndex == 0) are genesis
		// mints. A single part token disqualifies the whole transaction from
		// the mint DID gate (it's a split, allowed for any owner). Unparseable
		// IDs are left to the range check in the caller to reject.
		if elems, err := util.ParseRbtTokenID(t.TokenID); err == nil && elems.PartIndex > 0 {
			return false
		}
	}
	return true
}

// Rejects a mint transaction targeting any RBT token already present in
// TokenChain with Role=Mint. Catches duplicate mints regardless of whether
// the second attempt is by the same DID or another.
//
// Out-of-order safety: we check TokenChain (Role=Mint) rather than the
// presence of a Tokens row, because a Tokens row can also be created by a
// transfer that arrived before its originating mint (see the
// "Missed genesis" branch in token_operations.go). A late-arriving
// legitimate mint should NOT be rejected just because a transfer landed
// first.
//
// Race window: two concurrent duplicate-mints can both pass this check
// before either worker inserts a TokenChain row. The first to write wins
// downstream; the second silently overwrites Tokens fields. The validation-
// time check catches the common case (duplicates arriving with any time
// gap); tighter race protection would require row-level locking inside
// ProcessTransactionAssets, which is out of scope here.
func CheckNoDoubleMint(info *model.TransactionInfo) (bool, string) {
	if !isRBTMintTransaction(info) {
		return true, ""
	}
	for _, t := range info.Tokens.RBT {
		if t == nil {
			continue
		}
		if existsAt, ok := lookupExistingMintTxn(database.ReadDB, t.TokenID); ok {
			return false, fmt.Sprintf(
				"double mint: token %s already minted in transaction %s",
				t.TokenID, existsAt,
			)
		}
	}
	return true, ""
}

func lookupExistingMintTxn(db *gorm.DB, tokenID string) (string, bool) {
	if db == nil {
		return "", false
	}
	var row models.TokenChain
	err := db.Model(&models.TokenChain{}).
		Where("token_id = ? AND role = ?", tokenID, models.TokenRole_Mint).
		Select("transaction_id").
		Limit(1).
		First(&row).Error
	if err != nil {
		return "", false
	}
	return row.TransactionID, true
}

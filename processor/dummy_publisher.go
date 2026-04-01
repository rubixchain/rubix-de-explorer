// TODO: DELETE LATER - Entire file for dummy publisher testing
package processor

import (
	"encoding/json"
	"explorer-server/model"
	"explorer-server/pubsub"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Internal helper for structured SC data in simulation
type SmartContractCall struct {
	Function string                 `json:"function"`
	Params   map[string]interface{} `json:"params"`
}

// Constants for TokenRoles (matching rubixgoplatform core lookup.go)
const (
	RoleMint     int16 = 1
	RoleTransfer int16 = 2
	RoleExecute  int16 = 3
	RoleDeploy   int16 = 4
	RoleBurn     int16 = 5
	// RoleCommit   int16 = 6 // Not used currently
	// RoleUncommit int16 = 7 // Not used currently
	RolePledge   int16 = 8
	RoleUnpledge int16 = 9
)

// TokenStore tracks a token's state during the simulation
type TokenStore struct {
	ID      string
	Type    string
	Value   float64
	LastTxn string
	Owner   string
	Burned  bool   // Track if token has been burned
	Data    string // Custom data per token (e.g. SC calls, FT value)
}

// TransactionTracker stores the full payload of a published transaction
type TransactionTracker struct {
	TxnID string
	Txn   *model.Transactions
}

func randomBase32(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyz234567"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func randomHex(length int) string {
	chars := "0123456789abcdef"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func randomCID(length int) string {
	chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// PublishDummyTransaction generates a cohesive narrative of transactions
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Starting Refined Story-Driven Dummy Generation...")
	rand.Seed(time.Now().UnixNano())

	// 1. Generate 10 Stable DIDs
	dids := make([]string, 10)
	for i := 0; i < 10; i++ {
		dids[i] = fmt.Sprintf("bafy%s", randomBase32(55))
	}
	// Conceptual Mapping:
	// dids[0]: Creator 1, dids[1]: Creator 2, dids[2]: SC Dev
	// dids[3]: Trader, dids[4]: Collector
	// dids[5], dids[6], dids[7]: Quorum DIDs (they pledge their own tokens)
	// dids[8], dids[9]: Participants

	allTokens := make(map[string]*TokenStore)
	var transactions []TransactionTracker

	// --- TOKEN GENERATION ---

	// A. Whole RBTs (Level 1, TokenNumber i)
	wholeRBTs := make([]*TokenStore, 0)
	for i := 0; i < 100; i++ {
		t := &TokenStore{
			ID:    fmt.Sprintf("1_%d", i),
			Type:  "RBT",
			Value: math.Round(val*1000) / 1000,
		}
		allTokens[t.ID] = t
		wholeRBTs = append(wholeRBTs, t)
	}

	// B. Part RBTs (created later by burning parent whole RBTs)
	partRBTs := make([]*TokenStore, 0)
	for i := 0; i < 20; i++ {
		partIdx := rand.Intn(2) + 1 // 1 or 2 -> value 0.5 (Level 1 in tree)
		parentID := wholeRBTs[80+i].ID
		t := &TokenStore{
			ID:    fmt.Sprintf("%s_%d", parentID, partIdx),
			Type:  "RBT",
			Value: 0.5,
		}
		allTokens[t.ID] = t
		partRBTs = append(partRBTs, t)
	}

	// C. FTs
	for i := 0; i < 10; i++ {
		t := &TokenStore{ID: fmt.Sprintf("ORANGE_%d_%s", i, dids[0]), Type: "FT", Value: 0.1}
		allTokens[t.ID] = t
	}
	for i := 0; i < 15; i++ {
		t := &TokenStore{ID: fmt.Sprintf("ORANGE_%d_%s", i+10, dids[1]), Type: "FT", Value: 0.01}
		allTokens[t.ID] = t
	}
	for i := 0; i < 5; i++ {
		t := &TokenStore{ID: fmt.Sprintf("APPLE_%d_%s", i, dids[2]), Type: "FT", Value: 0.5}
		allTokens[t.ID] = t
	}

	// D. 10 NFTs, 10 SCs
	for i := 0; i < 10; i++ {
		t := &TokenStore{ID: fmt.Sprintf("Qm%s", randomCID(44)), Type: "NFT", Value: 1.0}
		allTokens[t.ID] = t
	}
	for i := 0; i < 10; i++ {
		t := &TokenStore{ID: fmt.Sprintf("Qm%s", randomCID(44)), Type: "SC", Value: 1.0}
		allTokens[t.ID] = t
	}

	// --- HELPERS ---

	publishEvent := func(txn *model.Transactions, status bool, message string) {
		event := &model.EventTransaction{
			Transaction: txn,
			Status:      status,
			Message:     message,
		}
		data, err := json.Marshal(event)
		if err == nil {
			ps.Publish("rubix_txns", data)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// inSlice checks if a TokenStore is already in the given slice
	inSlice := func(ts *TokenStore, slice []*TokenStore) bool {
		for _, s := range slice {
			if s.ID == ts.ID {
				return true
			}
		}
		return false
	}

	// findOwnedRBT finds a non-burned RBT owned by `did` that is NOT in `exclude`
	findOwnedRBT := func(did string, exclude []*TokenStore) *TokenStore {
		for _, t := range allTokens {
			if t.Owner == did && t.Type == "RBT" && !t.Burned && !inSlice(t, exclude) {
				return t
			}
		}
		return nil
	}

	// status: true for success, false for failed consensus
	publishTxn := func(initiator string, receiver string, mainTokens []*TokenStore, burnTokens []*TokenStore, memo string, status bool, message string) {
		txnID := randomHex(64)
		tokenMap := &model.TransactionTokens{
			RBT:           make([]*model.TokenInfo, 0),
			NFT:           make([]*model.TokenInfo, 0),
			FT:            make([]*model.TokenInfo, 0),
			SmartContract: make([]*model.TokenInfo, 0),
		}

		// Build Tokens field (main tokens)
		for _, t := range mainTokens {
			dataStr := ""
			if t.Type == "FT" || (t.Type == "RBT" && t.Value != 1.0) {
				dataStr = fmt.Sprintf(`{"token_value": %f}`, t.Value)
			}
			if t.Type == "SC" || t.Type == "NFT" {
				dataStr = fmt.Sprintf(`{"metadata_uri": "ipfs://Qm%s"}`, randomCID(44))
			}

			ti := &model.TokenInfo{
				TokenID:               t.ID,
				PreviousTransactionID: t.LastTxn,
				Data:                  dataStr,
			}

			switch t.Type {
			case "RBT":
				tokenMap.RBT = append(tokenMap.RBT, ti)
			case "FT":
				tokenMap.FT = append(tokenMap.FT, ti)
			case "NFT":
				tokenMap.NFT = append(tokenMap.NFT, ti)
			case "SC":
				ti.Data = t.Data // Pass custom data for Smart Contracts
				tokenMap.SmartContract = append(tokenMap.SmartContract, ti)
			}

			// ONLY update state if transaction succeeded
			if status {
				t.Owner = receiver
				t.LastTxn = txnID
			}
		}

		// Build CommittedTokens field
		committedTokens := make([]*model.TokenInfo, 0)

		// Add burn tokens (e.g. parent RBTs burned during FT creation)
		for _, bt := range burnTokens {
			committedTokens = append(committedTokens, &model.TokenInfo{
				TokenID:               bt.ID,
				PreviousTransactionID: bt.LastTxn,
			})
			if status {
				bt.Burned = true
				bt.LastTxn = txnID
			}
		}

		// Add quorum pledge tokens (auto-select from quorum DIDs)
		qDIDs := []string{dids[5], dids[6], dids[7]}
		var quorums []*model.QuorumInfo
		for _, qDid := range qDIDs {
			qTokens := make([]*model.TokenInfo, 0)
			pt := findOwnedRBT(qDid, mainTokens)
			if pt != nil {
				qTokens = append(qTokens, &model.TokenInfo{
					TokenID:               pt.ID,
					PreviousTransactionID: pt.LastTxn,
				})
				// Also add pledge token to committedTokens
				committedTokens = append(committedTokens, &model.TokenInfo{
					TokenID:               pt.ID,
					PreviousTransactionID: pt.LastTxn,
				})
				if status {
					pt.LastTxn = txnID
				}
			}
			quorums = append(quorums, &model.QuorumInfo{
				Did:    qDid,
				Tokens: qTokens,
			})
		}

		txnInfo := &model.TransactionInfo{
			Initiator:       initiator,
			Owner:           receiver,
			Epoch:           2,
			Network:         "testnet",
			Tokens:          tokenMap,
			CommittedTokens: committedTokens,
			Quorums:         quorums,
			Memo:            memo,
		}

		signatures := &model.Signature{
			InitiatorSignature: "sig_" + randomHex(32),
			Quorums: []model.QuorumSignature{
				{Did: dids[5], Signature: "qsig_" + randomHex(32)},
				{Did: dids[6], Signature: "qsig_" + randomHex(32)},
				{Did: dids[7], Signature: "qsig_" + randomHex(32)},
			},
		}

		txn := &model.Transactions{
			TransactionID:   txnID,
			TransactionInfo: txnInfo,
			Signatures:      signatures,
		}

		transactions = append(transactions, TransactionTracker{TxnID: txnID, Txn: txn})
		publishEvent(txn, status, message)
	}

	// ======================================================================
	// PHASE 1: GENESIS MINTING (Whole RBTs only)
	// ======================================================================
	log.Println("Phase 1: Genesis Minting Whole RBTs...")

	// Mint whole RBTs (no burn tokens for genesis)
	publishTxn(dids[0], dids[0], wholeRBTs[:40], nil, "Genesis Minting RBTs", true, "Success")
	publishTxn(dids[1], dids[1], wholeRBTs[40:60], nil, "Genesis Minting RBTs", true, "Success")

	// Quorum DIDs need RBTs to pledge later
	publishTxn(dids[5], dids[5], wholeRBTs[60:65], nil, "Minting Quorum Pledge Tokens", true, "Success")
	publishTxn(dids[6], dids[6], wholeRBTs[65:70], nil, "Minting Quorum Pledge Tokens", true, "Success")
	publishTxn(dids[7], dids[7], wholeRBTs[70:75], nil, "Minting Quorum Pledge Tokens", true, "Success")

	// DID 0 and DID 1 mint the parent RBTs that will later be burned for parts
	publishTxn(dids[0], dids[0], wholeRBTs[75:80], nil, "Minting RBTs", true, "Success")
	publishTxn(dids[0], dids[0], wholeRBTs[80:90], nil, "Minting RBTs (Future Part Parents)", true, "Success")
	publishTxn(dids[1], dids[1], wholeRBTs[90:], nil, "Minting RBTs (Future Part Parents)", true, "Success")

	// ======================================================================
	// PHASE 1.5: PART TOKEN CREATION (Burn parent whole RBT -> Mint part RBT)
	// ======================================================================
	log.Println("Phase 1.5: Part Token Creation (Burn 1_XXX -> Mint 1_XXX_YYY)...")

	// DID 0 creates part tokens from wholeRBTs[80:90] (10 parents -> 10 parts)
	for i := 0; i < 10; i++ {
		parentRBT := wholeRBTs[80+i]
		publishTxn(dids[0], dids[0], []*TokenStore{partRBTs[i]},
			[]*TokenStore{parentRBT}, "Creating Part Token (burning parent RBT)", true, "Success")
	}

	// DID 1 creates part tokens from wholeRBTs[90:100] (10 parents -> 10 parts)
	for i := 0; i < 10; i++ {
		parentRBT := wholeRBTs[90+i]
		publishTxn(dids[1], dids[1], []*TokenStore{partRBTs[10+i]},
			[]*TokenStore{parentRBT}, "Creating Part Token (burning parent RBT)", true, "Success")
	}

	// ======================================================================
	// PHASE 2: FT CREATION (Burn parent RBT + Mint new FT)
	// ======================================================================
	log.Println("Phase 2: FT Creation (Burn RBT -> Mint FT)...")
	allFTs := getTokensByType(allTokens, "FT")

	// DID 0 creates ORANGE FTs by burning one of their RBTs
	// DID 0 creates 10 ORANGE FTs by burning ONE of their RBTs (1.0 RBT -> 10 FTs = 0.1 value each)
	burnRBT0 := findOwnedRBT(dids[0], nil)
	publishTxn(dids[0], dids[0], allFTs[:10],
		[]*TokenStore{burnRBT0}, "Creating ORANGE FTs (burning 1.0 RBT for 10 tokens)", true, "Success")

	// DID 1 creates 15 ORANGE FTs by burning TWO of their RBTs (2.0 RBT -> 15 FTs = 0.133 value each - diverse!)
	burnRBT1a := findOwnedRBT(dids[1], nil)
	burnRBT1b := findOwnedRBT(dids[1], []*TokenStore{burnRBT1a})
	publishTxn(dids[1], dids[1], allFTs[10:25],
		[]*TokenStore{burnRBT1a, burnRBT1b}, "Creating ORANGE FTs (burning 2.0 RBT for 15 tokens)", true, "Success")

	// DID 2 needs RBTs first - DID 0 transfers some to DID 2
	transferToDID2 := wholeRBTs[30:35]
	publishTxn(dids[0], dids[2], transferToDID2, nil, "Funding DID 2 for FT creation", true, "Success")

	// DID 2 creates 5 APPLE FTs by burning TWO of their RBTs (2.0 RBT -> 5 FTs = 0.4 value each)
	burnRBT2a := findOwnedRBT(dids[2], nil)
	burnRBT2b := findOwnedRBT(dids[2], []*TokenStore{burnRBT2a})
	publishTxn(dids[2], dids[2], allFTs[25:],
		[]*TokenStore{burnRBT2a, burnRBT2b}, "Creating APPLE FTs (burning 2.0 RBT for 5 tokens)", true, "Success")

	// ======================================================================
	// PHASE 3: NFT & SC MINTING
	// ======================================================================
	log.Println("Phase 3: NFT & SC Minting...")
	allNFTs := getTokensByType(allTokens, "NFT")
	allSCs := getTokensByType(allTokens, "SC")

	publishTxn(dids[2], dids[2], allNFTs, nil, "Minting NFT Collection", true, "Success")
	publishTxn(dids[2], dids[2], allSCs, nil, "Deploying Smart Contracts", true, "Success")

	// ======================================================================
	// PHASE 4: BUNDLED TRANSFERS (main tokens in Tokens, pledges in CommittedTokens)
	// ======================================================================
	log.Println("Phase 4: Bundled Transfers...")

	// DID 0 sends 5 RBTs + 5 ORANGE FTs to DID 3
	bundle1 := make([]*TokenStore, 0)
	for _, t := range wholeRBTs[:5] {
		if t.Owner == dids[0] && !t.Burned {
			bundle1 = append(bundle1, t)
		}
	}
	for _, t := range allFTs[:5] {
		if t.Owner == dids[0] {
			bundle1 = append(bundle1, t)
		}
	}
	if len(bundle1) > 0 {
		publishTxn(dids[0], dids[3], bundle1, nil, "Settlement for services", true, "Success")
	}

	// DID 1 sends 5 fractional RBTs + 5 ORANGE FTs + 1 NFT to DID 4
	bundle2 := make([]*TokenStore, 0)
	count := 0
	for _, t := range partRBTs {
		if t.Owner == dids[1] && !t.Burned && count < 5 {
			bundle2 = append(bundle2, t)
			count++
		}
	}
	for _, t := range allFTs[10:15] {
		if t.Owner == dids[1] {
			bundle2 = append(bundle2, t)
		}
	}
	bundle2 = append(bundle2, allNFTs[0])
	if len(bundle2) > 0 {
		publishTxn(dids[1], dids[4], bundle2, nil, "NFT purchase with mixed assets", true, "Success")
	}

	// ======================================================================
	// PHASE 5: 5-HOP PROVENANCE CHAIN
	// ======================================================================
	log.Println("Phase 5: 5-Hop Provenance Chain...")
	// Find a good whole RBT owned by DID 1 that hasn't been burned
	var specialRBT *TokenStore
	for _, t := range wholeRBTs[40:60] {
		if t.Owner == dids[1] && !t.Burned {
			specialRBT = t
			break
		}
	}
	if specialRBT != nil {
		publishTxn(dids[1], dids[5], []*TokenStore{specialRBT}, nil, "Hop 1", true, "Success")
		publishTxn(dids[5], dids[6], []*TokenStore{specialRBT}, nil, "Hop 2", true, "Success")
		publishTxn(dids[6], dids[7], []*TokenStore{specialRBT}, nil, "Hop 3", true, "Success")
		publishTxn(dids[7], dids[8], []*TokenStore{specialRBT}, nil, "Hop 4", true, "Success")
		publishTxn(dids[8], dids[9], []*TokenStore{specialRBT}, nil, "Hop 5 (Final)", true, "Success")
	}

	// ======================================================================
	// PHASE 6: SMART CONTRACT LIFECYCLE
	// ======================================================================
	log.Println("Phase 6: Smart Contract Execution...")
	vaultSC := allSCs[0]
	// Structured Calls for Smart Contract (Flat Array)
	callDeposit := []SmartContractCall{
		{Function: "deposit", Params: map[string]interface{}{"amount": 10.5, "currency": "RBT"}},
		{Function: "logEvent", Params: map[string]interface{}{"message": "Deposit successful"}},
	}
	dataDeposit, _ := json.Marshal(callDeposit)

	callWithdraw := []SmartContractCall{
		{Function: "withdraw", Params: map[string]interface{}{"amount": 5.0}},
		{Function: "verifyIdentity", Params: map[string]interface{}{"status": "verified"}},
	}
	dataWithdraw, _ := json.Marshal(callWithdraw)

	callStake := []SmartContractCall{
		{Function: "stake", Params: map[string]interface{}{"lock_period": "30d", "amount": 100}},
	}
	dataStake, _ := json.Marshal(callStake)

	vaultSC.Data = string(dataDeposit)
	publishTxn(dids[3], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault deposit()", true, "Success")

	vaultSC.Data = string(dataWithdraw)
	publishTxn(dids[4], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault withdraw()", true, "Success")

	vaultSC.Data = string(dataStake)
	publishTxn(dids[3], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault stake()", true, "Success")

	// --- FAILED TRANSACTIONS ---
	log.Println("Simulating Failed Transactions...")
	publishTxn(dids[0], dids[1], wholeRBTs[0:1], nil, "Failing to transfer (Low balance)", false, "Insufficient balance")
	publishTxn(dids[1], dids[0], allFTs[10:12], nil, "Failing to send FT (Invalid Quorum)", false, "Quorum consensus failed")

	vaultSC.Data = string(dataDeposit) // Re-use deposit for failed case
	publishTxn(dids[3], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault (Gas limit exceeded)", false, "Gas calculation error")

	log.Printf("Dummy Generation Complete. Published %d Transactions.", len(transactions))
}

func getTokensByType(store map[string]*TokenStore, tType string) []*TokenStore {
	var results []*TokenStore
	for _, t := range store {
		if t.Type == tType {
			results = append(results, t)
		}
	}
	return results
}

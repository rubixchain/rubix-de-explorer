// TODO: DELETE LATER - Entire file for dummy publisher testing
package processor

import (
	"encoding/json"
	"explorer-server/model"
	"explorer-server/pubsub"
	"fmt"
	"log"
	"math/rand"
	"time"
)

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
	Burned  bool // Track if token has been burned
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

	// A. Whole RBTs (these get minted first; some will later be burned to create part tokens or FTs)
	wholeRBTs := make([]*TokenStore, 0)
	for i := 0; i < 100; i++ {
		t := &TokenStore{
			ID:    fmt.Sprintf("1_%d_%d", i, time.Now().UnixNano()%1000000),
			Type:  "RBT",
			Value: 1.0,
		}
		allTokens[t.ID] = t
		wholeRBTs = append(wholeRBTs, t)
	}

	// B. Part RBTs (created later by burning parent whole RBTs)
	partRBTs := make([]*TokenStore, 0)
	for i := 0; i < 20; i++ {
		partIdx := rand.Intn(2) + 1 // 1 or 2 -> value 0.5
		parentID := wholeRBTs[80+i].ID // These parents will be burned
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

	publishEvent := func(txn *model.Transactions) {
		event := &model.EventTransaction{
			Transaction: txn,
			Status:      true,
			Message:     "Success",
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

	// publishTxn builds and publishes a transaction
	// mainTokens -> go into Tokens field (with the main role like Mint/Transfer/Deploy/Execute)
	// burnTokens -> go into CommittedTokens field (with RoleBurn)
	// Quorum pledge tokens are auto-selected and go into CommittedTokens too (with RolePledge)
	publishTxn := func(initiator string, receiver string, mainTokens []*TokenStore, burnTokens []*TokenStore, memo string) {
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
				tokenMap.SmartContract = append(tokenMap.SmartContract, ti)
			}

			// Update main token state
			t.Owner = receiver
			t.LastTxn = txnID
		}

		// Build CommittedTokens field
		committedTokens := make([]*model.TokenInfo, 0)

		// Add burn tokens (e.g. parent RBTs burned during FT creation)
		for _, bt := range burnTokens {
			committedTokens = append(committedTokens, &model.TokenInfo{
				TokenID:               bt.ID,
				PreviousTransactionID: bt.LastTxn,
			})
			bt.Burned = true
			bt.LastTxn = txnID
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
				pt.LastTxn = txnID
			}
			quorums = append(quorums, &model.QuorumInfo{Did: qDid, Tokens: qTokens})
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
		publishEvent(txn)
	}

	// ======================================================================
	// PHASE 1: GENESIS MINTING (Whole RBTs only)
	// ======================================================================
	log.Println("Phase 1: Genesis Minting Whole RBTs...")

	// Mint whole RBTs (no burn tokens for genesis)
	publishTxn(dids[0], dids[0], wholeRBTs[:40], nil, "Genesis Minting RBTs")
	publishTxn(dids[1], dids[1], wholeRBTs[40:60], nil, "Genesis Minting RBTs")

	// Quorum DIDs need RBTs to pledge later
	publishTxn(dids[5], dids[5], wholeRBTs[60:65], nil, "Minting Quorum Pledge Tokens")
	publishTxn(dids[6], dids[6], wholeRBTs[65:70], nil, "Minting Quorum Pledge Tokens")
	publishTxn(dids[7], dids[7], wholeRBTs[70:75], nil, "Minting Quorum Pledge Tokens")

	// DID 0 and DID 1 mint the parent RBTs that will later be burned for parts
	publishTxn(dids[0], dids[0], wholeRBTs[75:80], nil, "Minting RBTs")
	publishTxn(dids[0], dids[0], wholeRBTs[80:90], nil, "Minting RBTs (Future Part Parents)")
	publishTxn(dids[1], dids[1], wholeRBTs[90:], nil, "Minting RBTs (Future Part Parents)")

	// ======================================================================
	// PHASE 1.5: PART TOKEN CREATION (Burn parent whole RBT -> Mint part RBT)
	// ======================================================================
	log.Println("Phase 1.5: Part Token Creation (Burn 1_XXX -> Mint 1_XXX_YYY)...")

	// DID 0 creates part tokens from wholeRBTs[80:90] (10 parents -> 10 parts)
	for i := 0; i < 10; i++ {
		parentRBT := wholeRBTs[80+i]
		publishTxn(dids[0], dids[0], []*TokenStore{partRBTs[i]},
			[]*TokenStore{parentRBT}, "Creating Part Token (burning parent RBT)")
	}

	// DID 1 creates part tokens from wholeRBTs[90:100] (10 parents -> 10 parts)
	for i := 0; i < 10; i++ {
		parentRBT := wholeRBTs[90+i]
		publishTxn(dids[1], dids[1], []*TokenStore{partRBTs[10+i]},
			[]*TokenStore{parentRBT}, "Creating Part Token (burning parent RBT)")
	}

	// ======================================================================
	// PHASE 2: FT CREATION (Burn parent RBT + Mint new FT)
	// ======================================================================
	log.Println("Phase 2: FT Creation (Burn RBT -> Mint FT)...")
	allFTs := getTokensByType(allTokens, "FT")

	// DID 0 creates ORANGE FTs by burning one of their RBTs
	burnRBT0 := findOwnedRBT(dids[0], nil)
	publishTxn(dids[0], dids[0], allFTs[:10],
		[]*TokenStore{burnRBT0}, "Creating ORANGE FTs (burning parent RBT)")

	// DID 1 creates ORANGE FTs by burning one of their RBTs
	burnRBT1 := findOwnedRBT(dids[1], nil)
	publishTxn(dids[1], dids[1], allFTs[10:25],
		[]*TokenStore{burnRBT1}, "Creating ORANGE FTs (burning parent RBT)")

	// DID 2 needs RBTs first - DID 0 transfers some to DID 2
	transferToDID2 := wholeRBTs[30:35]
	publishTxn(dids[0], dids[2], transferToDID2, nil, "Funding DID 2 for FT creation")

	burnRBT2 := findOwnedRBT(dids[2], nil)
	publishTxn(dids[2], dids[2], allFTs[25:],
		[]*TokenStore{burnRBT2}, "Creating APPLE FTs (burning parent RBT)")

	// ======================================================================
	// PHASE 3: NFT & SC MINTING
	// ======================================================================
	log.Println("Phase 3: NFT & SC Minting...")
	allNFTs := getTokensByType(allTokens, "NFT")
	allSCs := getTokensByType(allTokens, "SC")

	publishTxn(dids[2], dids[2], allNFTs, nil, "Minting NFT Collection")
	publishTxn(dids[2], dids[2], allSCs, nil, "Deploying Smart Contracts")

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
		publishTxn(dids[0], dids[3], bundle1, nil, "Settlement for services")
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
		publishTxn(dids[1], dids[4], bundle2, nil, "NFT purchase with mixed assets")
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
		publishTxn(dids[1], dids[5], []*TokenStore{specialRBT}, nil, "Hop 1")
		publishTxn(dids[5], dids[6], []*TokenStore{specialRBT}, nil, "Hop 2")
		publishTxn(dids[6], dids[7], []*TokenStore{specialRBT}, nil, "Hop 3")
		publishTxn(dids[7], dids[8], []*TokenStore{specialRBT}, nil, "Hop 4")
		publishTxn(dids[8], dids[9], []*TokenStore{specialRBT}, nil, "Hop 5 (Final)")
	}

	// ======================================================================
	// PHASE 6: SMART CONTRACT LIFECYCLE
	// ======================================================================
	log.Println("Phase 6: Smart Contract Execution...")
	vaultSC := allSCs[0]

	publishTxn(dids[3], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault deposit()")
	publishTxn(dids[4], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault withdraw()")
	publishTxn(dids[3], dids[2], []*TokenStore{vaultSC}, nil, "Executing Vault stake()")

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

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
	RoleBurn     int16 = 5
	RolePledge   int16 = 8
)

// TreeLevelRanges holds [min, max] part-index range for each tree level (L0-L6)
var TreeLevelRanges = [][2]int{
	{0, 0},      // L0: 1.0
	{1, 2},      // L1: 0.5
	{3, 12},     // L2: 0.1
	{13, 32},    // L3: 0.05
	{33, 132},   // L4: 0.01
	{133, 332},  // L5: 0.005
	{333, 1332}, // L6: 0.001
}

// TokenStore tracks a token's state during the simulation
type TokenStore struct {
	ID      string
	Value   float64
	LastTxn string
	Owner   string
	Burned  bool
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

func getDenom(level int) float64 {
	denoms := []float64{1.0, 0.5, 0.1, 0.05, 0.01, 0.005, 0.001}
	if level >= 0 && level < len(denoms) {
		return denoms[level]
	}
	return 0
}

func getLevelOfIndex(idx int) int {
	for l, r := range TreeLevelRanges {
		if idx >= r[0] && idx <= r[1] {
			return l
		}
	}
	return -1
}

func getChildrenCount(level int) int {
	if level%2 == 0 {
		return 2
	}
	return 5
}

// PublishDummyTransaction generates a sequence of mints, transfers, and splits
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Starting Redesigned Dummy Generation (500 Mints, 100 Mixed Transfers)...")
	rand.Seed(time.Now().UnixNano())

	// 1. Generate 50 stable DIDs
	dids := make([]string, 50)
	for i := 0; i < 50; i++ {
		dids[i] = fmt.Sprintf("bafy%s", randomBase32(55))
	}

	inventory := make(map[string][]*TokenStore) // DID -> Tokens
	allTokens := make(map[string]*TokenStore)   // ID -> Token

	publishEvent := func(txn *model.Transactions, status bool, message string) {
		event := &model.EventTransaction{
			Transaction: txn,
			Status:      status,
			Message:     message,
		}
		data, _ := json.Marshal(event)
		ps.Publish("rubix_txns", data)
		time.Sleep(100 * time.Millisecond) // Throttling for ingestion
	}

	publishProtocolTxn := func(initiator, owner string, tokens, committed []*model.TokenInfo, quorums []*model.QuorumInfo, memo string) string {
		txnID := randomHex(64)
		txn := &model.Transactions{
			TransactionID: txnID,
			TransactionInfo: &model.TransactionInfo{
				Initiator:       initiator,
				Owner:           owner,
				Epoch:           int(time.Now().Unix()),
				Network:         "custom",
				Tokens:          &model.TransactionTokens{RBT: tokens},
				CommittedTokens: committed,
				Quorums:         quorums,
				Memo:            memo,
			},
			Signatures: &model.Signature{
				InitiatorSignature: "sig_" + randomHex(32),
				Quorums:            make([]model.QuorumSignature, 0),
			},
		}
		// Fill signatures for quorums
		for i := 0; i < len(quorums); i++ {
			txn.Signatures.Quorums = append(txn.Signatures.Quorums, model.QuorumSignature{
				Did:       quorums[i].Did,
				Signature: "qsig_" + randomHex(32),
			})
		}
		publishEvent(txn, true, "Success")
		return txnID
	}

	// ----------------------------------------------------------------------
	// PHASE 1: MINT 500 RBTs (Level 1, Token Number 1-500)
	// ----------------------------------------------------------------------
	log.Println("Phase 1: Minting 500 RBTs across 50 DIDs...")
	for i := 1; i <= 500; i++ {
		did := dids[(i-1)%50]
		tokenID := fmt.Sprintf("1_%d", i)
		t := &TokenStore{ID: tokenID, Value: 1.0, Owner: did}

		ti := &model.TokenInfo{TokenID: t.ID, PreviousTransactionID: "", Data: ""}
		txnID := publishProtocolTxn(did, did, []*model.TokenInfo{ti}, nil, nil, "Genesis Minting")

		t.LastTxn = txnID
		allTokens[t.ID] = t
		inventory[did] = append(inventory[did], t)
	}

	time.Sleep(1 * time.Second) // Let Genesis settle

	// Helper: find and remove a token by value
	pickTokenByValue := func(did string, val float64) *TokenStore {
		for i, t := range inventory[did] {
			if t.Value == val && !t.Burned {
				// Remove from inventory
				inventory[did] = append(inventory[did][:i], inventory[did][i+1:]...)
				return t
			}
		}
		return nil
	}

	// Helper: find any available whole token to split
	pickAnyWhole := func(did string) *TokenStore {
		for i, t := range inventory[did] {
			if t.Value == 1.0 && !t.Burned {
				inventory[did] = append(inventory[did][:i], inventory[did][i+1:]...)
				return t
			}
		}
		return nil
	}

	// Subdivision logic: splits a whole token until a specific denom is reached
	// Recursive split but published in ONE transaction info as per user request
	splitToken := func(did string, parent *TokenStore, targetValue float64) []*TokenStore {
		var allFreeParts []*TokenStore
		var allCommittedParts []*model.TokenInfo

		// Initial carry
		allCommittedParts = append(allCommittedParts, &model.TokenInfo{
			TokenID: parent.ID, PreviousTransactionID: parent.LastTxn,
		})
		parent.Burned = true
		if !json.Valid([]byte("[]")) { /* just a dummy check */ }

		// Manual Split: Level 1 is whole (value 1.0)
		// Children of L1 are L1 children (0.5 each).
		// Wait, the user said 1_1 is whole. So Level 1 is the "root".
		// Root Index 0.
		
		// Root for these tokens is Level 1, start with part index 0
		var pP int
		fmt.Sscanf(parent.ID, "1_%d", &pP)

		var recurseSplit func(currID string, currVal float64, currLevel int, currP int)
		recurseSplit = func(currID string, currVal float64, currLevel int, currP int) {
			// Bounds check and Epsilon (avoid float precision issues)
			if currVal <= targetValue+1e-9 || currLevel > 6 {
				t := &TokenStore{ID: currID, Value: currVal, Owner: did}
				allFreeParts = append(allFreeParts, t)
				return
			}
			
			// Needs more splitting
			d := getChildrenCount(currLevel - 1) 
			relP := currP - TreeLevelRanges[currLevel-1][0]
			childMin := TreeLevelRanges[currLevel][0] + relP * d
			
			childVal := getDenom(currLevel)
			
			if currID != parent.ID {
				allCommittedParts = append(allCommittedParts, &model.TokenInfo{
					TokenID: currID, PreviousTransactionID: "TEMP_TXN",
				})
			}

			runningSum := 0.0
			splitDone := false
			for i := 0; i < d; i++ {
				childID := fmt.Sprintf("1_%d_%d", pP, childMin+i)
				if !splitDone && runningSum+childVal < targetValue+1e-9 {
					// Use whole child
					t := &TokenStore{ID: childID, Value: childVal, Owner: did}
					allFreeParts = append(allFreeParts, t)
					runningSum += childVal
				} else if !splitDone {
					// This child needs to be split further
					recurseSplit(childID, childVal, currLevel+1, childMin+i)
					splitDone = true // Only one child is split further per level
				} else {
					// Remaining children are free
					t := &TokenStore{ID: childID, Value: childVal, Owner: did}
					allFreeParts = append(allFreeParts, t)
				}
			}
		}

		recurseSplit(parent.ID, 1.0, 1, 0)

		// Publish unified split Txn
		tokensInfos := make([]*model.TokenInfo, len(allFreeParts))
		for i, t := range allFreeParts {
			tokensInfos[i] = &model.TokenInfo{TokenID: t.ID, PreviousTransactionID: ""}
		}
		
		txnID := publishProtocolTxn(did, did, tokensInfos, allCommittedParts, nil, "Token Subdivision")
		
		for _, t := range allFreeParts {
			t.LastTxn = txnID
			allTokens[t.ID] = t
			inventory[did] = append(inventory[did], t)
		}
		return allFreeParts
	}

	// ----------------------------------------------------------------------
	// PHASE 2: 100 MIXED TRANSACTIONS
	// ----------------------------------------------------------------------
	log.Println("Phase 2: Executing 100 Mixed Transactions...")
	for i := 1; i <= 100; i++ {
		senderDID := dids[rand.Intn(50)]
		receiverDID := dids[rand.Intn(50)]
		for senderDID == receiverDID {
			receiverDID = dids[rand.Intn(50)]
		}

		// Pick random value: whole (1-5) or part (0.001-0.9)
		var val float64
		if rand.Float64() > 0.3 {
			val = float64(rand.Intn(5) + 1) // 1 to 5 whole
		} else {
			denoms := []float64{0.5, 0.1, 0.05, 0.01, 0.005, 0.001}
			val = denoms[rand.Intn(len(denoms))]
		}

		// Ensure sender has the value
		var txTokens []*TokenStore
		currentSum := 0.0
		
		// Attempt to collect from inventory
		for j := 0; j < len(inventory[senderDID]); j++ {
			t := inventory[senderDID][j]
			if currentSum + t.Value <= val {
				txTokens = append(txTokens, t)
				currentSum += t.Value
				inventory[senderDID] = append(inventory[senderDID][:j], inventory[senderDID][j+1:]...)
				j--
			}
			if currentSum == val { break }
		}

		// If still short, split a whole token
		if currentSum < val {
			wt := pickAnyWhole(senderDID)
			if wt != nil {
				splitToken(senderDID, wt, val - currentSum)
				// parts are now added back to inventory, so we try again to pick
				for j := 0; j < len(inventory[senderDID]); j++ {
					t := inventory[senderDID][j]
					if currentSum + t.Value <= val {
						txTokens = append(txTokens, t)
						currentSum += t.Value
						inventory[senderDID] = append(inventory[senderDID][:j], inventory[senderDID][j+1:]...)
						j--
					}
					if currentSum == val { break }
				}
			}
		}

		if len(txTokens) == 0 { continue } // Skip if failed to collect

		// Quorums
		qDIDs := make([]string, 0)
		for len(qDIDs) < 3 {
			qd := dids[rand.Intn(50)]
			if qd != senderDID && qd != receiverDID {
				already := false
				for _, exist := range qDIDs { if exist == qd { already = true } }
				if !already { qDIDs = append(qDIDs, qd) }
			}
		}

		quorums := make([]*model.QuorumInfo, 3)
		for qi, qdid := range qDIDs {
			// Each quorum pledges tokens totaling 'val'
			qSum := 0.0
			var qPledge []*model.TokenInfo
			
			// Pick/split tokens for quorum pledge
			for qSum < val {
				pt := pickTokenByValue(qdid, 1.0) // Try whole first
				if pt == nil { pt = pickAnyWhole(qdid) } // split if needed
				if pt == nil { break } // Should not happen in this simulation

				if qSum + pt.Value > val {
					// Need to split the pledge token to get the exact amount
					parts := splitToken(qdid, pt, val - qSum)
					// pick what we need
					for _, p := range parts {
						if qSum + p.Value <= val {
							qPledge = append(qPledge, &model.TokenInfo{TokenID: p.ID, PreviousTransactionID: p.LastTxn})
							qSum += p.Value
							// Note: we don't remove p from inventory because splitToken already added them.
							// So we need to remove what we used.
							for idx, inv := range inventory[qdid] {
								if inv.ID == p.ID {
									inventory[qdid] = append(inventory[qdid][:idx], inventory[qdid][idx+1:]...)
									break
								}
							}
						}
						if qSum == val { break }
					}
				} else {
					qPledge = append(qPledge, &model.TokenInfo{TokenID: pt.ID, PreviousTransactionID: pt.LastTxn})
					qSum += pt.Value
				}
			}
			quorums[qi] = &model.QuorumInfo{Did: qdid, Tokens: qPledge}
		}

		// Final Transfer
		tokensInfos := make([]*model.TokenInfo, len(txTokens))
		for ti, t := range txTokens {
			tokensInfos[ti] = &model.TokenInfo{TokenID: t.ID, PreviousTransactionID: t.LastTxn}
		}

		txnID := publishProtocolTxn(senderDID, receiverDID, tokensInfos, nil, quorums, fmt.Sprintf("Transfer Value %.3f", val))
		
		// Update owner
		for _, t := range txTokens {
			t.Owner = receiverDID
			t.LastTxn = txnID
			inventory[receiverDID] = append(inventory[receiverDID], t)
		}
	}

	log.Printf("Dummy Generation Complete. Published transactions to rubix_txns.")
}

func getTokensByType(store map[string]*TokenStore, tType string) []*TokenStore {
	var results []*TokenStore
	for _, t := range store {
		// In new logic, everything is RBT for now as per first request
		results = append(results, t)
	}
	return results
}

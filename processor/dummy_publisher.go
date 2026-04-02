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

// TokenStore tracks a token's state during the simulation
type TokenStore struct {
	ID      string
	Type    string // RBT, FT, NFT, SC
	Value   float64
	LastTxn string
	Owner   string
	Burned  bool
	Data    string
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

// PublishDummyTransaction generates a cohesive narrative of transactions
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Starting Detailed SC/NFT Execution Narrative...")
	rand.Seed(time.Now().UnixNano())

	// 1. Generate 30 Stable DIDs
	dids := make([]string, 30)
	for i := 0; i < 30; i++ {
		dids[i] = fmt.Sprintf("bafy%s", randomBase32(55))
	}

	allTokens := make(map[string]*TokenStore)
	inventory := make(map[string][]*TokenStore) // DID -> Tokens

	publishEvent := func(txn *model.Transactions, status bool, message string) {
		event := &model.EventTransaction{
			Transaction: txn,
			Status:      status,
			Message:     message,
		}
		data, _ := json.Marshal(event)
		ps.Publish("rubix_txns", data)
		time.Sleep(100 * time.Millisecond)
	}

	publishProtocolTxn := func(initiator, owner string, rbt, ft, nft, sc []*model.TokenInfo, committed []*model.TokenInfo, quorums []*model.QuorumInfo, memo string) string {
		txnID := randomHex(64)
		txn := &model.Transactions{
			TransactionID: txnID,
			TransactionInfo: &model.TransactionInfo{
				Initiator:       initiator,
				Owner:           owner,
				Epoch:           int(time.Now().Unix()),
				Network:         "custom",
				Tokens: &model.TransactionTokens{
					RBT:           rbt,
					FT:            ft,
					NFT:           nft,
					SmartContract: sc,
				},
				CommittedTokens: committed,
				Quorums:         quorums,
				Memo:            memo,
			},
			Signatures: &model.Signature{
				InitiatorSignature: "sig_" + randomHex(32),
				Quorums:            make([]model.QuorumSignature, 0),
			},
		}
		for i := 0; i < len(quorums); i++ {
			txn.Signatures.Quorums = append(txn.Signatures.Quorums, model.QuorumSignature{
				Did:       quorums[i].Did,
				Signature: "qsig_" + randomHex(32),
			})
		}
		publishEvent(txn, true, "Success")
		return txnID
	}

	// Helper to find tokens
	pickTokens := func(did string, tType string, count int) []*TokenStore {
		var selected []*TokenStore
		remaining := inventory[did][:0]
		for _, t := range inventory[did] {
			if len(selected) < count && t.Type == tType && !t.Burned {
				selected = append(selected, t)
			} else {
				remaining = append(remaining, t)
			}
		}
		inventory[did] = remaining
		return selected
	}

	// PHASE 1: GENESIS RBT MINTING (400 RBT)
	log.Println("Phase 1: Minting 400 Genesis RBTs...")
	for i := 1; i <= 400; i++ {
		did := dids[i%30]
		tID := fmt.Sprintf("1_%d", i)
		t := &TokenStore{ID: tID, Type: "RBT", Value: 1.0, Owner: did}
		ti := &model.TokenInfo{TokenID: t.ID}
		t.LastTxn = publishProtocolTxn(did, did, []*model.TokenInfo{ti}, nil, nil, nil, nil, nil, "Genesis RBT Mint")
		allTokens[t.ID] = t
		inventory[did] = append(inventory[did], t)
	}

	// PHASE 2: FT CREATION (10 combinations, 500 tokens)
	log.Println("Phase 2: Creating FT sets...")
	ftConfigs := []struct { Name string; DID string; Count int; Burn int }{
		{"NIKE", dids[0], 50, 10}, {"ADIDAS", dids[1], 10, 1}, {"PUMA", dids[2], 10, 2},
		{"REEBOK", dids[3], 50, 10}, {"GUCCI", dids[4], 60, 20}, {"VANS", dids[5], 90, 10},
	}
	for _, cfg := range ftConfigs {
		rbtToBurn := pickTokens(cfg.DID, "RBT", cfg.Burn)
		var committed []*model.TokenInfo
		for _, r := range rbtToBurn {
			committed = append(committed, &model.TokenInfo{TokenID: r.ID, PreviousTransactionID: r.LastTxn})
			r.Burned = true
		}
		var tokens []*model.TokenInfo
		var storeFTs []*TokenStore
		for i := 0; i < cfg.Count; i++ {
			tID := fmt.Sprintf("%s_%d_%s", cfg.Name, i, cfg.DID)
			t := &TokenStore{ID: tID, Type: "FT", Value: float64(cfg.Burn) / float64(cfg.Count), Owner: cfg.DID}
			tokens = append(tokens, &model.TokenInfo{TokenID: tID})
			storeFTs = append(storeFTs, t)
			allTokens[tID] = t
		}
		txnID := publishProtocolTxn(cfg.DID, cfg.DID, nil, tokens, nil, nil, committed, nil, "FT Minting")
		for _, t := range storeFTs { t.LastTxn = txnID; inventory[cfg.DID] = append(inventory[cfg.DID], t) }
	}

	// PHASE 3: NFT & SC DEPLOYMENT (20 each)
	log.Println("Phase 3: Deploying NFTs and SCs...")
	var nftList []*TokenStore
	var scList []*TokenStore
	for i := 0; i < 20; i++ {
		creator := dids[i%20]
		burnVal := float64(rand.Intn(10) + 1)
		rbtToBurn := pickTokens(creator, "RBT", int(burnVal))
		var committed []*model.TokenInfo
		for _, r := range rbtToBurn {
			committed = append(committed, &model.TokenInfo{TokenID: r.ID, PreviousTransactionID: r.LastTxn})
			r.Burned = true
		}
		// NFT
		nftID := "Qm" + randomHex(64); nft := &TokenStore{ID: nftID, Type: "NFT", Owner: creator, Value: burnVal}
		txnID_NFT := publishProtocolTxn(creator, creator, nil, nil, []*model.TokenInfo{{TokenID: nftID, Data: "deployment-of-NFT"}}, nil, committed, nil, "NFT Deployment")
		nft.LastTxn = txnID_NFT; allTokens[nftID] = nft; inventory[creator] = append(inventory[creator], nft); nftList = append(nftList, nft)
		// SC
		scID := "Qm" + randomHex(64); sc := &TokenStore{ID: scID, Type: "SC", Owner: creator, Value: burnVal}
		txnID_SC := publishProtocolTxn(creator, "", nil, nil, nil, []*model.TokenInfo{{TokenID: scID, Data: "deployment-of-SC"}}, committed, nil, "SC Deployment")
		sc.LastTxn = txnID_SC; allTokens[scID] = sc; inventory[creator] = append(inventory[creator], sc); scList = append(scList, sc)
	}

	// PHASE 4: EXECUTION (SC calls and NFT transfers)
	log.Println("Phase 4: Executing SCs and NFT transfers...")
	// 1. SC Execution
	for i, sc := range scList {
		executor := dids[(i+1)%30]
		isMixed := i < 10
		var rbtTX, ftTX []*model.TokenInfo
		var committed []*model.TokenInfo
		ownerField := ""
		totalVal := sc.Value
		if isMixed {
			receiver := dids[(i+2)%30]; ownerField = receiver
			extraR := pickTokens(sc.Owner, "RBT", 1)
			for _, r := range extraR {
				rbtTX = append(rbtTX, &model.TokenInfo{TokenID: r.ID, PreviousTransactionID: r.LastTxn})
				r.Owner = receiver
				inventory[receiver] = append(inventory[receiver], r)
				totalVal += r.Value
			}
		}
		// Data JSON
		data := fmt.Sprintf(`{"function":"execute","param":["arg%d", %d]}`, i, rand.Intn(100))
		scTI := &model.TokenInfo{TokenID: sc.ID, PreviousTransactionID: sc.LastTxn, Data: data}
		// Quorum based on total value (SC Value + Assets)
		var quorums []*model.QuorumInfo
		for len(quorums) < 3 {
			qDID := dids[rand.Intn(30)]
			if qDID != executor && qDID != ownerField {
				qT := pickTokens(qDID, "RBT", int(totalVal)+1)
				var qTIs []*model.TokenInfo
				for _, r := range qT {
					qTIs = append(qTIs, &model.TokenInfo{TokenID: r.ID, PreviousTransactionID: r.LastTxn})
					inventory[qDID] = append(inventory[qDID], r)
				}
				quorums = append(quorums, &model.QuorumInfo{Did: qDID, Tokens: qTIs})
			}
		}
		sc.LastTxn = publishProtocolTxn(executor, ownerField, rbtTX, ftTX, nil, []*model.TokenInfo{scTI}, committed, quorums, "SC Execution")
	}

	// 2. NFT Execution (Transfer / Self)
	for i, nft := range nftList {
		sender := nft.Owner
		receiver := dids[(i+5)%30]
		if i % 3 == 0 { receiver = sender } // Self execution case
		
		nftTI := &model.TokenInfo{TokenID: nft.ID, PreviousTransactionID: nft.LastTxn}
		totalVal := nft.Value
		var quorums []*model.QuorumInfo
		for len(quorums) < 3 {
			qDID := dids[rand.Intn(30)]
			if qDID != sender && qDID != receiver {
				qT := pickTokens(qDID, "RBT", int(totalVal)+1)
				var qTIs []*model.TokenInfo
				for _, r := range qT {
					qTIs = append(qTIs, &model.TokenInfo{TokenID: r.ID, PreviousTransactionID: r.LastTxn})
					inventory[qDID] = append(inventory[qDID], r)
				}
				quorums = append(quorums, &model.QuorumInfo{Did: qDID, Tokens: qTIs})
			}
		}
		nft.LastTxn = publishProtocolTxn(sender, receiver, nil, nil, []*model.TokenInfo{nftTI}, nil, nil, quorums, "NFT Move")
		nft.Owner = receiver; inventory[receiver] = append(inventory[receiver], nft)
	}

	log.Printf("Dummy Generation Complete. Narrative fully implemented.")
}

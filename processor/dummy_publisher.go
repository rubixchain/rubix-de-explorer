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

// PublishDummyTransaction publishes multiple dummy transactions covering different scenarios.
// Scale: 100 Transactions, 1000 Tokens (300 RBT, 600 FT, 50 NFT, 50 SC)
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Publishing Realistic Dummy Data: 1000 Tokens, 100 Transactions...")

	rand.Seed(time.Now().UnixNano())

	// 1. Generate 20 DIDs
	dids := make([]string, 20)
	for i := 0; i < 20; i++ {
		dids[i] = fmt.Sprintf("bafybm%s%04d", randomString(40), i)
	}

	// 2. Token Storage for tracking ownership and chains
	type TokenStore struct {
		ID      string
		Type    string // RBT, FT, NFT, SC
		Value   float64
		LastTxn string
		Owner   string
	}

	allTokens := make([]*TokenStore, 0, 1000)
	rbtPool := make([]*TokenStore, 0, 300)

	// 300 RBT
	for i := 0; i < 300; i++ {
		t := &TokenStore{
			ID:    fmt.Sprintf("1_%d_%d", i, time.Now().UnixNano()%1000000),
			Type:  "RBT",
			Value: 1.0,
		}
		allTokens = append(allTokens, t)
		rbtPool = append(rbtPool, t)
	}

	// 600 FT (APPLE, MANGO, ORANGE)
	ftGroups := []string{"APPLE", "MANGO", "ORANGE"}
	for _, group := range ftGroups {
		for i := 0; i < 200; i++ {
			// ORANGE is created by multiple DIDs randomly
			creator := dids[rand.Intn(len(dids))]
			t := &TokenStore{
				ID:    fmt.Sprintf("%s_%d_%s", group, i, creator),
				Type:  "FT",
				Value: 1.0,
			}
			allTokens = append(allTokens, t)
		}
	}

	// 50 NFT, 50 SC
	for i := 0; i < 50; i++ {
		allTokens = append(allTokens, &TokenStore{
			ID:    fmt.Sprintf("QmNFT%s%04d", randomString(30), i),
			Type:  "NFT",
			Value: 1.0,
		})
	}
	for i := 0; i < 50; i++ {
		allTokens = append(allTokens, &TokenStore{
			ID:    fmt.Sprintf("QmSC%s%04d", randomString(30), i),
			Type:  "SC",
			Value: 1.0,
		})
	}

	// --------------------------------------------------
	// Phase 1: Genesis Distribution (20 Transactions)
	// --------------------------------------------------
	for i := 0; i < 20; i++ {
		txnID := fmt.Sprintf("TXN_GENESIS_%02d", i)
		owner := dids[i]

		// Distribute ~50 tokens per txn
		start := i * 50
		end := (i + 1) * 50
		if i == 19 { end = 1000 }

		batch := allTokens[start:end]
		tokens := &model.TransactionTokens{}
		for _, t := range batch {
			t.Owner = owner
			t.LastTxn = txnID
			info := &model.TokenInfo{TokenID: t.ID}
			switch t.Type {
			case "RBT": tokens.RBT = append(tokens.RBT, info)
			case "FT":  tokens.FT = append(tokens.FT, info)
			case "NFT": 
				info.Data = "ipfs://nft-metadata-uri"
				tokens.NFT = append(tokens.NFT, info)
			case "SC":  
				info.Data = "contract_init_payload"
				tokens.SmartContract = append(tokens.SmartContract, info)
			}
		}

		publish(ps, txnID, "", owner, "Genesis Distribution", tokens, nil, nil)
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Genesis complete. Starting 80 random transfers...")

	// --------------------------------------------------
	// Phase 2: Transfers (80 Transactions)
	// --------------------------------------------------
	for i := 0; i < 80; i++ {
		txnID := fmt.Sprintf("TXN_TRANSFER_%04d", i)
		
		// Find a DID with tokens
		var sender string
		var senderTokens []*TokenStore
		for {
			sender = dids[rand.Intn(len(dids))]
			senderTokens = nil
			for _, t := range allTokens {
				if t.Owner == sender {
					senderTokens = append(senderTokens, t)
				}
			}
			if len(senderTokens) > 5 { break }
		}

		receiver := dids[rand.Intn(len(dids))]
		for receiver == sender { receiver = dids[rand.Intn(len(dids))] }

		// Send 5-15 tokens
		num := rand.Intn(10) + 5
		if num > len(senderTokens) { num = len(senderTokens) }
		
		selected := senderTokens[:num]
		tokens := &model.TransactionTokens{}
		for _, t := range selected {
			info := &model.TokenInfo{
				TokenID:               t.ID,
				PreviousTransactionID: t.LastTxn,
			}
			t.Owner = receiver
			t.LastTxn = txnID
			switch t.Type {
			case "RBT": tokens.RBT = append(tokens.RBT, info)
			case "FT":  tokens.FT = append(tokens.FT, info)
			case "NFT": tokens.NFT = append(tokens.NFT, info)
			case "SC":  tokens.SmartContract = append(tokens.SmartContract, info)
			}
		}

		// Populated Quorums (3 random DIDs, each with 1 RBT)
		quorums := make([]*model.QuorumInfo, 3)
		for q := 0; q < 3; q++ {
			qDID := dids[rand.Intn(len(dids))]
			qRBT := rbtPool[rand.Intn(len(rbtPool))]
			quorums[q] = &model.QuorumInfo{
				Did: qDID,
				Tokens: []*model.TokenInfo{{TokenID: qRBT.ID}},
			}
		}

		// Populated Committed Tokens (5 random RBTs)
		committed := make([]*model.TokenInfo, 5)
		for c := 0; c < 5; c++ {
			committed[c] = &model.TokenInfo{TokenID: rbtPool[rand.Intn(len(rbtPool))].ID}
		}

		publish(ps, txnID, sender, receiver, fmt.Sprintf("Transfer #%d", i), tokens, committed, quorums)
		
		if i%20 == 0 {
			log.Printf("Progress: %d/80 transfers published", i)
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Finished publishing 100 realistic dummy transactions.")
}

func publish(ps *pubsub.PubSub, id, from, to, memo string, tokens *model.TransactionTokens, committed []*model.TokenInfo, quorums []*model.QuorumInfo) {
	event := model.EventTransaction{
		Status: true,
		Transaction: &model.Transactions{
			TransactionID: id,
			TransactionInfo: &model.TransactionInfo{
				Initiator:       from,
				Owner:           to,
				Epoch:           int(time.Now().Unix()),
				Network:         "TestNet",
				Memo:            memo,
				Tokens:          tokens,
				CommittedTokens: committed,
				Quorums:         quorums,
			},
			Signatures: &model.Signature{
				InitiatorSignature: "sig_" + id,
			},
		},
	}
	data, _ := json.Marshal(event)
	ps.Publish("rubix_txns", data)
}

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

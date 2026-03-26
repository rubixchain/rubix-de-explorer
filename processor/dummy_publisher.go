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

// TokenStore tracks token ownership and chains
type TokenStore struct {
	ID      string
	Type    string // RBT, FT, NFT, SC
	Value   float64
	LastTxn string
	Owner   string
}

// publishGenesis handles exact distribution of a token batch to a specific owner
func publishGenesis(ps *pubsub.PubSub, ownerDID string, tokens []*TokenStore) {
	if len(tokens) == 0 {
		return
	}
	txnID := randomHex(64)
	msg := &model.TransactionTokens{}
	for _, t := range tokens {
		t.Owner = ownerDID
		t.LastTxn = txnID
		info := &model.TokenInfo{TokenID: t.ID}
		switch t.Type {
		case "RBT":
			info.Data = fmt.Sprintf("%f", t.Value) // Hack to pass token value
			msg.RBT = append(msg.RBT, info)
		case "FT":
			info.Data = fmt.Sprintf("%f", t.Value) // Hack to pass token value
			msg.FT = append(msg.FT, info)
		case "NFT":
			info.Data = "ipfs://nft-metadata-uri"
			msg.NFT = append(msg.NFT, info)
		case "SC":
			info.Data = "contract_init_payload"
			msg.SmartContract = append(msg.SmartContract, info)
		}
	}
	publish(ps, txnID, ownerDID, ownerDID, "Genesis Distribution", msg, nil, nil)
	time.Sleep(50 * time.Millisecond)
}

// PublishDummyTransaction publishes multiple dummy transactions covering different scenarios.
// Scale: 100 Transactions, 1000 Tokens (300 RBT, 600 FT, 50 NFT, 50 SC)
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Publishing Realistic Dummy Data: 1000 Tokens, 100 Transactions...")

	rand.Seed(time.Now().UnixNano())

	// 1. Generate 20 DIDs (Must match didRegex: bafy + 55 chars of Base32)
	dids := make([]string, 20)
	for i := 0; i < 20; i++ {
		dids[i] = fmt.Sprintf("bafy%s", randomBase32(55))
	}

	// 2. Token Storage for tracking ownership and chains
	allTokens := make([]*TokenStore, 0, 170)
	rbtPool := make([]*TokenStore, 0, 100)

	// 100 RBTs
	for i := 0; i < 100; i++ {
		val := (rand.Float64() * 0.999) + 0.001
		t := &TokenStore{
			ID:    fmt.Sprintf("1_%d_%d", i, time.Now().UnixNano()%1000000),
			Type:  "RBT",
			Value: math.Round(val*1000) / 1000,
		}
		allTokens = append(allTokens, t)
		rbtPool = append(rbtPool, t)
	}

	// 50 FTs exactly as requested
	addFTs := func(group, creator string, count int, value float64, startIdx *int) {
		for i := 0; i < count; i++ {
			t := &TokenStore{
				ID:    fmt.Sprintf("%s_%d_%s", group, *startIdx, creator),
				Type:  "FT",
				Value: value,
			}
			allTokens = append(allTokens, t)
			*startIdx++
		}
	}

	oIdx, mIdx, aIdx := 0, 0, 0
	addFTs("ORANGE", dids[0], 10, 0.100, &oIdx)
	addFTs("ORANGE", dids[1], 15, 0.010, &oIdx)
	addFTs("MANGO", dids[2], 10, 0.500, &mIdx)
	addFTs("MANGO", dids[3], 10, 0.700, &mIdx)
	addFTs("APPLE", dids[4], 2, 0.001, &aIdx)
	addFTs("APPLE", dids[5], 2, 0.005, &aIdx)
	addFTs("APPLE", dids[6], 1, 0.010, &aIdx)

	// 10 NFT, 10 SC
	for i := 0; i < 10; i++ {
		allTokens = append(allTokens, &TokenStore{ID: fmt.Sprintf("Qm%s", randomBase58(44)), Type: "NFT", Value: 1.0})
	}
	for i := 0; i < 10; i++ {
		allTokens = append(allTokens, &TokenStore{ID: fmt.Sprintf("Qm%s", randomBase58(44)), Type: "SC", Value: 1.0})
	}

	// --------------------------------------------------
	// Phase 1: Genesis Distribution
	// --------------------------------------------------
	// RBTs to 5 DIDs (dids[10] to dids[14]), 20 each
	for i := 0; i < 5; i++ {
		owner := dids[10+i]
		batch := rbtPool[i*20 : (i+1)*20]
		publishGenesis(ps, owner, batch)
	}

	// FTs to their creators (dids[0] to dids[6])
	ftStart := 100
	ftCount := 50
	ftBatch := allTokens[ftStart : ftStart+ftCount]
	// We just group them by the creator DID inside a map
	ftMap := make(map[string][]*TokenStore)
	for _, t := range ftBatch {
		creator := strings.Split(t.ID, "_")[2]
		ftMap[creator] = append(ftMap[creator], t)
	}
	for owner, batch := range ftMap {
		publishGenesis(ps, owner, batch)
	}

	// NFTs / SCs to 3 DIDs (dids[15] to dids[17])
	nscStart := 150
	nscBatch := allTokens[nscStart : nscStart+20]
	idx := 0
	for i := 0; i < 3; i++ {
		end := idx + 7
		if i == 2 {
			end = len(nscBatch)
		}
		if idx >= len(nscBatch) {
			break
		}
		publishGenesis(ps, dids[15+i], nscBatch[idx:end])
		idx = end
	}

	log.Printf("Genesis complete. Starting 80 random transfers...")

	// --------------------------------------------------
	// Phase 2: Transfers (80 Transactions)
	// --------------------------------------------------
	for i := 0; i < 80; i++ {
		txnID := randomHex(64)

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
			if len(senderTokens) > 5 {
				break
			}
		}

		receiver := dids[rand.Intn(len(dids))]
		for receiver == sender {
			receiver = dids[rand.Intn(len(dids))]
		}

		// Send 5-15 tokens
		num := rand.Intn(10) + 5
		if num > len(senderTokens) {
			num = len(senderTokens)
		}

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
			case "RBT":
				info.Data = fmt.Sprintf("%f", t.Value) // Hack
				tokens.RBT = append(tokens.RBT, info)
			case "FT":
				info.Data = fmt.Sprintf("%f", t.Value) // Hack
				tokens.FT = append(tokens.FT, info)
			case "NFT":
				tokens.NFT = append(tokens.NFT, info)
			case "SC":
				tokens.SmartContract = append(tokens.SmartContract, info)
			}
		}

		// Populated Quorums (3 random DIDs, each with 1 RBT)
		quorums := make([]*model.QuorumInfo, 3)
		for q := 0; q < 3; q++ {
			qDID := dids[rand.Intn(len(dids))]
			qRBT := rbtPool[rand.Intn(len(rbtPool))]
			quorums[q] = &model.QuorumInfo{
				Did:    qDID,
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

func randomBase32(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz234567")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func randomBase58(n int) string {
	var letters = []rune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func randomHex(n int) string {
	var letters = []rune("0123456789abcdef")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

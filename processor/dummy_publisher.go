// TODO: DELETE LATER - Entire file for dummy publisher testing
package processor

import (
	"encoding/json"
	"explorer-server/model"
	"explorer-server/pubsub"
	"fmt"
	"log"
	"time"
)

// PublishDummyTransaction publishes multiple dummy transactions covering different scenarios.
// TODO: DELETE LATER
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Publishing merged dummy transactions: Multiple assets, DID balances, and token chains...")

	dids := []string{
		"bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq", // Alice
		"bafybmf6j7n5e4v4z7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v", // Bob
		"bafybmguu5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v", // Charlie
		"bafybmexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // David
		"bafybmeyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", // Eve
	}

	ftNames := []string{"APPLE", "MANGO", "ORANGE", "PEAR", "BANANA"}

	// 1. Create Token Pools (10 of each type)
	rbtPool := make([]string, 10)
	ftPool := make([]string, 10)
	nftPool := make([]string, 10)
	scPool := make([]string, 10)
	
	now := time.Now().Unix()
	for i := 0; i < 10; i++ {
		rbtPool[i] = fmt.Sprintf("1_%d_%d_%d", i, i+100, now)
		ftPool[i] = fmt.Sprintf("%s_%d_%s", ftNames[i%len(ftNames)], i, dids[i%len(dids)])
		nftPool[i] = fmt.Sprintf("QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6u%c%c", 'a'+i, 'a'+i)
		scPool[i] = fmt.Sprintf("QmR7XvF6T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7%c%c%c%c", 's', 'c', 'a'+i, 'a'+i)
	}

	lastTxnForToken := make(map[string]string)

	// 2. Publish 100 Transactions
	for i := 0; i < 100; i++ {
		txnID := fmt.Sprintf("TXN_%04d_%d", i, time.Now().UnixNano()%1000)
		sender := dids[i%len(dids)]
		receiver := dids[(i+1)%len(dids)]
		
		assetType := i % 4 // 0: RBT, 1: FT, 2: NFT, 3: SC
		var pool []string
		switch assetType {
		case 0: pool = rbtPool
		case 1: pool = ftPool
		case 2: pool = nftPool
		case 3: pool = scPool
		}

		// Pick 1-2 tokens from the pool
		numTokens := (i % 2) + 1
		tokenInfos := make([]*model.TokenInfo, numTokens)
		for k := 0; k < numTokens; k++ {
			tokenID := pool[(i+k)%len(pool)]
			tokenInfos[k] = &model.TokenInfo{
				TokenID:               tokenID,
				PreviousTransactionID: lastTxnForToken[tokenID],
			}
			lastTxnForToken[tokenID] = txnID
		}

		// Prepare Event
		tokens := &model.TransactionTokens{}
		switch assetType {
		case 0: tokens.RBT = tokenInfos
		case 1: tokens.FT = tokenInfos
		case 2: tokens.NFT = tokenInfos
			for _, t := range tokenInfos { t.Data = "ipfs://nft-metadata" }
		case 3: tokens.SmartContract = tokenInfos
			for _, t := range tokenInfos { t.Data = "contract_call" }
		}

		event := model.EventTransaction{
			Status: true,
			Transaction: &model.Transactions{
				TransactionID: txnID,
				TransactionInfo: &model.TransactionInfo{
					Initiator: sender,
					Owner:     receiver,
					Epoch:     int(time.Now().Unix()) - (100-i)*60,
					Network:   "TestNet",
					Memo:      fmt.Sprintf("Step %d - %s", i, txnID),
					Tokens:    tokens,
				},
				Signatures: &model.Signature{
					InitiatorSignature: "sig_" + txnID,
				},
			},
		}

		data, _ := json.Marshal(event)
		ps.Publish("rubix_txns", data)

		if i%25 == 0 {
			log.Printf("Progress: Published %d/100 transactions", i)
		}
		time.Sleep(30 * time.Millisecond)
	}

	log.Printf("Finished publishing 100 merged dummy transactions with consistent chains.")
}

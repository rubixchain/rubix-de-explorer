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
	log.Printf("Publishing 60 randomized dummy transactions to test APIs...")

	dids := []string{
		"bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq", // Alice
		"bafybmf6j7n5e4v4z7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v", // Bob
		"bafybmguu5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v", // Charlie
		"bafybmexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // David
		"bafybmeyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy", // Eve
		"bafybmfzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", // Frank
		"bafybmgaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Grace
		"bafybmhbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", // Heidi
		"bafybmicccccccccccccccccccccccccccccccccccccccccccccccccccc", // Ivan
		"bafybmidddddddddddddddddddddddddddddddddddddddddddddddddddd", // Judy
	}

	ftNames := []string{"APPLE", "MANGO", "ORANGE", "PEAR", "BANANA"}

	for i := 0; i < 86; i++ {
		time.Sleep(50 * time.Millisecond) // Faster publishing

		senderIdx := (i * 3) % len(dids)
		receiverIdx := (i * 7) % len(dids)
		if senderIdx == receiverIdx {
			receiverIdx = (receiverIdx + 1) % len(dids)
		}

		assetType := i % 4 // 0: RBT, 1: FT, 2: NFT, 3: SC
		sender := dids[senderIdx]
		receiver := dids[receiverIdx]
		
		// Special Test Scenario: Shared FT name 'TITAN' from different creators
		testName := ""
		if i >= 80 {
			assetType = 1 // Force FT
			testName = "TITAN"
			if i < 83 {
				sender = dids[0] // Alice creates TITAN
			} else {
				sender = dids[1] // Bob also creates TITAN
			}
			receiver = dids[2] // Charlie receives all TITANs
		}

		txnID := fmt.Sprintf("TXN_%04d_%d", i, time.Now().Unix()%1000)
		epoch := int(time.Now().Unix()) - (90-i)*60

		event := model.EventTransaction{
			Status: true,
			Transaction: &model.Transactions{
				TransactionID: txnID,
				TransactionInfo: &model.TransactionInfo{
					Initiator: sender,
					Owner:     receiver,
					Epoch:     epoch,
					Network:   "TestNet",
					Memo:      fmt.Sprintf("Dummy Txn %d - Asset Type %d", i, assetType),
				},
				Signatures: &model.Signature{
					InitiatorSignature: fmt.Sprintf("sig_%s", txnID),
				},
			},
		}

		// Vary the number of tokens to create different balances
		numTokens := (i % 3) + 1 // 1, 2, or 3 tokens
		tokens := &model.TransactionTokens{}

		switch assetType {
		case 0: // RBT
			tokens.RBT = make([]*model.TokenInfo, numTokens)
			for k := 0; k < numTokens; k++ {
				tokens.RBT[k] = &model.TokenInfo{TokenID: fmt.Sprintf("1_%d_%d_%d", i, k, epoch)}
			}
		case 1: // FT
			name := ftNames[i%len(ftNames)]
			if testName != "" {
				name = testName
			}
			tokens.FT = make([]*model.TokenInfo, numTokens)
			for k := 0; k < numTokens; k++ {
				// Must map strictly to {Name}_{Index}_{CreatorDID} to pass validation
				tokens.FT[k] = &model.TokenInfo{TokenID: fmt.Sprintf("%s_%d_%s", name, (i*10)+k, sender)}
			}
		case 2: // NFT
			// Exactly 46 characters: Qm + 44 base58 chars.
			tokens.NFT = make([]*model.TokenInfo, numTokens)
			for k := 0; k < numTokens; k++ {
				id := fmt.Sprintf("QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6u%c%c", 'a'+(i/26), 'a'+(k%26))
				tokens.NFT[k] = &model.TokenInfo{TokenID: id, Data: "ipfs://artwork"}
			}
		case 3: // SC
			// Exactly 46 characters: Qm + 44 base58 chars.
			tokens.SmartContract = make([]*model.TokenInfo, numTokens)
			for k := 0; k < numTokens; k++ {
				id := fmt.Sprintf("QmR7XvF6T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7%c%c%c%c", 'a'+(i/26), 'a'+(k%26), 's', 'c')
				tokens.SmartContract[k] = &model.TokenInfo{TokenID: id, Data: "contract_init"}
			}
		}
		event.Transaction.TransactionInfo.Tokens = tokens

		data, _ := json.Marshal(event)
		ps.Publish("rubix_txns", data)

		if i%20 == 0 {
			log.Printf("Progress: Published %d/%d dummy transactions", i, 86)
		}
	}

	log.Printf("All 86 randomized dummy transactions published! Including shared FT 'TITAN' test case.")
}

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
	log.Printf("Publishing multiple dummy transactions with different scenarios...")

	// DIDs
	didAlice := "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq"
	didBob := "bafybmf6j7n5e4v4z7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v"
	didCharlie := "bafybmguu5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v"
	didQuorum1 := "bafybmihq4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqluabc"
	didQuorum2 := "bafybmihx4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqluxyz"

	epoch := int(time.Now().Unix())

	scenarios := []model.EventTransaction{
		// ─── Scenario 1: RBT Transfer (Alice → Bob) ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXAAAAAAAAAAAAAAAAAAAAAAAA",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didAlice,
					Owner:     didBob,
					Epoch:     epoch,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						RBT: []*model.TokenInfo{
							{TokenID: "1_1001", PreviousTransactionID: "BRX00000000000000000000000"},
							{TokenID: "2_2002", PreviousTransactionID: "BRX00000000000000000000001"},
						},
					},
					CommittedTokens: []*model.TokenInfo{
						{TokenID: "1_1001", PreviousTransactionID: "BRX00000000000000000000000"},
						{TokenID: "2_2002", PreviousTransactionID: "BRX00000000000000000000001"},
					},
					Quorums: []*model.QuorumInfo{
						{Did: didQuorum1, Tokens: []*model.TokenInfo{{TokenID: "1_5001"}}},
						{Did: didQuorum2, Tokens: []*model.TokenInfo{{TokenID: "1_5002"}}},
					},
					Memo: "RBT transfer from Alice to Bob",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_alice_rbt_transfer",
					Quorums: []model.QuorumSignature{
						{Did: didQuorum1, Signature: "bafyk_sig_q1"},
						{Did: didQuorum2, Signature: "bafyk_sig_q2"},
					},
				},
			},
			Status:  true,
			Message: "",
		},

		// ─── Scenario 2: FT Transfer (Alice → Charlie) ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXBBBBBBBBBBBBBBBBBBBBBBBB",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didAlice,
					Owner:     didCharlie,
					Epoch:     epoch + 10,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						FT: []*model.TokenInfo{
							{TokenID: fmt.Sprintf("APPLE_1_%s", didAlice), PreviousTransactionID: "BRX11111111111111111111111"},
							{TokenID: fmt.Sprintf("APPLE_2_%s", didAlice), PreviousTransactionID: "BRX11111111111111111111112"},
							{TokenID: fmt.Sprintf("MANGO_1_%s", didAlice), PreviousTransactionID: "BRX11111111111111111111113"},
						},
					},
					Quorums: []*model.QuorumInfo{
						{Did: didQuorum1, Tokens: []*model.TokenInfo{{TokenID: "1_5003"}}},
					},
					Memo: "FT transfer: 2 APPLE + 1 MANGO from Alice to Charlie",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_alice_ft_transfer",
					Quorums: []model.QuorumSignature{
						{Did: didQuorum1, Signature: "bafyk_sig_q1_ft"},
					},
				},
			},
			Status:  true,
			Message: "",
		},

		// ─── Scenario 3: NFT Mint (Bob creates NFTs) ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXCCCCCCCCCCCCCCCCCCCCCCCC",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didBob,
					Owner:     didBob,
					Epoch:     epoch + 20,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						NFT: []*model.TokenInfo{
							{TokenID: "QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco", Data: "ipfs://QmArtwork1CID"},
							{TokenID: "QmPZ9gcCEpqKTo6aq61g2cyZ9V3KyfH4G91X5vS5pL712u", Data: "ipfs://QmArtwork2CID"},
						},
					},
					Quorums: []*model.QuorumInfo{
						{Did: didQuorum1, Tokens: []*model.TokenInfo{{TokenID: "1_5004"}}},
					},
					Memo: "NFT mint by Bob",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_bob_nft_mint",
					Quorums: []model.QuorumSignature{
						{Did: didQuorum1, Signature: "bafyk_sig_q1_nft"},
					},
				},
			},
			Status:  true,
			Message: "",
		},

		// ─── Scenario 4: SC Deploy (Charlie deploys a smart contract) ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXDDDDDDDDDDDDDDDDDDDDDD",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didCharlie,
					Owner:     didCharlie,
					Epoch:     epoch + 30,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						SmartContract: []*model.TokenInfo{
							{TokenID: "QmR7XvF6T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7", Data: "ipfs://QmContractCodeCID"},
						},
					},
					Quorums: []*model.QuorumInfo{
						{Did: didQuorum2, Tokens: []*model.TokenInfo{{TokenID: "1_5005"}}},
					},
					Memo: "Smart contract deployment by Charlie",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_charlie_sc_deploy",
					Quorums: []model.QuorumSignature{
						{Did: didQuorum2, Signature: "bafyk_sig_q2_sc"},
					},
				},
			},
			Status:  true,
			Message: "",
		},

		// ─── Scenario 5: RBT Transfer (Bob → Alice) — tests balance decrement on Bob ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXEEEEEEEEEEEEEEEEEEEEEEEE",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didBob,
					Owner:     didAlice,
					Epoch:     epoch + 40,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						RBT: []*model.TokenInfo{
							{TokenID: "1_1001", PreviousTransactionID: "BRXAAAAAAAAAAAAAAAAAAAAAAAA"},
						},
					},
					CommittedTokens: []*model.TokenInfo{
						{TokenID: "1_1001", PreviousTransactionID: "BRXAAAAAAAAAAAAAAAAAAAAAAAA"},
					},
					Quorums: []*model.QuorumInfo{
						{Did: didQuorum1, Tokens: []*model.TokenInfo{{TokenID: "1_5006"}}},
					},
					Memo: "RBT transfer back: Bob returns 1_1001 to Alice",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_bob_rbt_return",
					Quorums: []model.QuorumSignature{
						{Did: didQuorum1, Signature: "bafyk_sig_q1_return"},
					},
				},
			},
			Status:  true,
			Message: "",
		},

		// ─── Scenario 6: Failed Consensus ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXFFFFFFFFFFFFFFFFFFFFFFFR",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didAlice,
					Owner:     didCharlie,
					Epoch:     epoch + 50,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						RBT: []*model.TokenInfo{
							{TokenID: "1_9999", PreviousTransactionID: "BRX99999999999999999999999"},
						},
					},
					Memo: "This transaction should fail consensus",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_alice_failed",
				},
			},
			Status:  false,
			Message: "Quorum consensus failed: insufficient quorum signatures (2/5)",
		},

		// ─── Scenario 7: Part RBT and SC with Data ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRXGGGGGGGGGGGGGGGGGGGGGGGG",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didCharlie,
					Owner:     didAlice,
					Epoch:     epoch + 60,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						RBT: []*model.TokenInfo{
							{TokenID: "1_1001_5", PreviousTransactionID: "BRXEEEEEEEEEEEEEEEEEEEEEEEE"}, // Part RBT
						},
						SmartContract: []*model.TokenInfo{
							{TokenID: "QmR7XvF6T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7T7", Data: "init_params={\"value\":100}"},
						},
					},
					Memo: "Part RBT transfer + SC data update",
				},
				Signatures: &model.Signature{
					InitiatorSignature: "bafyk_sig_charlie_part_rbt",
				},
			},
			Status: true,
		},

		// ─── Scenario 8: Mass Asset Generation (for Stats testing) ───
		{
			Transaction: &model.Transactions{
				TransactionID: "BRX_MASS_ASSETS_GEN_001",
				TransactionInfo: &model.TransactionInfo{
					Initiator: didAlice,
					Owner:     didAlice,
					Epoch:     epoch + 100,
					Network:   "TestNet",
					Tokens: &model.TransactionTokens{
						RBT: func() []*model.TokenInfo {
							tokens := make([]*model.TokenInfo, 25)
							for i := 0; i < 25; i++ {
								tokens[i] = &model.TokenInfo{TokenID: fmt.Sprintf("1_200%d", i)}
							}
							return tokens
						}(),
						FT: func() []*model.TokenInfo {
							tokens := make([]*model.TokenInfo, 25)
							for i := 0; i < 25; i++ {
								tokens[i] = &model.TokenInfo{TokenID: fmt.Sprintf("FT_TOKEN_%d_%s", i, didAlice)}
							}
							return tokens
						}(),
						NFT: func() []*model.TokenInfo {
							tokens := make([]*model.TokenInfo, 25)
							for i := 0; i < 25; i++ {
								tokens[i] = &model.TokenInfo{TokenID: fmt.Sprintf("QmNFT_MASS_%d", i)}
							}
							return tokens
						}(),
						SmartContract: func() []*model.TokenInfo {
							tokens := make([]*model.TokenInfo, 25)
							for i := 0; i < 25; i++ {
								tokens[i] = &model.TokenInfo{TokenID: fmt.Sprintf("QmSC_MASS_%d", i)}
							}
							return tokens
						}(),
					},
					Memo: "Generating 25 tokens of each type for Stats API testing",
				},
				Signatures: &model.Signature{InitiatorSignature: "bafyk_sig_mass_gen"},
			},
			Status: true,
		},
	}

	for i, event := range scenarios {
		time.Sleep(500 * time.Millisecond) // Small delay between publishes

		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("Error marshaling scenario %d: %v", i+1, err)
			continue
		}

		err = ps.Publish("rubix_txns", data)
		if err != nil {
			log.Printf("Error publishing scenario %d: %v", i+1, err)
		} else {
			status := "✓ SUCCESS"
			if !event.Status {
				status = "✗ FAILED"
			}
			log.Printf("Published Scenario %d [%s]: %s — %s",
				i+1, status, event.Transaction.TransactionID, event.Transaction.TransactionInfo.Memo)
		}
	}

	log.Printf("All %d dummy scenarios published!", len(scenarios))
}

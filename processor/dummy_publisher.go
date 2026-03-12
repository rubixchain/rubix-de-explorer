// TODO: DELETE LATER - Entire file for dummy publisher testing
package processor

import (
	"encoding/json"
	"explorer-server/database/models"
	"explorer-server/pubsub"
	"log"
	"time"
)

// PublishDummyTransaction sends a single dummy EventTransaction to the "rubix_txns" topic.
// TODO: DELETE LATER
func PublishDummyTransaction(ps *pubsub.PubSub) {
	log.Printf("Publishing single dummy PubSub transaction with realistic data...")

	// Realistic BIP-39 DIDs
	initiatorDID := "bafybmihy4panvvrjssdjqksrwjcxza6xpgnxvcyufn2wuam75idnqlugdq"
	ownerDID := "bafybmf6j7n5e4v4z7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v7v"
	quorumDID := "bafybmguu5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v5v"
	
	epoch := time.Now().Unix()

	dummyTxn := models.EventTransaction{
		Transaction: &models.Transactions{
			// Mocking a base24-like encoded string for the transaction ID
			TransactionID: "BRX1234567890ABCDEFGHJKLMN", 
			TransactionInfo: &models.TransInfo{
				Initiator: &initiatorDID,
				Owner:     &ownerDID,
				Epoch:     &epoch,
				Network:   "TestNet",
				Tokens: &models.TransactionTokens{
					RBT: []*models.TokenInfo{
						{TokenID: "1_1001", PreviousTransactionID: "BRX00000000000000000000000", Data: ""},
					},
					FT: []*models.TokenInfo{
						{TokenID: "APPLE_55_" + ownerDID, PreviousTransactionID: "BRX11111111111111111111111", Data: ""},
					},
					NFT: []*models.TokenInfo{
						{TokenID: "QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco", PreviousTransactionID: "BRX22222222222222222222222", Data: "metadata_cid_here"},
					},
				},
				CommittedTokens: []*models.TokenInfo{
					{TokenID: "1_1001", PreviousTransactionID: "BRX00000000000000000000000", Data: ""},
				},
				Quorums: []*models.QuorumInfo{
					{
						DID: quorumDID,
						TokenInfo: []*models.TokenInfo{
							{TokenID: "QmPbMvY1vPvPvPvPvPvPvPvPvPvPvPvPvPvPvPvPvPvPvPv", Data: "quorum_validation_data"},
						},
					},
				},
				Memo: "Realistic data format test",
			},
			Signatures: &models.Signatures{
				InitiatorSignature: "bafyk...initiator_sig",
				QuorumSignatures: []*models.QuorumSignatures{
					{DID: quorumDID, Signature: "bafyk...quorum_sig"},
				},
			},
		},
		Status:  true,
		Message: "",
	}

	data, err := json.Marshal(dummyTxn)
	if err != nil {
		log.Printf("Error marshaling dummy transaction: %v", err)
		return
	}

	err = ps.Publish("rubix_txns", data)
	if err != nil {
		log.Printf("Error publishing dummy transaction: %v", err)
	} else {
		log.Printf("Successfully published dummy transaction with realistic formats to rubix_txns")
	}
}

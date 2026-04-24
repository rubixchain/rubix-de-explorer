package processor

import (
	"encoding/base64"
	"encoding/json"
	"explorer-server/database/models"
	"explorer-server/model"
	"log"
)

// TxnCallBack processes incoming PubSub events (transactions or DID maps).
func TxnCallBack(peerID string, topic string, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PubSub Callback] CRITICAL: recovered from unexpected panic: %v", r)
		}
	}()

	// The IPFS pubsub HTTP API streams data as Base64 encoded payload
	decodedData, b64Err := base64.StdEncoding.DecodeString(string(data))
	if b64Err != nil {
		// If it's not base64, assume it's raw JSON
		decodedData = data
	}

	switch topic {
	case models.Event_RubixTxns:
		var newEvent model.EventTransaction
		err := json.Unmarshal(decodedData, &newEvent)
		if err != nil {
			log.Printf("Warning: Failed to parse published Transaction event: %v", err)
			return
		}
		// Hand off to the transaction processor
		HandleIncomingTxn(&newEvent)

	case models.Event_RubixDID:
		var didInfo model.DIDInfo
		err := json.Unmarshal(decodedData, &didInfo)
		if err != nil {
			log.Printf("Warning: Failed to parse published DIDInfo event: %v", err)
			return
		}
		// Hand off to the PeerID-DID processor
		HandleIncomingDIDInfo(&didInfo)

	default:
		log.Printf("Warning: Received message for unknown topic: %s", topic)
	}
}

package processor

import (
	"encoding/base64"
	"encoding/json"
	"explorer-server/model"
	"log"
)

// TxnCallBack processes incoming transaction events from the PubSub topic.
// It is now a thin wrapper that hands off to HandleIncomingTxn.
func TxnCallBack(peerID string, topic string, data []byte) {
	var newEvent model.EventTransaction

	// The IPFS pubsub HTTP API streams data as Base64 encoded payload
	decodedData, b64Err := base64.StdEncoding.DecodeString(string(data))
	if b64Err != nil {
		// If it's not base64, assume it's raw JSON
		decodedData = data
	}

	err := json.Unmarshal(decodedData, &newEvent)
	if err != nil {
		log.Printf("Warning: Failed to parse published event from PubSub: %v", err)
		return
	}

	// Hand off to the transaction processor for validation and orchestration
	HandleIncomingTxn(&newEvent)
}

package processor

import (
	"encoding/base64"
	"encoding/json"
	"explorer-server/model"
	"log"
)

// TxnCallBack processes incoming transaction events from the PubSub topic.
// Mirrors the Fullnode's TxnCallBack in core/fullnode.go.
func TxnCallBack(peerID string, topic string, data []byte) {
	var newEvent model.PubSubTxnInfo

	// The IPFS pubsub HTTP API streams m.Data as Base64 encoded payload
	decodedData, b64Err := base64.StdEncoding.DecodeString(string(data))
	if b64Err != nil {
		// If it's not base64, assume it's raw JSON and try it anyway
		decodedData = data
	}

	err := json.Unmarshal(decodedData, &newEvent)
	if err != nil {
		log.Printf("Warning: Failed to parse published event from PubSub: %v (raw data: %s)", err, string(decodedData))
		return
	}

	if newEvent.BlockHash == "" {
		log.Printf("Warning: Received PubSub message with empty BlockHash, skipping")
		return
	}

	// Enqueue the transaction into the dynamic worker pool
	// This prevents the IPFS PubSub network reader from blocking
	if GlobalTxnProcessor != nil {
		GlobalTxnProcessor.EnqueueTransaction(&newEvent)
	} else {
		log.Printf("Warning: GlobalTxnProcessor not initialized, dropping transaction %s", newEvent.BlockHash)
	}
}

package processor

import (
	"encoding/json"
	"explorer-server/model"
	"log"
)

// TxnCallBack processes incoming transaction events from the PubSub topic.
// Mirrors the Fullnode's TxnCallBack in core/fullnode.go.
func TxnCallBack(peerID string, topic string, data []byte) {
	var newEvent model.PubSubTxnInfo

	err := json.Unmarshal(data, &newEvent)
	if err != nil {
		log.Printf("⚠️ Failed to parse published event from PubSub: %v", err)
		return
	}

	if newEvent.BlockHash == "" {
		log.Printf("⚠️ Received PubSub message with empty BlockHash, skipping")
		return
	}

	// Enqueue the transaction into the dynamic worker pool
	// This prevents the IPFS PubSub network reader from blocking
	if GlobalTxnProcessor != nil {
		GlobalTxnProcessor.EnqueueTransaction(&newEvent)
	} else {
		log.Printf("⚠️ GlobalTxnProcessor not initialized, dropping transaction %s", newEvent.BlockHash)
	}
}

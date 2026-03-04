package services

import (
	"encoding/json"
	"explorer-server/model"
	"log"
)

// TxnCallBack processes incoming transaction/block events from the PubSub topic.
// This function acts as the bridge between the PubSub network layer and the block updating logic.
func TxnCallBack(peerID string, topic string, data []byte) {
	var newEvent model.IncomingBlockInfo

	err := json.Unmarshal(data, &newEvent)
	if err != nil {
		log.Printf("⚠️ Failed to parse published event from PubSub: %v", err)
		return
	}

	if newEvent.BlockHash == "" {
		log.Printf("⚠️ Received PubSub message with empty BlockHash, skipping")
		return
	}

	// Validate whether the Fullnode includes the BlockMap in the pubsub message.
	// UpdateBlocks heavily relies on the BlockMap.
	if newEvent.BlockMap == nil {
		log.Printf("⚠️ Received PubSub message for BlockHash %s but BlockMap is nil. Cannot process.", newEvent.BlockHash)
		return
	}

	log.Printf("📥 Received transaction from PubSub [%s]: %v", topic, newEvent)

	// Delegate to the standard Queue / Worker Pool if available
	// okTask := EnqueueBlockUpdateTask(func() {
	// 	UpdateBlocks(&newEvent)
	// })

	// if !okTask {
	// 	UpdateBlocks(&newEvent)
	// }
}

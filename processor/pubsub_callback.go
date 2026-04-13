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
	// The IPFS pubsub HTTP API streams data as Base64 encoded payload
	decodedData, b64Err := base64.StdEncoding.DecodeString(string(data))
	if b64Err != nil {
		// If it's not base64, assume it's raw JSON
		decodedData = data
	}

	// Debug: log the raw incoming payload (truncated to avoid flooding logs)
	if len(decodedData) > 0 {
		const maxLog = 4096
		if len(decodedData) <= maxLog {
			log.Printf("[PubSub][%s] Raw payload from %s (len=%d): %s", topic, peerID, len(decodedData), string(decodedData))
		} else {
			log.Printf("[PubSub][%s] Raw payload from %s (len=%d) (truncated): %s...", topic, peerID, len(decodedData), string(decodedData[:maxLog]))
		}
	}

	switch topic {
	case models.Event_RubixTxns:
		var newEvent model.EventTransaction
		err := json.Unmarshal(decodedData, &newEvent)
		if err != nil {
			log.Printf("Warning: Failed to parse published Transaction event: %v", err)
			return
		}
		// Debug: pretty-print the unmarshaled EventTransaction for visibility
		if b, err := json.MarshalIndent(newEvent, "", "  "); err == nil {
			log.Printf("[PubSub][%s] Parsed EventTransaction (from %s): %s", topic, peerID, string(b))
		}

		// Debug: try to pretty-print the inner Transaction.Info (raw JSON) if present
		if newEvent.Transaction != nil && newEvent.Transaction.Info != nil {
			var prettyInfo interface{}
			if err := json.Unmarshal(newEvent.Transaction.Info, &prettyInfo); err == nil {
				if ib, err := json.MarshalIndent(prettyInfo, "", "  "); err == nil {
					log.Printf("[PubSub][%s] Inner Transaction.Info (pretty): %s", topic, string(ib))
				}
			} else {
				// Fallback: raw bytes
				log.Printf("[PubSub][%s] Inner Transaction.Info (raw): %s", topic, string(newEvent.Transaction.Info))
			}
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

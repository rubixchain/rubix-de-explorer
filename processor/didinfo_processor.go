package processor

import (
	"explorer-server/database/operations"
	"explorer-server/model"
	"log"
)

// HandleIncomingDIDInfo processes the decentralized identifier to peer ID mapping.
func HandleIncomingDIDInfo(pm *model.DIDInfo) {
	log.Printf("Received DIDInfo: DID=%s, PeerID=%s", pm.DID, pm.PeerID)

	if err := operations.SaveDIDInfo(pm); err != nil {
		log.Printf("Error saving DIDInfo to database: %v", err)
		return
	}

	log.Printf("Successfully saved DIDInfo for DID: %s", pm.DID)
}

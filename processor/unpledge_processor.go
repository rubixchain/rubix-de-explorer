package processor

import (
	"explorer-server/database/operations"
	"explorer-server/model"
	"log"
)

// HandleIncomingUnpledge processes the unpledge event published by quorum nodes.
// It validates the incoming token IDs and delegates to the DB operation layer.
func HandleIncomingUnpledge(event *model.UnpledgeEvent) {
	if event == nil || len(event.UnpledgeInfo) == 0 {
		log.Println("[Unpledge] Received empty unpledge event, ignoring")
		return
	}

	log.Printf("[Unpledge] Received unpledge event with %d token(s), txnID=%s",
		len(event.UnpledgeInfo), event.UnpledgeTransactionID)

	if err := operations.ProcessUnpledgeTokens(event); err != nil {
		log.Printf("[Unpledge] Error processing unpledge tokens: %v", err)
		return
	}

	log.Printf("[Unpledge] Successfully processed unpledge event")
}

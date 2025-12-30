package router

import (
	"net/http"

	"explorer-server/handlers"

	"github.com/gorilla/mux"
)

// NewRouter returns a mux.Router with all routes wired to handlers
func NewRouter() *mux.Router {
	r := mux.NewRouter()

	// Health
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	r.HandleFunc("/api/allrbtcount", handlers.GetRBTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allftcount", handlers.GetFTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alldidcount", handlers.GetDIDCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alltransactionscount", handlers.GetTxnsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allsmartcontractscount", handlers.GetSCsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allnftcount", handlers.GetNFTsCountHandler).Methods(http.MethodGet)

	r.HandleFunc("/api/didwithmostrbts", handlers.GetDIDHoldersListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/txnblocks", handlers.GetTransferBlockListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/getdidinfo", handlers.GetDIDInfoHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/txnhash", handlers.GetBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/blockhash", handlers.GetBlockInfoFromBlockHash).Methods(http.MethodGet)
	r.HandleFunc("/api/smartcontract", handlers.GetSmartContractInfoFromSCID).Methods(http.MethodGet)
	r.HandleFunc("/api/nft", handlers.GetNFTInfoFromNFTID).Methods(http.MethodGet)
	r.HandleFunc("/api/rbt", handlers.GetRBTInfoFromRBTID).Methods(http.MethodGet)
	r.HandleFunc("/api/ft", handlers.GetFTInfoFromFTID).Methods(http.MethodGet)
	r.HandleFunc("/api/getrbtlist", handlers.GetRBTListHandler).Methods(http.MethodGet)

	r.HandleFunc("/api/search", handlers.GetInfo).Methods(http.MethodGet)
	r.HandleFunc("/api/token-chain", handlers.GetTokenChainFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/token-blocks", handlers.GetTokenBlocksFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/sc-blocks", handlers.GetSCBlockList).Methods(http.MethodGet)
	r.HandleFunc("/api/burnt-blocks", handlers.GetBurntBlockList).Methods(http.MethodGet)

	r.HandleFunc("/api/sctxn-info", handlers.GetSCBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/burnttxn-info", handlers.GetBurntTxnInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/ftholdings", handlers.GetFtHoldingList).Methods(http.MethodGet)

	// ==== New async notification endpoints ====
	r.HandleFunc("/api/block-update", handlers.UpdateBlocksHandler).Methods(http.MethodPost)

	// Worker pool / queue status (for monitoring)
	r.HandleFunc("/api/queue-status", handlers.QueueStatusHandler).Methods(http.MethodGet)

	return r
}

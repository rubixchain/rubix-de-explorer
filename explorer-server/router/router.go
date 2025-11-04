package router

import (
	"explorer-server/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter returns a mux.Router with all routes wired to handlers
func NewRouter() *mux.Router {
	r := mux.NewRouter()

	// Health and database routes
	// r.HandleFunc("/api/health", handlers.HealthHandler).Methods(http.MethodGet)
	// r.HandleFunc("/api/database/stats", handlers.DatabaseStatsHandler).Methods(http.MethodGet)

	// Database-powered API routes (primary endpoints)
	r.HandleFunc("/api/allrbtcount", handlers.GetRBTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allftcount", handlers.GetFTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alldidcount", handlers.GetDIDCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alltransactionscount", handlers.GetTxnsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allsmartcontractscount", handlers.GetSCsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allnftcount", handlers.GetNFTsCountHandler).Methods(http.MethodGet)

	// // route to get the did with most rbts
	r.HandleFunc("/api/didwithmostrbts", handlers.GetDIDHoldersListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/txnblocks", handlers.GetTransferBlockListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/getdidinfo", handlers.GetDIDInfoHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/txnhash", handlers.GetBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/blockhash", handlers.GetBlockInfoFromBlockHash).Methods(http.MethodGet)
	r.HandleFunc("/api/smartcontract", handlers.GetSmartContractInfoFromSCID).Methods(http.MethodGet)
	r.HandleFunc("/api/nft", handlers.GetNFTInfoFromNFTID).Methods(http.MethodGet) //done
	r.HandleFunc("/api/rbt", handlers.GetRBTInfoFromRBTID).Methods(http.MethodGet) //done
	r.HandleFunc("/api/ft", handlers.GetFTInfoFromFTID).Methods(http.MethodGet)    //done
	r.HandleFunc("/api/getrbtlist", handlers.GetRBTListHandler).Methods(http.MethodGet)

	r.HandleFunc("/api/search", handlers.GetInfo).Methods(http.MethodGet)
	r.HandleFunc("/api/token-chain", handlers.GetTokenChainFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/token-blocks", handlers.GetTokenBlocksFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/sc-blocks", handlers.GetSCBlockList).Methods(http.MethodGet)
	r.HandleFunc("/api/burnt-blocks", handlers.GetBurntBlockList).Methods(http.MethodGet)

	r.HandleFunc("/api/sctxn-info", handlers.GetSCBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/burnttxn-info", handlers.GetBurntTxnInfoFromTxnHash).Methods(http.MethodGet)
	// r.HandleFunc("/api/analytics", handlers.DatabaseAnalyticsHandler).Methods(http.MethodGet)
	// r.HandleFunc("/api/database", handlers.DatabaseInterfaceHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/ftholdings",handlers.GetFtHoldingList).Methods(http.MethodGet )

	// docs endpoint
	// r.HandleFunc("/api/docs", handlers.DocsHandler).Methods(http.MethodGet)

	//Block Updation endpoint
	r.HandleFunc("/api/block-update", handlers.UpdateBlocksHandler).Methods(http.MethodPost)

	return r
}

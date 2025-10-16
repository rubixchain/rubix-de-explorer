package router

import (
	"net/http"

	"explorer-server/client"
	"explorer-server/config"
	"explorer-server/handlers"
	"explorer-server/services"

	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()

	// Health and database routes
	r.HandleFunc("/api/health", handlers.HealthHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/database/stats", handlers.DatabaseStatsHandler).Methods(http.MethodGet)

	// Database-powered API routes (primary endpoints)
	r.HandleFunc("/api/allassetcount", handlers.DatabaseAllAssetCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alldidcount", handlers.DatabaseAllDIDCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/alltransactionscount", handlers.DatabaseAllDIDCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allsmartcontractscount", handlers.DatabaseAllSmartContractsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/allnftcount", handlers.DatabaseAllNFTCountHandler).Methods(http.MethodGet)

	// route to get the did with most rbts
	r.HandleFunc("/api/didwithmostrbts", handlers.DatabaseDidWithMostRBTsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/txnblocks", handlers.DatabaseBlockByIDHandler).Methods(http.MethodGet)
	// r.HandleFunc("/api/tokens", handlers.DatabaseTokensHandler).Methods(http.MethodGet). // 

	// create an api for the total transcations 
	
	r.HandleFunc("/api/txnhash", handlers.DatabaseBlocksHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/blockhash", handlers.DatabaseTransactionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/smartcontract", handlers.DatabaseBlocksHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/nft", handlers.DatabaseDIDsHandler).Methods(http.MethodGet)


    r.HandleFunc("/api/search", handlers.DatabaseSearchHandler).Methods(http.MethodGet)   
	r.HandleFunc("/api/token", handlers.DatabaseTokenByIDHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/token-chain", handlers.DatabaseTokenChainHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/did", handlers.DatabaseDIDHandler).Methods(http.MethodGet)

	r.HandleFunc("/api/analytics", handlers.DatabaseAnalyticsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/database", handlers.DatabaseInterfaceHandler).Methods(http.MethodGet)

	return r
}

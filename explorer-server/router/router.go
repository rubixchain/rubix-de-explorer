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

	// Initialize Rubix client
	rubixClient := client.NewRubixClient(config.RubixNodeURL)

	// Services
	rbtService := services.NewRBTService(rubixClient)
	ftService := services.NewFTService(rubixClient)

	// Health and database routes
	r.HandleFunc("/api/health", handlers.HealthHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/database/stats", handlers.DatabaseStatsHandler).Methods(http.MethodGet)

	// Database-powered API routes (primary endpoints)
	r.HandleFunc("/api/allassetcount", handlers.DatabaseAllAssetCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/didcount", handlers.DatabaseAllDIDCountHandler).Methods(http.MethodGet)

	// route to get the did with most rbts
	r.HandleFunc("/api/didwithmostrbts", handlers.DatabaseDidWithMostRBTsHandler).Methods(http.MethodGet)
	
	// create an api for the total transcations 
	

	r.HandleFunc("/api/tokens", handlers.DatabaseTokensHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/blocks", handlers.DatabaseBlocksHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/token", handlers.DatabaseTokenByIDHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/token-chain", handlers.DatabaseTokenChainHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/block", handlers.DatabaseBlockByIDHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/did", handlers.DatabaseDIDHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/analytics", handlers.DatabaseAnalyticsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/search", handlers.DatabaseSearchHandler).Methods(http.MethodGet)   // how its working : for dids and smart contracts and for the asset , things will be different 
	r.HandleFunc("/api/database", handlers.DatabaseInterfaceHandler).Methods(http.MethodGet)

	// Legacy routes (direct full node calls - deprecated)
	r.HandleFunc("/api/get-rbt", handlers.GetRBTHandler(rbtService)).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft", handlers.GetFTHandler(ftService)).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-token-chain", handlers.GetFTTokenchainHandler(ftService)).Methods(http.MethodGet)


	//for token search 

	return r
}

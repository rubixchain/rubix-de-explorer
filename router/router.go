package router

import (
	"log"
	"net/http"
	"time"

	"explorer-server/handlers"

	"github.com/gorilla/mux"
)

// loggingMiddleware logs incoming HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s | %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// NewRouter returns a mux.Router with all routes wired to handlers
func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Health
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Stats
	r.HandleFunc("/api/get-rbt-count", handlers.GetRBTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-count", handlers.GetFTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-nft-count", handlers.GetNFTsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-sc-count", handlers.GetSCsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-txn-count", handlers.GetTxnsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-did-count", handlers.GetDIDCountHandler).Methods(http.MethodGet)

	// Latest transactions
	r.HandleFunc("/api/get-latest-transactions", handlers.GetLatestTransactionsListHandler).Methods(http.MethodGet)

	// Top Holders
	r.HandleFunc("/api/get-did-with-most-rbts", handlers.GetDIDHoldersListHandler).Methods(http.MethodGet)

	// Tokens
	// RBT
	r.HandleFunc("/api/get-rbt-list", handlers.GetRBTListHandler).Methods(http.MethodGet)
	// FT
	r.HandleFunc("/api/get-ft-group-list", handlers.GetFTGroupListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-list-by-ftname", handlers.GetFTListByFTNameHandler).Methods(http.MethodGet)

	//SC
	r.HandleFunc("/api/get-sc-list", handlers.GetSCListHandler).Methods(http.MethodGet)

	r.HandleFunc("/api/get-txn-hash", handlers.GetBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/get-block-hash", handlers.GetBlockInfoFromBlockHash).Methods(http.MethodGet)
	r.HandleFunc("/api/get-smart-contract-info", handlers.GetSmartContractInfoFromSCID).Methods(http.MethodGet)
	r.HandleFunc("/api/get-nft-info", handlers.GetNFTInfoFromNFTID).Methods(http.MethodGet)
	r.HandleFunc("/api/get-rbt-info", handlers.GetRBTInfoFromRBTID).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-info", handlers.GetFTInfoFromFTID).Methods(http.MethodGet)

	r.HandleFunc("/api/get-info", handlers.GetInfo).Methods(http.MethodGet)
	r.HandleFunc("/api/get-token-chain", handlers.GetLatestTokenChainFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/token-blocks", handlers.GetLatestTokenBlocksFromTokenID).Methods(http.MethodGet)
	r.HandleFunc("/api/sclist", handlers.GetSCListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/sc-blocks", handlers.GetSCBlockList).Methods(http.MethodGet)
	r.HandleFunc("/api/burnt-blocks", handlers.GetBurntBlockList).Methods(http.MethodGet)

	r.HandleFunc("/api/sctxn-info", handlers.GetSCBlockInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/burnttxn-info", handlers.GetBurntTxnInfoFromTxnHash).Methods(http.MethodGet)
	r.HandleFunc("/api/ftholdings", handlers.GetFtHoldingList).Methods(http.MethodGet)

	return r
}

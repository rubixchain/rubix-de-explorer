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

	// Search
	r.HandleFunc("/api/get-info", handlers.GetInfo).Methods(http.MethodGet)

	// Stats
	r.HandleFunc("/api/get-rbt-count", handlers.GetRBTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-count", handlers.GetFTCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-nft-count", handlers.GetNFTsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-sc-count", handlers.GetSCsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-txn-count", handlers.GetTxnsCountHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-did-count", handlers.GetDIDCountHandler).Methods(http.MethodGet)

	// Latest transactions
	r.HandleFunc("/api/get-latest-transactions", handlers.GetLatestTransactionsListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/dagtxns", handlers.GetDAGTransactionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/dagtxn/{txnID}", handlers.GetDAGTxnHandler).Methods(http.MethodGet)

	// Top Holders
	r.HandleFunc("/api/get-did-with-most-rbts", handlers.GetDIDHoldersListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-did-balance", handlers.GetDIDBalanceHandler).Methods(http.MethodGet)

	// Tokens
	// RBT
	r.HandleFunc("/api/get-rbt-list", handlers.GetRBTListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/search-rbt-suggestions", handlers.GetRBTSuggestionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-rbt-info", handlers.GetRBTInfoHandler).Methods(http.MethodGet)
	// FT
	r.HandleFunc("/api/get-ft-group-list", handlers.GetFTGroupListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-list-by-ftname", handlers.GetFTListByFTNameHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/search-ft-suggestions", handlers.GetFTSuggestionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-top-holders", handlers.GetFTTopHoldersHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-ft-info", handlers.GetFTInfoHandler).Methods(http.MethodGet)

	//SC
	r.HandleFunc("/api/get-sc-list", handlers.GetSCListHandler).Methods(http.MethodGet)

	// Transaction Info
	r.HandleFunc("/api/get-txns-by-did", handlers.GetTxnsByDIDHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-transaction-info", handlers.GetTransactionInfoHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-transaction-info-list", handlers.GetTransactionInfoListHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/get-transaction-id-list", handlers.GetTransactionIDListHandler).Methods(http.MethodGet)

	// Token Info
	r.HandleFunc("/api/get-token-info", handlers.GetTokenInfoHandler).Methods(http.MethodGet)

	return r
}

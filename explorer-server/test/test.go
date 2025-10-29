package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"explorer-server/router"
)

// helper function to make GET requests and print response
func testAPI(t *testing.T, path string) {
	r := router.NewRouter()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("Failed to decode response for %s: %v", path, err)
		return
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Response from %s:\n%s\n\n", path, string(output))
}

func TestAPIs(t *testing.T) {
	// List all your API endpoints here
	endpoints := []string{
		"/api/allrbtcount",
		"/api/allftcount",
		"/api/alldidcount",
		"/api/alltransactionscount",
		"/api/allsmartcontractscount",
		"/api/allnftcount",
		"/api/didwithmostrbts",
		"/api/txnblocks",
		"/api/getdidinfo?id=some-id",
		"/api/txnhash?hash=some-hash",
		"/api/blockhash?hash=some-hash",
		"/api/smartcontract?id=some-id",
		"/api/nft?id=some-id",
		"/api/rbt?id=some-id",
		"/api/ft?id=some-id",
		"/api/getrbtlist",
		"/api/search?q=test",
	}

	for _, ep := range endpoints {
		testAPI(t, ep)
	}
}

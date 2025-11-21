package services

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

var (
	nodeHTTPClient     *http.Client
	nodeHTTPClientOnce sync.Once
)

// max parallel requests to fullnode
const maxNodeConcurrentRequests = 20

// semaphore for limiting concurrent fullnode requests
var nodeReqLimiter = make(chan struct{}, maxNodeConcurrentRequests)

// acquireNodeSlot blocks until a slot is available and returns a release func.
func acquireNodeSlot() func() {
	nodeReqLimiter <- struct{}{}
	return func() { <-nodeReqLimiter }
}

// GetNodeHTTPClient returns a singleton HTTP client configured for the fullnode.
func GetNodeHTTPClient() *http.Client {
	nodeHTTPClientOnce.Do(func() {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // fullnode runs with self-signed cert
			},
			DisableKeepAlives:   false,
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   false, // avoid implicit TLS revalidation with HTTP/2
		}

		nodeHTTPClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: tr,
		}
	})

	return nodeHTTPClient
}

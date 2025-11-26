package services

import (
	"crypto/tls"
	"net"
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
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
			IdleConnTimeout:     90 * time.Second,
		}

		nodeHTTPClient = &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second,
		}
	})
	return nodeHTTPClient
}

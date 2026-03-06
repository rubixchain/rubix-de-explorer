package services

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value and its expiry time.
type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// TTLCache is a simple goroutine-safe in-memory cache with per-key TTL.
type TTLCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

// responseCache is the global cache instance used by read services.
var responseCache = &TTLCache{items: make(map[string]cacheEntry)}

// Get retrieves a cached value. Returns (value, true) on hit, (nil, false) on miss/expired.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

// Set stores a value with a TTL duration.
func (c *TTLCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

package querybroker

import (
	"sync"
	"time"
)

// Cache is a short-TTL result cache keyed by the exact SQL text (M1.4:
// "cache résultat court TTL pour requêtes identiques") — a dashboard
// widget re-running the same query every few seconds shouldn't re-scan
// ClickHouse each time. Expiry is checked lazily on Get rather than swept
// by a background goroutine: simpler, and fine for the small number of
// distinct queries a single control-plane instance handles.
type Cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, entries: make(map[string]cacheEntry)}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	if c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.body, true
}

func (c *Cache) Set(key string, body []byte) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{body: body, expiresAt: time.Now().Add(c.ttl)}
}

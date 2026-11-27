package geocode

import (
	"sync"
	"time"
)

// ttlCache is a small in-memory cache with per-entry expiry and a size bound.
// When full, expired entries are dropped first, then the entry closest to
// expiry (the oldest, since all share one TTL).
type ttlCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	now     func() time.Time
	entries map[string]cacheEntry
}

type cacheEntry struct {
	v       any
	expires time.Time
}

func newTTLCache(ttl time.Duration, max int, now func() time.Time) *ttlCache {
	return &ttlCache{ttl: ttl, max: max, now: now, entries: map[string]cacheEntry{}}
}

func (c *ttlCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.v, true
}

func (c *ttlCache) put(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		var oldestKey string
		var oldest time.Time
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || e.expires.Before(oldest) {
				oldestKey, oldest = k, e.expires
			}
		}
		if len(c.entries) >= c.max {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = cacheEntry{v: v, expires: now.Add(c.ttl)}
}

package proxy

import (
	"net/url"
	"sync"
	"time"
)

type cacheEntry struct {
	target    *url.URL
	expiresAt time.Time
}

// routeCache is a thread-safe in-memory cache mapping sandbox names to target URLs.
type routeCache struct {
	mu  sync.RWMutex
	m   map[string]cacheEntry
	ttl time.Duration
}

func newRouteCache(ttl time.Duration) *routeCache {
	return &routeCache{
		m:   make(map[string]cacheEntry),
		ttl: ttl,
	}
}

func (c *routeCache) get(name string) (*url.URL, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.m[name]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.target, true
}

func (c *routeCache) set(name string, target *url.URL) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m[name] = cacheEntry{
		target:    target,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a sandbox from the cache.
func (c *routeCache) Invalidate(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, name)
}

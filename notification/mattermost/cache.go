package mattermost

import (
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	value  T
	expiry time.Time
}

type ttlCache[K comparable, V any] struct {
	mx    sync.RWMutex
	items map[K]cacheEntry[V]
	ttl   time.Duration
	max   int
}

func newTTLCache[K comparable, V any](max int, ttl time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{
		items: make(map[K]cacheEntry[V]),
		ttl:   ttl,
		max:   max,
	}
}

func (c *ttlCache[K, V]) Get(key K) (V, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(entry.expiry) {
		var zero V
		return zero, false
	}

	return entry.value, true
}

func (c *ttlCache[K, V]) Add(key K, value V) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if len(c.items) >= c.max {
		// simple eviction: clear oldest
		var oldestKey K
		var oldestTime time.Time
		first := true
		for k, v := range c.items {
			if first || v.expiry.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiry
				first = false
			}
		}
		delete(c.items, oldestKey)
	}

	c.items[key] = cacheEntry[V]{
		value:  value,
		expiry: time.Now().Add(c.ttl),
	}
}

func (c *ttlCache[K, V]) Delete(key K) {
	c.mx.Lock()
	defer c.mx.Unlock()
	delete(c.items, key)
}

// Package lru implements a generic LRU cache with automatic expiration.
package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache is a thread-safe LRU cache with automatic expiration.
type Cache[K comparable, V any] struct {
	mu       sync.RWMutex
	capacity int
	ttl      time.Duration
	items    map[K]*list.Element
	order    *list.List // front = most recent, back = least recent
	onEvict  func(K, V)

	// for testing
	now func() time.Time
}

// entry holds a cache entry with its expiration time.
type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// Option configures the cache.
type Option[K comparable, V any] func(*Cache[K, V])

// WithTTL sets the default TTL for cache entries.
func WithTTL[K comparable, V any](ttl time.Duration) Option[K, V] {
	return func(c *Cache[K, V]) {
		c.ttl = ttl
	}
}

// WithOnEvict sets a callback invoked when an entry is evicted.
func WithOnEvict[K comparable, V any](fn func(K, V)) Option[K, V] {
	return func(c *Cache[K, V]) {
		c.onEvict = fn
	}
}

// New creates a new LRU cache with the given capacity.
func New[K comparable, V any](capacity int, opts ...Option[K, V]) *Cache[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	c := &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		order:    list.New(),
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Set adds or updates an entry in the cache.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL adds or updates an entry with a specific TTL.
// If ttl is 0, the entry never expires.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = c.now().Add(ttl)
	}

	e := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}

	if elem, ok := c.items[key]; ok {
		// update existing
		elem.Value = e
		c.order.MoveToFront(elem)
		return
	}

	// evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	elem := c.order.PushFront(e)
	c.items[key] = elem
}

// Get retrieves an entry from the cache.
// Returns the value and true if found and not expired; otherwise zero value and false.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	e := elem.Value.(*entry[K, V])
	if !e.expiresAt.IsZero() && c.now().After(e.expiresAt) {
		// expired
		c.removeElement(elem)
		var zero V
		return zero, false
	}

	c.order.MoveToFront(elem)
	return e.value, true
}

// Peek retrieves an entry without updating its position.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	e := elem.Value.(*entry[K, V])
	if !e.expiresAt.IsZero() && c.now().After(e.expiresAt) {
		var zero V
		return zero, false
	}

	return e.value, true
}

// Delete removes an entry from the cache.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeElement(elem)
	return true
}

// Len returns the number of entries in the cache.
// Note: may include expired entries until they are accessed or evicted.
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Clear removes all entries from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.onEvict != nil {
		for _, elem := range c.items {
			e := elem.Value.(*entry[K, V])
			c.onEvict(e.key, e.value)
		}
	}

	c.items = make(map[K]*list.Element)
	c.order.Init()
}

// Keys returns all non-expired keys in the cache, from most to least recent.
func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]K, 0, c.order.Len())
	now := c.now()
	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		e := elem.Value.(*entry[K, V])
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			keys = append(keys, e.key)
		}
	}
	return keys
}

// Purge removes all expired entries from the cache.
func (c *Cache[K, V]) Purge() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	count := 0
	for elem := c.order.Back(); elem != nil; {
		prev := elem.Prev()
		e := elem.Value.(*entry[K, V])
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			c.removeElement(elem)
			count++
		}
		elem = prev
	}
	return count
}

// evictOldest removes the least recently used entry. must hold lock.
func (c *Cache[K, V]) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
}

// removeElement removes an element. must hold lock.
func (c *Cache[K, V]) removeElement(elem *list.Element) {
	e := elem.Value.(*entry[K, V])
	delete(c.items, e.key)
	c.order.Remove(elem)
	if c.onEvict != nil {
		c.onEvict(e.key, e.value)
	}
}

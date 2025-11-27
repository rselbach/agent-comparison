// Package lru provides a generic LRU cache with automatic expiration support.
package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache is a generic LRU cache with expiration support.
type Cache[K comparable, V any] struct {
	mu       sync.RWMutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
	ttl      time.Duration

	// for testing/mocking
	now func() time.Time
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// New creates a new LRU cache with the given capacity and TTL.
// If ttl is 0, items never expire.
func New[K comparable, V any](capacity int, ttl time.Duration) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 1
	}
	return &Cache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element),
		order:    list.New(),
		ttl:      ttl,
		now:      time.Now,
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, otherwise zero value and false.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	e := elem.Value.(*entry[K, V])

	// check expiration
	if c.ttl > 0 && c.now().After(e.expiresAt) {
		c.removeElement(elem)
		var zero V
		return zero, false
	}

	// move to front (most recently used)
	c.order.MoveToFront(elem)
	return e.value, true
}

// Set adds or updates a value in the cache.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = c.now().Add(c.ttl)
	}

	// update existing
	if elem, ok := c.items[key]; ok {
		e := elem.Value.(*entry[K, V])
		e.value = value
		e.expiresAt = expiresAt
		c.order.MoveToFront(elem)
		return
	}

	// evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	// add new entry
	e := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.order.PushFront(e)
	c.items[key] = elem
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// Len returns the number of items in the cache.
// Note: may include expired items that haven't been cleaned up yet.
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Clear removes all items from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[K]*list.Element)
	c.order.Init()
}

// Cleanup removes all expired items from the cache.
// Call this periodically if you want proactive cleanup.
func (c *Cache[K, V]) Cleanup() {
	if c.ttl == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	for elem := c.order.Back(); elem != nil; {
		e := elem.Value.(*entry[K, V])
		prev := elem.Prev()
		if now.After(e.expiresAt) {
			c.removeElement(elem)
		}
		elem = prev
	}
}

func (c *Cache[K, V]) evictOldest() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *Cache[K, V]) removeElement(elem *list.Element) {
	e := elem.Value.(*entry[K, V])
	delete(c.items, e.key)
	c.order.Remove(elem)
}

package lru

import (
	"container/list"
	"sync"
	"time"
)

// item stores the value and expiration.
type item[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// Cache implements an LRU cache with expiration.
type Cache[K comparable, V any] struct {
	capacity int
	ttl      time.Duration
	list     *list.List
	items    map[K]*list.Element
	mu       sync.Mutex
}

// New creates a new Cache.
// capacity must be positive. ttl of 0 means no expiration.
func New[K comparable, V any](capacity int, ttl time.Duration) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 1
	}
	return &Cache[K, V]{
		capacity: capacity,
		ttl:      ttl,
		list:     list.New(),
		items:    make(map[K]*list.Element),
	}
}

// Set adds or updates a value in the cache.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.list.MoveToFront(elem)
		it := elem.Value.(*item[K, V])
		it.value = value
		if c.ttl > 0 {
			it.expiresAt = time.Now().Add(c.ttl)
		}
		return
	}

	if c.list.Len() >= c.capacity {
		c.evictOldest()
	}

	it := &item[K, V]{
		key:   key,
		value: value,
	}
	if c.ttl > 0 {
		it.expiresAt = time.Now().Add(c.ttl)
	}

	elem := c.list.PushFront(it)
	c.items[key] = elem
}

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	it := elem.Value.(*item[K, V])

	// check expiration
	if c.ttl > 0 && time.Now().After(it.expiresAt) {
		c.removeElement(elem)
		var zero V
		return zero, false
	}

	c.list.MoveToFront(elem)
	return it.value, true
}

// Len returns the number of items in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.list.Len()
}

// evictOldest removes the oldest item.
// assumes mutex is held.
func (c *Cache[K, V]) evictOldest() {
	elem := c.list.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes an element from the list and map.
// assumes mutex is held.
func (c *Cache[K, V]) removeElement(elem *list.Element) {
	c.list.Remove(elem)
	it := elem.Value.(*item[K, V])
	delete(c.items, it.key)
}

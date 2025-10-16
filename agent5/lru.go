package agent5

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key       interface{}
	value     interface{}
	expiresAt time.Time
}

// Cache is an LRU cache with automatic expiration support.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	items    map[interface{}]*list.Element
	lru      *list.List
	ttl      time.Duration
}

// New creates a new LRU cache with the specified capacity and TTL.
// If ttl is 0, items never expire automatically.
func New(capacity int, ttl time.Duration) *Cache {
	return &Cache{
		capacity: capacity,
		items:    make(map[interface{}]*list.Element),
		lru:      list.New(),
		ttl:      ttl,
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, nil and false otherwise.
func (c *Cache) Get(key interface{}) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	e := elem.Value.(*entry)
	if c.isExpired(e) {
		c.removeElement(elem)
		return nil, false
	}

	c.lru.MoveToFront(elem)
	return e.value, true
}

// Set adds or updates a value in the cache.
func (c *Cache) Set(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.lru.MoveToFront(elem)
		e := elem.Value.(*entry)
		e.value = value
		e.expiresAt = c.getExpirationTime()
		return
	}

	e := &entry{
		key:       key,
		value:     value,
		expiresAt: c.getExpirationTime(),
	}

	elem := c.lru.PushFront(e)
	c.items[key] = elem

	if c.lru.Len() > c.capacity {
		c.evict()
	}
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

// Len returns the current number of items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[interface{}]*list.Element)
	c.lru.Init()
}

// Purge removes all expired items from the cache.
func (c *Cache) Purge() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	var next *list.Element
	for elem := c.lru.Back(); elem != nil; elem = next {
		next = elem.Prev()
		e := elem.Value.(*entry)
		if c.isExpired(e) {
			c.removeElement(elem)
			count++
		}
	}
	return count
}

func (c *Cache) evict() {
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *Cache) removeElement(elem *list.Element) {
	c.lru.Remove(elem)
	e := elem.Value.(*entry)
	delete(c.items, e.key)
}

func (c *Cache) isExpired(e *entry) bool {
	if c.ttl == 0 {
		return false
	}
	return time.Now().After(e.expiresAt)
}

func (c *Cache) getExpirationTime() time.Time {
	if c.ttl == 0 {
		return time.Time{}
	}
	return time.Now().Add(c.ttl)
}

// Package lrucache implements a thread-safe Least Recently Used cache with automatic expiration.
package lrucache

import (
	"container/list"
	"sync"
	"time"
)

// entry represents an item in the cache with its expiration time.
type entry struct {
	key       string
	value     any
	expiresAt time.Time
	element   *list.Element
}

// LRUCache implements a thread-safe Least Recently Used cache with automatic expiration.
// It uses a doubly-linked list for O(1) LRU operations and a map for O(1) key-based access.
type LRUCache struct {
	mu        sync.RWMutex
	capacity  int
	items     map[string]*entry
	evictList *list.List
	stopChan  chan struct{}
}

// New creates a new LRUCache with the specified capacity.
// The cache starts a background goroutine to clean up expired items.
func New(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 1
	}

	c := &LRUCache{
		capacity:  capacity,
		items:     make(map[string]*entry),
		evictList: list.New(),
		stopChan:  make(chan struct{}),
	}

	// start cleanup goroutine
	go c.cleanupExpired()

	return c
}

// Set adds a value to the cache with the specified TTL (time to live).
// If the key already exists, it updates the value and expiration time.
// If the cache is full, it evicts the least recently used item.
func (c *LRUCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// calculate expiration time
	expiresAt := time.Now().Add(ttl)

	// if key exists, update it
	if ent, exists := c.items[key]; exists {
		ent.value = value
		ent.expiresAt = expiresAt
		c.evictList.MoveToFront(ent.element)
		return
	}

	// add new entry
	ent := &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	ent.element = c.evictList.PushFront(ent)
	c.items[key] = ent

	// check if we need to evict
	if len(c.items) > c.capacity {
		c.evictLRU()
	}
}

// Get retrieves a value from the cache.
// It returns the value and a boolean indicating if the key was found and not expired.
func (c *LRUCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// check if expired
	if time.Now().After(ent.expiresAt) {
		c.removeEntry(ent)
		return nil, false
	}

	// move to front (most recently used)
	c.evictList.MoveToFront(ent.element)
	return ent.value, true
}

// Delete removes a key from the cache.
// It returns true if the key was found and removed.
func (c *LRUCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, exists := c.items[key]
	if !exists {
		return false
	}

	c.removeEntry(ent)
	return true
}

// Clear removes all items from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*entry)
	c.evictList.Init()
}

// Len returns the number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Close stops the cleanup goroutine and clears the cache.
func (c *LRUCache) Close() {
	close(c.stopChan)
	c.Clear()
}

// evictLRU removes the least recently used item from the cache.
// this must be called with the write lock held.
func (c *LRUCache) evictLRU() {
	element := c.evictList.Back()
	if element != nil {
		c.removeElement(element)
	}
}

// removeEntry removes an entry from the cache.
// this must be called with the write lock held.
func (c *LRUCache) removeEntry(ent *entry) {
	delete(c.items, ent.key)
	c.evictList.Remove(ent.element)
}

// removeElement removes an element from the eviction list and its corresponding entry.
// this must be called with the write lock held.
func (c *LRUCache) removeElement(element *list.Element) {
	if element == nil {
		return
	}

	ent := element.Value.(*entry)
	delete(c.items, ent.key)
	c.evictList.Remove(element)
}

// cleanupExpired runs in a goroutine and periodically removes expired items.
func (c *LRUCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopChan:
			return
		}
	}
}

// removeExpired removes all expired items from the cache.
func (c *LRUCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var next *list.Element

	for element := c.evictList.Back(); element != nil; element = next {
		next = element.Prev() // save next before we potentially remove current

		ent := element.Value.(*entry)
		if now.After(ent.expiresAt) {
			c.removeElement(element)
		}
	}
}

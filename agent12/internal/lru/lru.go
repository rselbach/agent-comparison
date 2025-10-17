package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache is an LRU cache with automatic expiration support.
type Cache struct {
	maxSize   int
	items     map[string]*list.Element
	list      *list.List
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// entry holds a cache value with its expiration time.
type entry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

// New creates a new LRU cache with the specified maximum size and cleanup interval.
// The cache will automatically remove expired entries.
// If cleanupInterval is 0, a default of 1 minute is used.
func New(maxSize int, cleanupInterval time.Duration) *Cache {
	if maxSize <= 0 {
		panic("lru: maxSize must be greater than 0")
	}

	if cleanupInterval == 0 {
		cleanupInterval = time.Minute
	}

	c := &Cache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		list:    list.New(),
		stopCh:  make(chan struct{}),
	}

	// start background cleanup goroutine
	c.wg.Add(1)
	go c.cleanup(cleanupInterval)

	return c
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, or nil and false otherwise.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		return nil, false
	}

	ent := elem.Value.(*entry)

	// check if expired (skip check if expiresAt is zero, meaning no expiration)
	if !ent.expiresAt.IsZero() && time.Now().After(ent.expiresAt) {
		c.removeElement(elem)
		return nil, false
	}

	// move to front (most recently used)
	c.list.MoveToFront(elem)

	return ent.value, true
}

// Set adds or updates a value in the cache with the specified TTL (time to live).
// If TTL is 0 or negative, the item never expires.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	// if ttl <= 0, leave expiresAt as zero value to indicate no expiration

	// check if key already exists
	if elem, exists := c.items[key]; exists {
		// update existing entry
		ent := elem.Value.(*entry)
		ent.value = value
		ent.expiresAt = expiresAt
		c.list.MoveToFront(elem)
		return
	}

	// add new entry
	ent := &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.list.PushFront(ent)
	c.items[key] = elem

	// evict least recently used if over capacity
	if c.list.Len() > c.maxSize {
		c.evict()
	}
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.removeElement(elem)
	}
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.list.Init()
	c.items = make(map[string]*list.Element)
}

// Len returns the current number of non-expired items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	count := 0

	for elem := c.list.Front(); elem != nil; elem = elem.Next() {
		ent := elem.Value.(*entry)
		// count items that never expire or haven't expired yet
		if ent.expiresAt.IsZero() || now.Before(ent.expiresAt) {
			count++
		}
	}

	return count
}

// Close stops the background cleanup goroutine and waits for it to finish.
// It is safe to call Close multiple times.
func (c *Cache) Close() {
	c.closeOnce.Do(func() {
		close(c.stopCh)
		c.wg.Wait()
	})
}

// removeElement removes an element from both the list and the map.
// must be called with lock held.
func (c *Cache) removeElement(elem *list.Element) {
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
	c.list.Remove(elem)
}

// evict removes the least recently used item from the cache.
// must be called with lock held.
func (c *Cache) evict() {
	elem := c.list.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// cleanup periodically removes expired entries from the cache.
func (c *Cache) cleanup(interval time.Duration) {
	defer c.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.removeExpired()
		}
	}
}

// removeExpired removes all expired entries from the cache.
func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var toRemove []*list.Element

	// collect expired elements
	for elem := c.list.Front(); elem != nil; elem = elem.Next() {
		ent := elem.Value.(*entry)
		// skip items that never expire (expiresAt.IsZero())
		if !ent.expiresAt.IsZero() && now.After(ent.expiresAt) {
			toRemove = append(toRemove, elem)
		}
	}

	// remove expired elements
	for _, elem := range toRemove {
		c.removeElement(elem)
	}
}

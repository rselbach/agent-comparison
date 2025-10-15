package lrucache

import (
	"container/list"
	"sync"
	"time"
)

// entry is used to hold a value in the cache.
type entry struct {
	key      interface{}
	value    interface{}
	expiresAt time.Time
}

// Cache is an LRU cache. It is safe for concurrent access.
type Cache struct {
	maxEntries int
	ll         *list.List
	cache      map[interface{}]*list.Element
	mu         sync.Mutex
}

// New creates a new Cache.
func New(maxEntries int) *Cache {
	return &Cache{
		maxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[interface{}]*list.Element),
	}
}

// Add adds a value to the cache.
func (c *Cache) Add(key, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		c.cache = make(map[interface{}]*list.Element)
		c.ll = list.New()
	}

	if ee, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ee)
		ee.Value.(*entry).value = value
		ee.Value.(*entry).expiresAt = time.Now().Add(ttl)
		return
	}

	ele := c.ll.PushFront(&entry{key, value, time.Now().Add(ttl)})
	c.cache[key] = ele

	if c.maxEntries != 0 && c.ll.Len() > c.maxEntries {
		c.removeOldest()
	}
}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key interface{}) (value interface{}, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		return
	}

	if ele, hit := c.cache[key]; hit {
		if time.Now().After(ele.Value.(*entry).expiresAt) {
			c.removeElement(ele)
			return nil, false
		}
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.removeElement(ele)
	}
}

// removeOldest removes the oldest item from the cache.
func (c *Cache) removeOldest() {
	if c.cache == nil {
		return
	}
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

// removeElement is used to remove a given list element from the cache
func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	delete(c.cache, kv.key)
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		return 0
	}
	return c.ll.Len()
}

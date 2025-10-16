package lrucache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key        interface{}
	value      interface{}
	expiration time.Time
}

type Cache struct {
	mu       sync.RWMutex
	capacity int
	ttl      time.Duration
	items    map[interface{}]*list.Element
	lru      *list.List
	stopCh   chan struct{}
}

func New(capacity int, ttl time.Duration) *Cache {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	c := &Cache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[interface{}]*list.Element),
		lru:      list.New(),
		stopCh:   make(chan struct{}),
	}

	if ttl > 0 {
		go c.cleanupExpired()
	}

	return c
}

func (c *Cache) Set(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Time{}
	if c.ttl > 0 {
		expiration = time.Now().Add(c.ttl)
	}

	if elem, exists := c.items[key]; exists {
		c.lru.MoveToFront(elem)
		e := elem.Value.(*entry)
		e.value = value
		e.expiration = expiration
		return
	}

	if c.lru.Len() >= c.capacity {
		c.evictOldest()
	}

	e := &entry{
		key:        key,
		value:      value,
		expiration: expiration,
	}

	elem := c.lru.PushFront(e)
	c.items[key] = elem
}

func (c *Cache) Get(key interface{}) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		return nil, false
	}

	e := elem.Value.(*entry)

	if !e.expiration.IsZero() && time.Now().After(e.expiration) {
		c.removeElement(elem)
		return nil, false
	}

	c.lru.MoveToFront(elem)
	return e.value, true
}

func (c *Cache) Delete(key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.removeElement(elem)
	}
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[interface{}]*list.Element)
	c.lru.Init()
}

func (c *Cache) Close() {
	close(c.stopCh)
}

func (c *Cache) evictOldest() {
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

func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpiredItems()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cache) removeExpiredItems() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for elem := c.lru.Back(); elem != nil; {
		e := elem.Value.(*entry)
		if !e.expiration.IsZero() && now.After(e.expiration) {
			next := elem.Prev()
			c.removeElement(elem)
			elem = next
		} else {
			elem = elem.Prev()
		}
	}
}

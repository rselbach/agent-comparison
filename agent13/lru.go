package agent13

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key        string
	value      interface{}
	expiration time.Time
}

type Cache struct {
	mu          sync.RWMutex
	capacity    int
	items       map[string]*list.Element
	evictList   *list.List
	stopCleanup chan struct{}
}

func New(capacity int, cleanupInterval time.Duration) *Cache {
	if capacity <= 0 {
		capacity = 100
	}

	c := &Cache{
		capacity:    capacity,
		items:       make(map[string]*list.Element),
		evictList:   list.New(),
		stopCleanup: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go c.cleanupExpired(cleanupInterval)
	}

	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Time{}
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	if elem, exists := c.items[key]; exists {
		c.evictList.MoveToFront(elem)
		elem.Value.(*entry).value = value
		elem.Value.(*entry).expiration = expiration
		return
	}

	ent := &entry{
		key:        key,
		value:      value,
		expiration: expiration,
	}
	elem := c.evictList.PushFront(ent)
	c.items[key] = elem

	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		return nil, false
	}

	ent := elem.Value.(*entry)
	if !ent.expiration.IsZero() && time.Now().After(ent.expiration) {
		c.removeElement(elem)
		return nil, false
	}

	c.evictList.MoveToFront(elem)
	return ent.value, true
}

func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		return false
	}

	c.removeElement(elem)
	return true
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictList.Len()
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

func (c *Cache) Close() {
	close(c.stopCleanup)
}

func (c *Cache) removeOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *Cache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
}

func (c *Cache) cleanupExpired(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpiredItems()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *Cache) removeExpiredItems() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var toRemove []*list.Element

	for elem := c.evictList.Back(); elem != nil; elem = elem.Prev() {
		ent := elem.Value.(*entry)
		if !ent.expiration.IsZero() && now.After(ent.expiration) {
			toRemove = append(toRemove, elem)
		}
	}

	for _, elem := range toRemove {
		c.removeElement(elem)
	}
}

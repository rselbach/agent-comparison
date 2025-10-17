package agent14

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("key not found")

type entry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

type Cache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	stopCh   chan struct{}
}

type Config struct {
	Capacity        int
	CleanupInterval time.Duration
}

func New(cfg Config) *Cache {
	capacity := cfg.Capacity
	if capacity <= 0 {
		capacity = 128
	}

	c := &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
		stopCh:   make(chan struct{}),
	}

	if cfg.CleanupInterval > 0 {
		go c.startCleanup(cfg.CleanupInterval)
	}

	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	if elem, ok := c.items[key]; ok {
		ent := elem.Value.(*entry)
		ent.value = value
		ent.expiresAt = expiresAt
		c.order.MoveToFront(elem)
		return
	}

	ent := &entry{key: key, value: value, expiresAt: expiresAt}
	elem := c.order.PushFront(ent)
	c.items[key] = elem

	if len(c.items) > c.capacity {
		c.removeOldestLocked()
	}
}

func (c *Cache) Get(key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, ErrNotFound
	}

	ent := elem.Value.(*entry)
	if ent.expiresAt.IsZero() || time.Now().Before(ent.expiresAt) {
		c.order.MoveToFront(elem)
		return ent.value, nil
	}

	c.removeElementLocked(elem)
	return nil, ErrNotFound
}

func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeElementLocked(elem)
	return true
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element, c.capacity)
	c.order.Init()
}

func (c *Cache) Close() {
	close(c.stopCh)
}

func (c *Cache) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for elem := c.order.Back(); elem != nil; {
		prev := elem.Prev()
		ent := elem.Value.(*entry)
		if !ent.expiresAt.IsZero() && now.After(ent.expiresAt) {
			c.removeElementLocked(elem)
		}
		elem = prev
	}
}

func (c *Cache) removeOldestLocked() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElementLocked(elem)
	}
}

func (c *Cache) removeElementLocked(elem *list.Element) {
	c.order.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
}

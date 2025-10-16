package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache stores values up to a fixed capacity using an LRU eviction policy and optional per-item expiration.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	entries  map[K]*list.Element
	order    *list.List
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	timer     *time.Timer
}

// New constructs a cache with the provided capacity. Capacity must be greater than zero.
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		panic("lru: capacity must be greater than zero")
	}

	return &Cache[K, V]{
		capacity: capacity,
		entries:  make(map[K]*list.Element, capacity),
		order:    list.New(),
	}
}

// Set stores value for key with the provided ttl. A ttl of zero or less disables expiration.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		ent := elem.Value.(*entry[K, V])
		ent.value = value
		ent.expiresAt = expirationTime(now, ttl)
		if ent.timer != nil {
			if !ent.timer.Stop() {
				// timer already fired or is running; allow callback to observe updated expiration
			}
			ent.timer = nil
		}
		if ttl > 0 {
			ent.timer = c.scheduleExpiration(key, ent.expiresAt)
		}
		c.order.MoveToFront(elem)
		return
	}

	ent := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: expirationTime(now, ttl),
	}
	if ttl > 0 {
		ent.timer = c.scheduleExpiration(key, ent.expiresAt)
	}

	elem := c.order.PushFront(ent)
	c.entries[key] = elem
	if c.order.Len() > c.capacity {
		c.evictOldestLocked()
	}
}

// Get retrieves the value for key. The boolean indicates whether the value was present and not expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return zero, false
	}

	ent := elem.Value.(*entry[K, V])
	if ent.expiresAt.IsZero() || time.Now().Before(ent.expiresAt) {
		c.order.MoveToFront(elem)
		return ent.value, true
	}

	c.removeElementLocked(elem)
	return zero, false
}

// Delete removes key from the cache, returning true if it was present.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return false
	}

	c.removeElementLocked(elem)
	return true
}

// Len reports the number of items currently stored in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.order.Len()
}

func (c *Cache[K, V]) scheduleExpiration(key K, expiresAt time.Time) *time.Timer {
	delay := time.Until(expiresAt)
	if delay < 0 {
		delay = 0
	}

	return time.AfterFunc(delay, func() {
		c.expire(key, expiresAt)
	})
}

func (c *Cache[K, V]) expire(key K, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return
	}

	ent := elem.Value.(*entry[K, V])
	if !ent.expiresAt.Equal(expiresAt) {
		return
	}

	c.removeElementLocked(elem)
}

func (c *Cache[K, V]) evictOldestLocked() {
	elem := c.order.Back()
	if elem == nil {
		return
	}

	c.removeElementLocked(elem)
}

func (c *Cache[K, V]) removeElementLocked(elem *list.Element) {
	c.order.Remove(elem)
	ent := elem.Value.(*entry[K, V])
	delete(c.entries, ent.key)
	if ent.timer != nil {
		if !ent.timer.Stop() {
			// timer has already fired; allow callback to exit via expiration check
		}
		ent.timer = nil
	}
}

func expirationTime(now time.Time, ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}

	return now.Add(ttl)
}

package lru

import (
	"container/list"
	"sync"
	"time"
)

// Option configures cache behavior during construction.
type Option func(*options)

type options struct {
	defaultTTL      time.Duration
	cleanupInterval time.Duration
}

// WithTTL sets a default time-to-live applied to entries inserted with Set.
// A zero duration disables expiration, allowing entries to live until evicted
// by LRU policy or explicit removal.
func WithTTL(ttl time.Duration) Option {
	return func(o *options) {
		if ttl < 0 {
			ttl = 0
		}
		o.defaultTTL = ttl
	}
}

// WithCleanupInterval enables background cleanup of expired entries on the
// provided interval. Passing a non-positive duration disables the background
// sweeper.
func WithCleanupInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval <= 0 {
			interval = 0
		}
		o.cleanupInterval = interval
	}
}

// Cache implements a size-bound least-recently-used cache with optional TTL
// based expiration. Cache provides safe concurrent access.
type Cache[K comparable, V any] struct {
	mu              sync.Mutex
	capacity        int
	defaultTTL      time.Duration
	items           map[K]*list.Element
	evictionList    *list.List
	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

type entry[K comparable, V any] struct {
	key     K
	value   V
	expires time.Time
}

// New constructs an LRU cache with the provided capacity. Capacity must be
// greater than zero.
func New[K comparable, V any](capacity int, opts ...Option) *Cache[K, V] {
	if capacity <= 0 {
		panic("lru: capacity must be greater than zero")
	}

	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	c := &Cache[K, V]{
		capacity:        capacity,
		defaultTTL:      o.defaultTTL,
		items:           make(map[K]*list.Element, capacity),
		evictionList:    list.New(),
		cleanupInterval: o.cleanupInterval,
	}

	if c.cleanupInterval > 0 {
		c.stopCh = make(chan struct{})
		go c.runCleanup()
	}

	return c
}

// Close stops the background cleanup goroutine, if one was started.
func (c *Cache[K, V]) Close() {
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
}

// Set inserts or updates the value for key, applying the cache default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL inserts or updates the value for key using the provided TTL. A TTL
// of zero or negative disables expiration for that entry.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(time.Now())

	if element, ok := c.items[key]; ok {
		ent := element.Value.(*entry[K, V])
		ent.value = value
		ent.expires = c.expiryTime(ttl)
		c.evictionList.MoveToFront(element)
		return
	}

	for c.evictionList.Len() >= c.capacity {
		c.removeOldestLocked()
	}

	ent := &entry[K, V]{
		key:     key,
		value:   value,
		expires: c.expiryTime(ttl),
	}

	c.items[key] = c.evictionList.PushFront(ent)
}

// Get returns the value associated with key. The boolean result indicates
// whether the value was present and unexpired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	ent := element.Value.(*entry[K, V])
	if c.isExpired(ent, time.Now()) {
		c.removeElementLocked(element)
		var zero V
		return zero, false
	}

	c.evictionList.MoveToFront(element)
	return ent.value, true
}

// Peek returns the value associated with key without updating its recency.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	ent := element.Value.(*entry[K, V])
	if c.isExpired(ent, time.Now()) {
		c.removeElementLocked(element)
		var zero V
		return zero, false
	}

	return ent.value, true
}

// Delete removes key from the cache if present, returning true when an entry
// was removed.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElementLocked(element)
	return true
}

// Len returns the number of currently stored (non-expired) entries.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(time.Now())
	return c.evictionList.Len()
}

// Cleanup removes expired entries immediately.
func (c *Cache[K, V]) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.purgeExpiredLocked(time.Now())
}

func (c *Cache[K, V]) runCleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Cleanup()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cache[K, V]) expiryTime(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

func (c *Cache[K, V]) isExpired(ent *entry[K, V], now time.Time) bool {
	if ent.expires.IsZero() {
		return false
	}
	return now.After(ent.expires)
}

func (c *Cache[K, V]) purgeExpiredLocked(now time.Time) int {
	removed := 0
	for element := c.evictionList.Back(); element != nil; {
		prev := element.Prev()
		ent := element.Value.(*entry[K, V])
		if !c.isExpired(ent, now) {
			element = prev
			continue
		}
		c.evictionList.Remove(element)
		delete(c.items, ent.key)
		removed++
		element = prev
	}
	return removed
}

func (c *Cache[K, V]) removeOldestLocked() {
	element := c.evictionList.Back()
	if element == nil {
		return
	}
	c.removeElementLocked(element)
}

func (c *Cache[K, V]) removeElementLocked(element *list.Element) {
	c.evictionList.Remove(element)
	ent := element.Value.(*entry[K, V])
	delete(c.items, ent.key)
}

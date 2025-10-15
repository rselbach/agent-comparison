package lru

import (
	"errors"
	"sync"
	"time"
)

// ErrInvalidCapacity is returned when New is called with a non-positive capacity.
var ErrInvalidCapacity = errors.New("lru: capacity must be positive")

// Cache implements a concurrency-safe LRU cache with optional per-entry expiry.
type Cache[K comparable, V any] struct {
	mu              sync.Mutex
	capacity        int
	entries         map[K]*entry[K, V]
	head            *entry[K, V]
	tail            *entry[K, V]
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
	doneCh          chan struct{}
	now             func() time.Time
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	prev      *entry[K, V]
	next      *entry[K, V]
}

// Option configures cache behaviour.
type Option func(*options)

type options struct {
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	now             func() time.Time
}

// WithDefaultTTL sets the default TTL applied when using Set.
// A non-positive TTL disables expiry unless a custom TTL is provided at insertion time.
func WithDefaultTTL(ttl time.Duration) Option {
	return func(opt *options) {
		opt.defaultTTL = ttl
	}
}

// WithCleanupInterval overrides the interval used by the background sweeper.
// A non-positive value disables background cleanup.
func WithCleanupInterval(interval time.Duration) Option {
	return func(opt *options) {
		opt.cleanupInterval = interval
	}
}

// WithNow customises the clock used for determining expiry.
// Intended for testing.
func WithNow(now func() time.Time) Option {
	return func(opt *options) {
		opt.now = now
	}
}

// New constructs an LRU cache with the provided capacity.
func New[K comparable, V any](capacity int, opts ...Option) (*Cache[K, V], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	cfg := options{
		defaultTTL:      0,
		cleanupInterval: 0,
		now:             time.Now,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	cache := &Cache[K, V]{
		capacity:        capacity,
		entries:         make(map[K]*entry[K, V], capacity),
		defaultTTL:      cfg.defaultTTL,
		cleanupInterval: cfg.cleanupInterval,
		now:             cfg.now,
	}

	// Default cleanup interval if TTL is enabled but no interval configured.
	if cache.defaultTTL > 0 && cache.cleanupInterval <= 0 {
		cache.cleanupInterval = clampDuration(cache.defaultTTL/2, 10*time.Millisecond, cache.defaultTTL)
	}

	if cache.cleanupInterval > 0 {
		cache.startCleaner()
	}

	return cache, nil
}

// Close stops background cleanup. After Close the cache remains usable but no background sweeps run.
func (c *Cache[K, V]) Close() {
	c.mu.Lock()
	stopCh := c.stopCh
	doneCh := c.doneCh
	if stopCh == nil {
		c.mu.Unlock()
		return
	}
	c.stopCh = nil
	c.doneCh = nil
	close(stopCh)
	c.mu.Unlock()

	<-doneCh
}

// Set stores value under the provided key using the cache's default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores value under key applying ttl. Non-positive ttl disables expiry for that entry.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeExpiredLocked()

	if existing, ok := c.entries[key]; ok {
		existing.value = value
		existing.expiresAt = c.computeExpiry(ttl)
		c.moveToFront(existing)
		return
	}

	if len(c.entries) >= c.capacity {
		c.evictLRU()
	}

	item := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: c.computeExpiry(ttl),
	}
	c.insertAtFront(item)
	c.entries[key] = item
}

// Get retrieves the value associated with key.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.entries[key]; ok {
		if item.expiresAt.IsZero() || !c.now().After(item.expiresAt) {
			c.moveToFront(item)
			return item.value, true
		}

		c.removeEntry(item)
		delete(c.entries, key)
	}

	var zero V
	return zero, false
}

// Delete removes key from the cache.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.entries[key]; ok {
		c.removeEntry(item)
		delete(c.entries, key)
		return true
	}
	return false
}

// Len reports the number of non-expired entries.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked()
	return len(c.entries)
}

func (c *Cache[K, V]) startCleaner() {
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	ticker := time.NewTicker(c.cleanupInterval)
	go func() {
		defer close(c.doneCh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.cleanupExpired()
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *Cache[K, V]) cleanupExpired() {
	c.mu.Lock()
	c.removeExpiredLocked()
	c.mu.Unlock()
}

func (c *Cache[K, V]) removeExpiredLocked() {
	if len(c.entries) == 0 {
		return
	}

	now := c.now()
	for key, item := range c.entries {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			c.removeEntry(item)
			delete(c.entries, key)
		}
	}
}

func (c *Cache[K, V]) evictLRU() {
	// Attempt to drop expired items first.
	if c.removeTailExpired() {
		return
	}

	if c.tail == nil {
		return
	}

	evicted := c.tail
	c.removeEntry(evicted)
	delete(c.entries, evicted.key)
}

func (c *Cache[K, V]) removeTailExpired() bool {
	now := c.now()
	cursor := c.tail
	evicted := false
	for cursor != nil {
		if cursor.expiresAt.IsZero() || !now.After(cursor.expiresAt) {
			break
		}
		prev := cursor.prev
		c.removeEntry(cursor)
		delete(c.entries, cursor.key)
		cursor = prev
		evicted = true
	}
	return evicted
}

func (c *Cache[K, V]) computeExpiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return c.now().Add(ttl)
}

func (c *Cache[K, V]) insertAtFront(item *entry[K, V]) {
	item.prev = nil
	item.next = c.head
	if c.head != nil {
		c.head.prev = item
	} else {
		c.tail = item
	}
	c.head = item
}

func (c *Cache[K, V]) moveToFront(item *entry[K, V]) {
	if c.head == item {
		return
	}
	c.removeEntry(item)
	c.insertAtFront(item)
}

func (c *Cache[K, V]) removeEntry(item *entry[K, V]) {
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		c.head = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	} else {
		c.tail = item.prev
	}
	item.prev = nil
	item.next = nil
}

func clampDuration(value, min, max time.Duration) time.Duration {
	if min > 0 && value < min {
		value = min
	}
	if max > 0 && value > max {
		value = max
	}
	if value <= 0 {
		return min
	}
	return value
}

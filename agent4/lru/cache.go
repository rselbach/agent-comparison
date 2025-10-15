package lru

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var (
	// ErrInvalidCapacity indicates that the cache capacity was not greater than zero.
	ErrInvalidCapacity = errors.New("lru: capacity must be positive")
	// ErrNegativeTTL indicates that a negative TTL was supplied.
	ErrNegativeTTL = errors.New("lru: ttl must be non-negative")
)

const defaultCleanupInterval = time.Second

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

type config struct {
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	clock           func() time.Time
}

// Option configures cache construction.
type Option func(*config)

// WithDefaultTTL sets a default TTL applied by Set.
func WithDefaultTTL(ttl time.Duration) Option {
	return func(cfg *config) {
		cfg.defaultTTL = ttl
	}
}

// WithCleanupInterval overrides the interval used for expiration sweeps.
func WithCleanupInterval(interval time.Duration) Option {
	return func(cfg *config) {
		cfg.cleanupInterval = interval
	}
}

// WithClock overrides the clock used to make expiration decisions.
func WithClock(clock func() time.Time) Option {
	return func(cfg *config) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// Cache implements an LRU cache with TTL-based expiration.
type Cache[K comparable, V any] struct {
	mu         sync.Mutex
	capacity   int
	entries    map[K]*list.Element
	order      *list.List
	defaultTTL time.Duration

	cleanupInterval time.Duration
	clock           func() time.Time
	stopOnce        sync.Once
	stopCh          chan struct{}
}

// New constructs a Cache with the provided capacity and options.
func New[K comparable, V any](capacity int, opts ...Option) (*Cache[K, V], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	cfg := config{
		cleanupInterval: defaultCleanupInterval,
		clock:           time.Now,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.defaultTTL < 0 {
		return nil, ErrNegativeTTL
	}

	if cfg.cleanupInterval <= 0 {
		cfg.cleanupInterval = defaultCleanupInterval
	}

	if cfg.clock == nil {
		cfg.clock = time.Now
	}

	cache := &Cache[K, V]{
		capacity:        capacity,
		entries:         make(map[K]*list.Element, capacity),
		order:           list.New(),
		defaultTTL:      cfg.defaultTTL,
		cleanupInterval: cfg.cleanupInterval,
		clock:           cfg.clock,
		stopCh:          make(chan struct{}),
	}

	go cache.runCleanup()

	return cache, nil
}

// Set inserts or updates the value for key using the default TTL if configured.
func (c *Cache[K, V]) Set(key K, value V) error {
	return c.SetWithTTL(key, value, 0)
}

// SetWithTTL inserts or updates key with an explicit TTL.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) error {
	if ttl < 0 {
		return ErrNegativeTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ttlToUse := ttl
	if ttlToUse == 0 {
		ttlToUse = c.defaultTTL
	}

	var expiresAt time.Time
	if ttlToUse > 0 {
		expiresAt = c.now().Add(ttlToUse)
	}

	if elem, ok := c.entries[key]; ok {
		ent := elem.Value.(*entry[K, V])
		ent.value = value
		ent.expiresAt = expiresAt
		c.order.MoveToFront(elem)
		return nil
	}

	ent := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.order.PushFront(ent)
	c.entries[key] = elem
	c.enforceCapacityLocked()
	return nil
}

// Get retrieves the value for key if present and not expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V

	elem, ok := c.entries[key]
	if !ok {
		return zero, false
	}

	ent := elem.Value.(*entry[K, V])
	now := c.now()
	if c.isExpired(ent, now) {
		c.removeElementLocked(elem)
		return zero, false
	}

	c.order.MoveToFront(elem)
	return ent.value, true
}

// Delete removes key if it exists.
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

// Len returns the number of active entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeExpiredLocked(c.now())
	return c.order.Len()
}

// Capacity returns the cache capacity.
func (c *Cache[K, V]) Capacity() int {
	return c.capacity
}

// Close stops the background cleanup goroutine.
func (c *Cache[K, V]) Close() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *Cache[K, V]) now() time.Time {
	if c.clock == nil {
		return time.Now()
	}
	return c.clock()
}

func (c *Cache[K, V]) runCleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpiredEntries()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cache[K, V]) removeExpiredEntries() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeExpiredLocked(c.now())
}

func (c *Cache[K, V]) removeExpiredLocked(now time.Time) {
	for elem := c.order.Back(); elem != nil; {
		prev := elem.Prev()
		ent := elem.Value.(*entry[K, V])
		if c.isExpired(ent, now) {
			c.removeElementLocked(elem)
		}
		elem = prev
	}
}

func (c *Cache[K, V]) enforceCapacityLocked() {
	for c.order.Len() > c.capacity {
		tail := c.order.Back()
		if tail == nil {
			return
		}
		c.removeElementLocked(tail)
	}
}

func (c *Cache[K, V]) removeElementLocked(elem *list.Element) {
	if elem == nil {
		return
	}
	ent := elem.Value.(*entry[K, V])
	delete(c.entries, ent.key)
	c.order.Remove(elem)
}

func (c *Cache[K, V]) isExpired(ent *entry[K, V], now time.Time) bool {
	if ent.expiresAt.IsZero() {
		return false
	}
	return !ent.expiresAt.After(now)
}

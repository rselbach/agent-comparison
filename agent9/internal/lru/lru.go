package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache is an LRU cache with per-entry ttl expiration and background janitor.
// Zero value is not ready; use New to construct.
// All exported methods are safe for concurrent use.
type Cache[K comparable, V any] struct {
	cap     int
	mu      sync.RWMutex
	items   map[K]*list.Element
	list    *list.List // front = most recent
	janitor *janitor
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	ttl       time.Duration
}

// Option configures cache creation.
type Option[K comparable, V any] func(*Cache[K, V])

// WithCapacity overrides default capacity.
func WithCapacity[K comparable, V any](c int) Option[K, V] {
	return func(cache *Cache[K, V]) {
		cache.cap = c
	}
}

// WithJanitorInterval sets the interval for background expiration scan.
func WithJanitorInterval[K comparable, V any](d time.Duration) Option[K, V] {
	return func(cache *Cache[K, V]) {
		if cache.janitor != nil {
			cache.janitor.interval = d
		}
	}
}

// New constructs a cache with given capacity and options. Capacity must be > 0.
func New[K comparable, V any](capacity int, opts ...Option[K, V]) *Cache[K, V] {
	if capacity <= 0 {
		panic("capacity must be > 0")
	}
	c := &Cache[K, V]{
		cap:   capacity,
		items: make(map[K]*list.Element, capacity),
		list:  list.New(),
	}
	c.janitor = &janitor{interval: time.Second * 30, stop: make(chan struct{})}
	for _, o := range opts {
		o(c)
	}
	c.startJanitor()
	return c
}

// Set inserts or updates a value with ttl. ttl <= 0 means no expiration.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		ent := el.Value.(*entry[K, V])
		ent.value = value
		ent.ttl = ttl
		ent.expiresAt = exp
		c.list.MoveToFront(el)
		return
	}
	if c.list.Len() >= c.cap {
		c.removeOldestLocked()
	}
	el := c.list.PushFront(&entry[K, V]{key: key, value: value, ttl: ttl, expiresAt: exp})
	c.items[key] = el
}

// Get returns value and a bool indicating presence. Expired items are evicted and reported absent.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	ent := el.Value.(*entry[K, V])
	if ent.ttl > 0 && time.Now().After(ent.expiresAt) {
		c.removeElementLocked(el)
		var zero V
		return zero, false
	}
	c.list.MoveToFront(el)
	return ent.value, true
}

// Peek returns value without updating recency. Expired items are evicted.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	ent := el.Value.(*entry[K, V])
	if ent.ttl > 0 && time.Now().After(ent.expiresAt) {
		c.removeElementLocked(el)
		var zero V
		return zero, false
	}
	return ent.value, true
}

// Delete removes a key if present.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElementLocked(el)
	return true
}

// Len returns current number of items.
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list.Len()
}

// Capacity returns configured capacity.
func (c *Cache[K, V]) Capacity() int { return c.cap }

// Close stops background janitor. Safe to call multiple times.
func (c *Cache[K, V]) Close() {
	c.mu.Lock()
	j := c.janitor
	c.mu.Unlock()
	if j == nil {
		return
	}
	select {
	case <-j.stop:
		return
	default:
		close(j.stop)
	}
}

func (c *Cache[K, V]) removeOldestLocked() {
	el := c.list.Back()
	if el == nil {
		return
	}
	c.removeElementLocked(el)
}

func (c *Cache[K, V]) removeElementLocked(el *list.Element) {
	ent := el.Value.(*entry[K, V])
	delete(c.items, ent.key)
	c.list.Remove(el)
}

type janitor struct {
	interval time.Duration
	stop     chan struct{}
}

func (c *Cache[K, V]) startJanitor() {
	j := c.janitor
	if j == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.expireScan()
			case <-j.stop:
				return
			}
		}
	}()
}

// expireScan removes expired entries. holds lock briefly per check.
func (c *Cache[K, V]) expireScan() {
	now := time.Now()
	c.mu.Lock()
	for el := c.list.Back(); el != nil; {
		prev := el.Prev()
		ent := el.Value.(*entry[K, V])
		if ent.ttl > 0 && now.After(ent.expiresAt) {
			c.removeElementLocked(el)
		}
		el = prev
	}
	c.mu.Unlock()
}

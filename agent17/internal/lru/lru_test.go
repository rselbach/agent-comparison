package lru

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCache_BasicOperations(t *testing.T) {
	tests := map[string]struct {
		setup   func(*Cache[string, int])
		key     string
		wantVal int
		wantOK  bool
		wantLen int
	}{
		"get existing": {
			setup: func(c *Cache[string, int]) {
				c.Set("a", 1)
			},
			key:     "a",
			wantVal: 1,
			wantOK:  true,
			wantLen: 1,
		},
		"get missing": {
			setup:   func(c *Cache[string, int]) {},
			key:     "missing",
			wantVal: 0,
			wantOK:  false,
			wantLen: 0,
		},
		"overwrite": {
			setup: func(c *Cache[string, int]) {
				c.Set("a", 1)
				c.Set("a", 2)
			},
			key:     "a",
			wantVal: 2,
			wantOK:  true,
			wantLen: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			c := New[string, int](10)
			tc.setup(c)

			val, ok := c.Get(tc.key)
			r.Equal(tc.wantOK, ok)
			r.Equal(tc.wantVal, val)
			r.Equal(tc.wantLen, c.Len())
		})
	}
}

func TestCache_LRUEviction(t *testing.T) {
	r := require.New(t)
	c := New[string, int](3)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// access "a" to make it most recent
	_, _ = c.Get("a")

	// add "d", should evict "b" (least recent)
	c.Set("d", 4)

	_, ok := c.Get("b")
	r.False(ok, "b should be evicted")

	_, ok = c.Get("a")
	r.True(ok, "a should still exist")

	_, ok = c.Get("c")
	r.True(ok, "c should still exist")

	_, ok = c.Get("d")
	r.True(ok, "d should exist")
}

func TestCache_Expiration(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	c := New[string, int](10, WithTTL[string, int](time.Minute))
	c.now = func() time.Time { return now }

	c.Set("a", 1)

	// still valid
	val, ok := c.Get("a")
	r.True(ok)
	r.Equal(1, val)

	// advance time past TTL
	c.now = func() time.Time { return now.Add(2 * time.Minute) }

	_, ok = c.Get("a")
	r.False(ok, "should be expired")

	// entry should be removed
	r.Equal(0, c.Len())
}

func TestCache_SetWithTTL(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	c := New[string, int](10, WithTTL[string, int](time.Hour))
	c.now = func() time.Time { return now }

	// set with shorter TTL
	c.SetWithTTL("short", 1, time.Second)
	// set with default TTL
	c.Set("long", 2)

	// advance past short TTL but before default
	c.now = func() time.Time { return now.Add(time.Minute) }

	_, ok := c.Get("short")
	r.False(ok, "short TTL should be expired")

	val, ok := c.Get("long")
	r.True(ok, "long TTL should still be valid")
	r.Equal(2, val)
}

func TestCache_ZeroTTL(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	c := New[string, int](10) // no default TTL
	c.now = func() time.Time { return now }

	c.Set("forever", 1)

	// advance time significantly
	c.now = func() time.Time { return now.Add(24 * 365 * time.Hour) }

	val, ok := c.Get("forever")
	r.True(ok, "should never expire with zero TTL")
	r.Equal(1, val)
}

func TestCache_Delete(t *testing.T) {
	r := require.New(t)
	c := New[string, int](10)

	c.Set("a", 1)
	r.Equal(1, c.Len())

	deleted := c.Delete("a")
	r.True(deleted)
	r.Equal(0, c.Len())

	deleted = c.Delete("nonexistent")
	r.False(deleted)
}

func TestCache_Peek(t *testing.T) {
	r := require.New(t)
	c := New[string, int](2)

	c.Set("a", 1)
	c.Set("b", 2)

	// peek at "a" without updating order
	val, ok := c.Peek("a")
	r.True(ok)
	r.Equal(1, val)

	// add "c", should evict "a" since peek doesn't update order
	c.Set("c", 3)

	_, ok = c.Get("a")
	r.False(ok, "a should be evicted")
}

func TestCache_Clear(t *testing.T) {
	r := require.New(t)

	var evicted []string
	c := New[string, int](10, WithOnEvict[string, int](func(k string, _ int) {
		evicted = append(evicted, k)
	}))

	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	r.Equal(0, c.Len())
	r.ElementsMatch([]string{"a", "b"}, evicted)
}

func TestCache_Keys(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	c := New[string, int](10, WithTTL[string, int](time.Minute))
	c.now = func() time.Time { return now }

	c.Set("a", 1)
	c.Set("b", 2)
	c.SetWithTTL("c", 3, time.Second) // short TTL

	// advance past short TTL
	c.now = func() time.Time { return now.Add(30 * time.Second) }

	keys := c.Keys()
	r.ElementsMatch([]string{"a", "b"}, keys)
}

func TestCache_Purge(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	c := New[string, int](10)
	c.now = func() time.Time { return now }

	c.SetWithTTL("short1", 1, time.Second)
	c.SetWithTTL("short2", 2, time.Second)
	c.SetWithTTL("long", 3, time.Hour)

	// advance past short TTL
	c.now = func() time.Time { return now.Add(time.Minute) }

	purged := c.Purge()
	r.Equal(2, purged)
	r.Equal(1, c.Len())

	val, ok := c.Get("long")
	r.True(ok)
	r.Equal(3, val)
}

func TestCache_OnEvict(t *testing.T) {
	r := require.New(t)

	var evictedKey string
	var evictedVal int
	c := New[string, int](2, WithOnEvict[string, int](func(k string, v int) {
		evictedKey = k
		evictedVal = v
	}))

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // should evict "a"

	r.Equal("a", evictedKey)
	r.Equal(1, evictedVal)
}

func TestCache_Concurrent(t *testing.T) {
	r := require.New(t)
	c := New[int, int](100)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set(n, n*10)
			c.Get(n)
			c.Delete(n % 50)
		}(i)
	}
	wg.Wait()

	// just verify no panics and cache is usable
	c.Set(999, 999)
	val, ok := c.Get(999)
	r.True(ok)
	r.Equal(999, val)
}

func TestCache_MinCapacity(t *testing.T) {
	r := require.New(t)

	c := New[string, int](0) // should be clamped to 1
	c.Set("a", 1)
	c.Set("b", 2)

	r.Equal(1, c.Len())

	_, ok := c.Get("a")
	r.False(ok, "a should be evicted")

	val, ok := c.Get("b")
	r.True(ok)
	r.Equal(2, val)
}

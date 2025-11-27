package lru

import (
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
		"get existing key": {
			setup: func(c *Cache[string, int]) {
				c.Set("a", 1)
			},
			key:     "a",
			wantVal: 1,
			wantOK:  true,
			wantLen: 1,
		},
		"get missing key": {
			setup:   func(c *Cache[string, int]) {},
			key:     "missing",
			wantVal: 0,
			wantOK:  false,
			wantLen: 0,
		},
		"update existing key": {
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
			c := New[string, int](10, 0)
			tc.setup(c)

			val, ok := c.Get(tc.key)
			r.Equal(tc.wantOK, ok)
			r.Equal(tc.wantVal, val)
			r.Equal(tc.wantLen, c.Len())
		})
	}
}

func TestCache_Eviction(t *testing.T) {
	r := require.New(t)
	c := New[string, int](3, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	r.Equal(3, c.Len())

	// adding d should evict a (oldest)
	c.Set("d", 4)
	r.Equal(3, c.Len())

	_, ok := c.Get("a")
	r.False(ok, "a should have been evicted")

	_, ok = c.Get("b")
	r.True(ok, "b should still exist")
}

func TestCache_LRUOrder(t *testing.T) {
	r := require.New(t)
	c := New[string, int](3, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// access a to make it most recently used
	c.Get("a")

	// adding d should evict b (now oldest)
	c.Set("d", 4)

	_, ok := c.Get("b")
	r.False(ok, "b should have been evicted")

	_, ok = c.Get("a")
	r.True(ok, "a should still exist")
}

func TestCache_Expiration(t *testing.T) {
	r := require.New(t)
	c := New[string, int](10, 100*time.Millisecond)

	// mock time
	now := time.Now()
	c.now = func() time.Time { return now }

	c.Set("a", 1)

	// should exist immediately
	val, ok := c.Get("a")
	r.True(ok)
	r.Equal(1, val)

	// advance time past expiration
	now = now.Add(150 * time.Millisecond)

	// should be expired
	_, ok = c.Get("a")
	r.False(ok, "a should have expired")
	r.Equal(0, c.Len(), "expired item should be removed")
}

func TestCache_Delete(t *testing.T) {
	r := require.New(t)
	c := New[string, int](10, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	r.Equal(2, c.Len())

	c.Delete("a")
	r.Equal(1, c.Len())

	_, ok := c.Get("a")
	r.False(ok)

	// deleting non-existent key should not panic
	c.Delete("nonexistent")
}

func TestCache_Clear(t *testing.T) {
	r := require.New(t)
	c := New[string, int](10, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	r.Equal(3, c.Len())

	c.Clear()
	r.Equal(0, c.Len())

	_, ok := c.Get("a")
	r.False(ok)
}

func TestCache_Cleanup(t *testing.T) {
	r := require.New(t)
	c := New[string, int](10, 100*time.Millisecond)

	now := time.Now()
	c.now = func() time.Time { return now }

	c.Set("a", 1)
	c.Set("b", 2)

	// advance time and add another
	now = now.Add(50 * time.Millisecond)
	c.Set("c", 3)

	// advance past first two items' expiration
	now = now.Add(60 * time.Millisecond)

	r.Equal(3, c.Len(), "items not yet cleaned up")

	c.Cleanup()
	r.Equal(1, c.Len(), "expired items should be cleaned")

	_, ok := c.Get("c")
	r.True(ok, "c should still exist")
}

func TestCache_ZeroCapacity(t *testing.T) {
	r := require.New(t)
	// zero capacity should default to 1
	c := New[string, int](0, 0)

	c.Set("a", 1)
	c.Set("b", 2)

	r.Equal(1, c.Len())
	_, ok := c.Get("b")
	r.True(ok)
}

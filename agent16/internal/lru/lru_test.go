package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLRU_Basic(t *testing.T) {
	r := require.New(t)

	// Capacity 2, No TTL
	c := New[string, int](2, 0)

	c.Set("a", 1)
	c.Set("b", 2)

	val, ok := c.Get("a")
	r.True(ok)
	r.Equal(1, val)

	// Add 'c', should evict 'b' because 'a' was just accessed?
	// Wait, 'a' accessed -> 'a' is front. 'b' is back.
	// c.Set("c", 3) -> evicts 'b'.
	c.Set("c", 3)

	_, ok = c.Get("b")
	r.False(ok, "b should be evicted")

	val, ok = c.Get("c")
	r.True(ok)
	r.Equal(3, val)

	val, ok = c.Get("a")
	r.True(ok)
	r.Equal(1, val)
}

func TestLRU_Expiration(t *testing.T) {
	r := require.New(t)

	ttl := 100 * time.Millisecond
	c := New[string, string](10, ttl)

	c.Set("foo", "bar")

	val, ok := c.Get("foo")
	r.True(ok)
	r.Equal("bar", val)

	time.Sleep(200 * time.Millisecond)

	_, ok = c.Get("foo")
	r.False(ok, "foo should have expired")
	r.Equal(0, c.Len(), "cache should be empty after expiration check")
}

func TestLRU_UpdateResetsTTL(t *testing.T) {
	r := require.New(t)

	ttl := 200 * time.Millisecond
	c := New[string, int](10, ttl)

	c.Set("a", 1)
	time.Sleep(100 * time.Millisecond)

	// Update 'a'
	c.Set("a", 2)

	time.Sleep(150 * time.Millisecond)
	// Total time 250ms, but reset at 100ms.
	// 100ms passed since set. Should be alive.

	val, ok := c.Get("a")
	r.True(ok, "a should still be alive after update")
	r.Equal(2, val)

	time.Sleep(100 * time.Millisecond)
	_, ok = c.Get("a")
	r.False(ok, "a should have expired")
}

func TestLRU_Capacity(t *testing.T) {
	tests := map[string]struct {
		cap  int
		keys []int
		want []int // keys expected to remain
	}{
		"capacity 1": {
			cap:  1,
			keys: []int{1, 2, 3},
			want: []int{3},
		},
		"capacity 3": {
			cap:  3,
			keys: []int{1, 2, 3, 4},
			want: []int{2, 3, 4},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			c := New[int, int](tc.cap, 0)

			for _, k := range tc.keys {
				c.Set(k, k)
			}

			r.Equal(len(tc.want), c.Len())
			for _, k := range tc.want {
				val, ok := c.Get(k)
				r.True(ok, "key %d should exist", k)
				r.Equal(k, val)
			}
		})
	}
}

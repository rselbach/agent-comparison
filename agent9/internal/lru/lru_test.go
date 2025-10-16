package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheBasic(t *testing.T) {
	r := require.New(t)
	c := New[string, int](2)
	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	{ // initial fetch
		v, ok := c.Get("a")
		r.True(ok)
		r.Equal(1, v)
	}
	c.Set("c", 3, 0) // evicts oldest (b)
	_, okB := c.Get("b")
	r.False(okB)
	vC, okC := c.Get("c")
	r.True(okC)
	r.Equal(3, vC)
	c.Close()
}

func TestCacheRecency(t *testing.T) {
	r := require.New(t)
	c := New[string, int](2)
	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	// access a so b becomes oldest
	_, _ = c.Get("a")
	c.Set("c", 3, 0) // should evict b
	_, okB := c.Get("b")
	r.False(okB)
	_, okA := c.Get("a")
	r.True(okA)
	c.Close()
}

func TestExpiration(t *testing.T) {
	r := require.New(t)
	c := New[string, int](3, WithJanitorInterval[string, int](10*time.Millisecond))
	c.Set("short", 1, 20*time.Millisecond)
	c.Set("long", 2, 200*time.Millisecond)
	c.Set("none", 3, 0)
	// immediately all present
	_, ok := c.Get("short")
	r.True(ok)
	_, ok = c.Get("long")
	r.True(ok)
	_, ok = c.Get("none")
	r.True(ok)
	time.Sleep(60 * time.Millisecond) // allow short to expire and janitor run
	_, ok = c.Get("short")
	r.False(ok)
	_, ok = c.Get("long")
	r.True(ok)
	_, ok = c.Get("none")
	r.True(ok)
	c.Close()
}

func TestManualExpireOnGet(t *testing.T) {
	r := require.New(t)
	c := New[string, int](2)
	c.Set("x", 9, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	_, ok := c.Get("x")
	r.False(ok)
	c.Close()
}

func TestUpdateResetsTTL(t *testing.T) {
	r := require.New(t)
	c := New[string, int](1)
	c.Set("a", 1, 10*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	c.Set("a", 2, 20*time.Millisecond)
	time.Sleep(12 * time.Millisecond)
	v, ok := c.Get("a")
	r.True(ok)
	r.Equal(2, v)
	c.Close()
}

func TestDelete(t *testing.T) {
	r := require.New(t)
	c := New[string, int](1)
	c.Set("a", 1, 0)
	r.True(c.Delete("a"))
	r.False(c.Delete("a"))
	_, ok := c.Get("a")
	r.False(ok)
	c.Close()
}

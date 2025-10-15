package lrucache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	r := require.New(t)

	t.Run("valid capacity", func(t *testing.T) {
		c := New(10)
		r.NotNil(c)
		r.Equal(10, c.capacity)
		r.Equal(0, c.Len())
	})

	t.Run("zero capacity defaults to 1", func(t *testing.T) {
		c := New(0)
		r.NotNil(c)
		r.Equal(1, c.capacity)
	})

	t.Run("negative capacity defaults to 1", func(t *testing.T) {
		c := New(-5)
		r.NotNil(c)
		r.Equal(1, c.capacity)
	})
}

func TestSetAndGet(t *testing.T) {
	r := require.New(t)
	c := New(3)

	tests := map[string]struct {
		key    string
		value  any
		ttl    time.Duration
		want   any
		wantOk bool
		setup  func() // optional setup before get
	}{
		"simple set and get": {
			key:    "key1",
			value:  "value1",
			ttl:    time.Minute,
			want:   "value1",
			wantOk: true,
		},
		"zero ttl immediately expires": {
			key:    "key2",
			value:  "value2",
			ttl:    0,
			wantOk: false,
		},
		"negative ttl immediately expires": {
			key:    "key3",
			value:  "value3",
			ttl:    -time.Minute,
			wantOk: false,
		},
		"update existing key": {
			key:    "key1",
			value:  "updated_value",
			ttl:    time.Minute,
			want:   "updated_value",
			wantOk: true,
			setup: func() {
				c.Set("key1", "original", time.Minute)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			c.Set(tc.key, tc.value, tc.ttl)
			got, ok := c.Get(tc.key)

			r.Equal(tc.wantOk, ok)
			if tc.wantOk {
				r.Equal(tc.want, got)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	r := require.New(t)
	c := New(3)

	t.Run("delete existing key", func(t *testing.T) {
		c.Set("key1", "value1", time.Minute)
		r.Equal(1, c.Len())

		deleted := c.Delete("key1")
		r.True(deleted)
		r.Equal(0, c.Len())

		_, ok := c.Get("key1")
		r.False(ok)
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		deleted := c.Delete("nonexistent")
		r.False(deleted)
		r.Equal(0, c.Len())
	})
}

func TestClear(t *testing.T) {
	r := require.New(t)
	c := New(3)

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)
	r.Equal(2, c.Len())

	c.Clear()
	r.Equal(0, c.Len())

	_, ok := c.Get("key1")
	r.False(ok)
	_, ok = c.Get("key2")
	r.False(ok)
}

func TestEviction(t *testing.T) {
	r := require.New(t)
	c := New(2) // capacity of 2

	t.Run("evicts least recently used", func(t *testing.T) {
		c.Set("key1", "value1", time.Minute)
		c.Set("key2", "value2", time.Minute)
		r.Equal(2, c.Len())

		// access key1 to make it most recently used
		c.Get("key1")

		// add key3, should evict key2 (least recently used)
		c.Set("key3", "value3", time.Minute)
		r.Equal(2, c.Len())

		// key1 should still exist (was accessed)
		_, ok := c.Get("key1")
		r.True(ok)

		// key2 should be evicted
		_, ok = c.Get("key2")
		r.False(ok)

		// key3 should exist
		_, ok = c.Get("key3")
		r.True(ok)
	})
}

func TestExpiration(t *testing.T) {
	r := require.New(t)
	c := New(5)

	t.Run("items expire after ttl", func(t *testing.T) {
		c.Set("key1", "value1", 10*time.Millisecond)

		// should exist immediately
		_, ok := c.Get("key1")
		r.True(ok)

		// wait for expiration
		time.Sleep(20 * time.Millisecond)

		// should be expired
		_, ok = c.Get("key1")
		r.False(ok)
	})

	t.Run("cleanup removes expired items", func(t *testing.T) {
		c.Set("key2", "value2", 10*time.Millisecond)
		c.Set("key3", "value3", time.Hour) // won't expire

		r.Equal(2, c.Len())

		// wait for key2 to expire
		time.Sleep(20 * time.Millisecond)

		// manually trigger cleanup by checking key3 (this doesn't trigger cleanup)
		// the background cleanup runs every minute, so we'll trigger it manually
		c.removeExpired()

		// only key3 should remain
		r.Equal(1, c.Len())

		_, ok := c.Get("key2")
		r.False(ok)

		_, ok = c.Get("key3")
		r.True(ok)
	})
}

func TestConcurrentAccess(t *testing.T) {
	r := require.New(t)
	c := New(100)

	done := make(chan bool, 2)

	// goroutine 1: write values
	go func() {
		for i := 0; i < 50; i++ {
			c.Set("key"+string(rune(i)), "value"+string(rune(i)), time.Minute)
		}
		done <- true
	}()

	// goroutine 2: read values
	go func() {
		for i := 0; i < 50; i++ {
			c.Get("key" + string(rune(i)))
		}
		done <- true
	}()

	// wait for both goroutines
	<-done
	<-done

	// cache should still be functional
	c.Set("final", "final_value", time.Minute)
	val, ok := c.Get("final")
	r.True(ok)
	r.Equal("final_value", val)
}

func TestClose(t *testing.T) {
	r := require.New(t)
	c := New(5)

	c.Set("key1", "value1", time.Minute)
	r.Equal(1, c.Len())

	c.Close()
	r.Equal(0, c.Len())
}

func TestEdgeCases(t *testing.T) {
	r := require.New(t)
	c := New(1)

	t.Run("nil values", func(t *testing.T) {
		c.Set("nil_key", nil, time.Minute)
		got, ok := c.Get("nil_key")
		r.True(ok)
		r.Nil(got)
	})

	t.Run("empty strings as keys", func(t *testing.T) {
		c.Set("", "empty_key_value", time.Minute)
		got, ok := c.Get("")
		r.True(ok)
		r.Equal("empty_key_value", got)
	})
}

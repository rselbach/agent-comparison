package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := map[string]struct {
		maxSize         int
		cleanupInterval time.Duration
		wantPanic       bool
	}{
		"valid cache": {
			maxSize:         10,
			cleanupInterval: time.Second,
			wantPanic:       false,
		},
		"zero cleanup interval uses default": {
			maxSize:         5,
			cleanupInterval: 0,
			wantPanic:       false,
		},
		"zero maxSize panics": {
			maxSize:         0,
			cleanupInterval: time.Second,
			wantPanic:       true,
		},
		"negative maxSize panics": {
			maxSize:         -1,
			cleanupInterval: time.Second,
			wantPanic:       true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)

			if tc.wantPanic {
				r.Panics(func() {
					New(tc.maxSize, tc.cleanupInterval)
				})
				return
			}

			cache := New(tc.maxSize, tc.cleanupInterval)
			r.NotNil(cache)
			r.Equal(0, cache.Len())
			cache.Close()
		})
	}
}

func TestCache_SetAndGet(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	// set some values
	cache.Set("key1", "value1", 0)
	cache.Set("key2", 42, 0)
	cache.Set("key3", []string{"a", "b"}, 0)

	// verify values
	val, ok := cache.Get("key1")
	r.True(ok)
	r.Equal("value1", val)

	val, ok = cache.Get("key2")
	r.True(ok)
	r.Equal(42, val)

	val, ok = cache.Get("key3")
	r.True(ok)
	r.Equal([]string{"a", "b"}, val)

	// get non-existent key
	val, ok = cache.Get("nonexistent")
	r.False(ok)
	r.Nil(val)
}

func TestCache_Update(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key1", "updated", 0)

	val, ok := cache.Get("key1")
	r.True(ok)
	r.Equal("updated", val)
	r.Equal(1, cache.Len())
}

func TestCache_Eviction(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	// fill the cache
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)
	r.Equal(3, cache.Len())

	// access key1 to make it most recently used
	_, ok := cache.Get("key1")
	r.True(ok)

	// add a fourth item, should evict key2 (least recently used)
	cache.Set("key4", "value4", 0)
	r.Equal(3, cache.Len())

	// key2 should be evicted
	_, ok = cache.Get("key2")
	r.False(ok)

	// other keys should still exist
	_, ok = cache.Get("key1")
	r.True(ok)
	_, ok = cache.Get("key3")
	r.True(ok)
	_, ok = cache.Get("key4")
	r.True(ok)
}

func TestCache_Delete(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	r.Equal(2, cache.Len())

	cache.Delete("key1")
	r.Equal(1, cache.Len())

	_, ok := cache.Get("key1")
	r.False(ok)

	_, ok = cache.Get("key2")
	r.True(ok)

	// delete non-existent key should not panic
	cache.Delete("nonexistent")
	r.Equal(1, cache.Len())
}

func TestCache_Clear(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)
	r.Equal(3, cache.Len())

	cache.Clear()
	r.Equal(0, cache.Len())

	_, ok := cache.Get("key1")
	r.False(ok)
}

func TestCache_Expiration(t *testing.T) {
	r := require.New(t)
	cache := New(10, time.Minute)
	defer cache.Close()

	// set item with short TTL
	cache.Set("key1", "value1", 100*time.Millisecond)
	cache.Set("key2", "value2", 0) // no expiration

	// immediately get should work
	val, ok := cache.Get("key1")
	r.True(ok)
	r.Equal("value1", val)

	// wait for expiration
	time.Sleep(150 * time.Millisecond)

	// expired item should return false
	_, ok = cache.Get("key1")
	r.False(ok)

	// non-expired item should still work
	val, ok = cache.Get("key2")
	r.True(ok)
	r.Equal("value2", val)
}

func TestCache_AutomaticCleanup(t *testing.T) {
	r := require.New(t)
	// use short cleanup interval for testing
	cache := New(10, 100*time.Millisecond)
	defer cache.Close()

	// add items with short TTL
	cache.Set("key1", "value1", 50*time.Millisecond)
	cache.Set("key2", "value2", 50*time.Millisecond)
	cache.Set("key3", "value3", 0) // no expiration

	r.Equal(3, cache.Len())

	// wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// expired items should be cleaned up
	r.Equal(1, cache.Len())

	_, ok := cache.Get("key3")
	r.True(ok)
}

func TestCache_Concurrency(t *testing.T) {
	r := require.New(t)
	cache := New(100, time.Minute)
	defer cache.Close()

	// run concurrent operations
	done := make(chan bool)

	// writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("key", i, 0)
		}
		done <- true
	}()

	// reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("key")
		}
		done <- true
	}()

	// deleter goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Delete("key")
		}
		done <- true
	}()

	// wait for all goroutines
	<-done
	<-done
	<-done

	// cache should still be usable
	cache.Set("final", "value", 0)
	val, ok := cache.Get("final")
	r.True(ok)
	r.Equal("value", val)
}

func TestCache_LRUOrdering(t *testing.T) {
	r := require.New(t)
	cache := New(3, time.Minute)
	defer cache.Close()

	// add three items
	cache.Set("a", 1, 0)
	cache.Set("b", 2, 0)
	cache.Set("c", 3, 0)

	// access "a" to make it more recently used than "b"
	cache.Get("a")

	// add "d", should evict "b" (oldest)
	cache.Set("d", 4, 0)

	_, ok := cache.Get("b")
	r.False(ok)

	_, ok = cache.Get("a")
	r.True(ok)
	_, ok = cache.Get("c")
	r.True(ok)
	_, ok = cache.Get("d")
	r.True(ok)
}

func TestCache_Close(t *testing.T) {
	r := require.New(t)
	cache := New(10, time.Millisecond)

	cache.Set("key1", "value1", 0)

	// close should stop the cleanup goroutine
	cache.Close()

	// cache should still be readable after close
	val, ok := cache.Get("key1")
	r.True(ok)
	r.Equal("value1", val)
}

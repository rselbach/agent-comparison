package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLRU_Get(t *testing.T) {
	r := require.New(t)
	lru := New(2)

	// Test getting non-existent key
	_, ok := lru.Get("nonexistent")
	r.False(ok)

	// Put a value and get it
	lru.Put("key1", "value1", time.Minute)
	val, ok := lru.Get("key1")
	r.True(ok)
	r.Equal("value1", val)
}

func TestLRU_Put(t *testing.T) {
	r := require.New(t)
	lru := New(2)

	// Put values
	lru.Put("key1", "value1", time.Minute)
	lru.Put("key2", "value2", time.Minute)

	// Check capacity
	r.Equal(2, lru.l.Len())

	// Put third value, should evict oldest
	lru.Put("key3", "value3", time.Minute)
	r.Equal(2, lru.l.Len())

	// key1 should be evicted
	_, ok := lru.Get("key1")
	r.False(ok)

	// key2 and key3 should be present
	val, ok := lru.Get("key2")
	r.True(ok)
	r.Equal("value2", val)

	val, ok = lru.Get("key3")
	r.True(ok)
	r.Equal("value3", val)
}

func TestLRU_Expiration(t *testing.T) {
	r := require.New(t)
	lru := New(2)

	// Put with short TTL
	lru.Put("key1", "value1", time.Millisecond*10)

	// Immediately get, should be ok
	val, ok := lru.Get("key1")
	r.True(ok)
	r.Equal("value1", val)

	// Wait for expiration
	time.Sleep(time.Millisecond * 20)

	// Now should be expired
	_, ok = lru.Get("key1")
	r.False(ok)
}

func TestLRU_Update(t *testing.T) {
	r := require.New(t)
	lru := New(2)

	// Put initial value
	lru.Put("key1", "value1", time.Minute)

	// Update value
	lru.Put("key1", "value2", time.Minute)

	// Get updated value
	val, ok := lru.Get("key1")
	r.True(ok)
	r.Equal("value2", val)
}

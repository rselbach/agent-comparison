package agent8

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLRU_SetAndGet(t *testing.T) {
	tests := map[string]struct {
		capacity int
		ttl      time.Duration
		setup    func(*LRU)
		key      string
		want     interface{}
		wantOk   bool
	}{
		"get existing key": {
			capacity: 3,
			ttl:      0,
			setup: func(lru *LRU) {
				lru.Set("key1", "value1")
			},
			key:    "key1",
			want:   "value1",
			wantOk: true,
		},
		"get non-existing key": {
			capacity: 3,
			ttl:      0,
			setup:    func(lru *LRU) {},
			key:      "missing",
			want:     nil,
			wantOk:   false,
		},
		"update existing key": {
			capacity: 3,
			ttl:      0,
			setup: func(lru *LRU) {
				lru.Set("key1", "value1")
				lru.Set("key1", "value2")
			},
			key:    "key1",
			want:   "value2",
			wantOk: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			lru := NewLRU(tc.capacity, tc.ttl)
			defer lru.Close()

			tc.setup(lru)

			got, ok := lru.Get(tc.key)
			r.Equal(tc.wantOk, ok)
			r.Equal(tc.want, got)
		})
	}
}

func TestLRU_Eviction(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(3, 0)
	defer lru.Close()

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")
	lru.Set("key3", "value3")
	lru.Set("key4", "value4")

	r.Equal(3, lru.Len())

	_, ok := lru.Get("key1")
	r.False(ok)

	_, ok = lru.Get("key4")
	r.True(ok)
}

func TestLRU_LRUOrder(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(3, 0)
	defer lru.Close()

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")
	lru.Set("key3", "value3")

	lru.Get("key1")

	lru.Set("key4", "value4")

	_, ok := lru.Get("key2")
	r.False(ok)

	_, ok = lru.Get("key1")
	r.True(ok)
}

func TestLRU_Expiration(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(3, 100*time.Millisecond)
	defer lru.Close()

	lru.Set("key1", "value1")

	_, ok := lru.Get("key1")
	r.True(ok)

	time.Sleep(150 * time.Millisecond)

	_, ok = lru.Get("key1")
	r.False(ok)
}

func TestLRU_CleanupExpired(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(5, 100*time.Millisecond)
	defer lru.Close()

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")
	lru.Set("key3", "value3")

	r.Equal(3, lru.Len())

	time.Sleep(150 * time.Millisecond)

	r.Eventually(func() bool {
		return lru.Len() == 0
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func TestLRU_Delete(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(3, 0)
	defer lru.Close()

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")

	r.Equal(2, lru.Len())

	lru.Delete("key1")

	r.Equal(1, lru.Len())

	_, ok := lru.Get("key1")
	r.False(ok)

	_, ok = lru.Get("key2")
	r.True(ok)
}

func TestLRU_Clear(t *testing.T) {
	r := require.New(t)
	lru := NewLRU(3, 0)
	defer lru.Close()

	lru.Set("key1", "value1")
	lru.Set("key2", "value2")
	lru.Set("key3", "value3")

	r.Equal(3, lru.Len())

	lru.Clear()

	r.Equal(0, lru.Len())
}

func TestLRU_PanicOnInvalidCapacity(t *testing.T) {
	r := require.New(t)
	r.Panics(func() {
		NewLRU(0, 0)
	})
}

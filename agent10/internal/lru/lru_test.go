package lru_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"agent10/internal/lru"
)

func TestCacheSetGet(t *testing.T) {
	tests := map[string]struct {
		ttl    time.Duration
		wait   time.Duration
		wantOK bool
	}{
		"without ttl": {
			ttl:    0,
			wait:   0,
			wantOK: true,
		},
		"with unexpired ttl": {
			ttl:    200 * time.Millisecond,
			wait:   50 * time.Millisecond,
			wantOK: true,
		},
		"with expired ttl": {
			ttl:    30 * time.Millisecond,
			wait:   70 * time.Millisecond,
			wantOK: false,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)

			cache := lru.New[string, int](2)
			cache.Set("alpha", 42, tc.ttl)

			if tc.wait > 0 {
				time.Sleep(tc.wait)
			}

			value, ok := cache.Get("alpha")
			r.Equal(tc.wantOK, ok)
			if tc.wantOK {
				r.Equal(42, value)
				r.Equal(1, cache.Len())
			} else {
				r.Zero(value)
				r.Equal(0, cache.Len())
			}
		})
	}
}

func TestCacheEviction(t *testing.T) {
	tests := map[string]struct {
		capacity int
		setup    func(*lru.Cache[string, int])
		verify   func(*require.Assertions, *lru.Cache[string, int])
	}{
		"evicts least recently used": {
			capacity: 2,
			setup: func(cache *lru.Cache[string, int]) {
				cache.Set("a", 1, 0)
				cache.Set("b", 2, 0)
				_, _ = cache.Get("a")
				cache.Set("c", 3, 0)
			},
			verify: func(r *require.Assertions, cache *lru.Cache[string, int]) {
				_, okB := cache.Get("b")
				r.False(okB)

				valueA, okA := cache.Get("a")
				r.True(okA)
				r.Equal(1, valueA)

				valueC, okC := cache.Get("c")
				r.True(okC)
				r.Equal(3, valueC)

				r.Equal(2, cache.Len())
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)

			cache := lru.New[string, int](tc.capacity)
			tc.setup(cache)
			tc.verify(r, cache)
		})
	}
}

func TestCacheExpirationRefresh(t *testing.T) {
	tests := map[string]struct {
		initialTTL time.Duration
		refreshTTL time.Duration
		waits      []time.Duration
		expects    []bool
	}{
		"refresh extends lifetime": {
			initialTTL: 40 * time.Millisecond,
			refreshTTL: 80 * time.Millisecond,
			waits:      []time.Duration{20 * time.Millisecond, 50 * time.Millisecond, 40 * time.Millisecond},
			expects:    []bool{true, true, false},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)

			cache := lru.New[string, int](1)
			cache.Set("token", 7, tc.initialTTL)

			time.Sleep(tc.waits[0])
			value, ok := cache.Get("token")
			r.Equal(tc.expects[0], ok)
			if ok {
				r.Equal(7, value)
			}

			cache.Set("token", 7, tc.refreshTTL)

			time.Sleep(tc.waits[1])
			value, ok = cache.Get("token")
			r.Equal(tc.expects[1], ok)
			if ok {
				r.Equal(7, value)
			}

			time.Sleep(tc.waits[2])
			_, ok = cache.Get("token")
			r.Equal(tc.expects[2], ok)
		})
	}
}
func TestCacheDelete(t *testing.T) {
	tests := map[string]struct {
		operations func(*require.Assertions, *lru.Cache[string, int])
	}{
		"delete removes entries": {
			operations: func(r *require.Assertions, cache *lru.Cache[string, int]) {
				cache.Set("key", 5, 0)
				removed := cache.Delete("key")
				r.True(removed)

				_, ok := cache.Get("key")
				r.False(ok)
				r.Equal(0, cache.Len())

				removedAgain := cache.Delete("key")
				r.False(removedAgain)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)

			cache := lru.New[string, int](2)
			tc.operations(r, cache)
		})
	}
}

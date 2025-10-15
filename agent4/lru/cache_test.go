package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewValidation(t *testing.T) {
	tests := map[string]struct {
		capacity int
		options  []Option
		wantErr  error
	}{
		"invalid capacity": {
			capacity: 0,
			wantErr:  ErrInvalidCapacity,
		},
		"negative default ttl": {
			capacity: 1,
			options:  []Option{WithDefaultTTL(-time.Second)},
			wantErr:  ErrNegativeTTL,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](tc.capacity, tc.options...)
			if tc.wantErr != nil {
				r.Error(err)
				r.ErrorIs(err, tc.wantErr)
				return
			}

			r.NoError(err)
			cache.Close()
		})
	}
}

func TestCacheSetAndGet(t *testing.T) {
	tests := map[string]struct {
		prepare func(r *require.Assertions, c *Cache[string, int])
		wantVal int
		wantOK  bool
	}{
		"hit": {
			prepare: func(r *require.Assertions, c *Cache[string, int]) {
				r.NoError(c.Set("a", 42))
			},
			wantVal: 42,
			wantOK:  true,
		},
		"miss": {
			prepare: func(r *require.Assertions, c *Cache[string, int]) {},
			wantVal: 0,
			wantOK:  false,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](2)
			r.NoError(err)
			defer cache.Close()

			tc.prepare(r, cache)

			val, ok := cache.Get("a")
			r.Equal(tc.wantOK, ok)
			if ok {
				r.Equal(tc.wantVal, val)
			}
		})
	}
}

func TestCacheEviction(t *testing.T) {
	tests := map[string]struct {
		sequence func(r *require.Assertions, c *Cache[string, int])
	}{
		"evicts least recently used": {
			sequence: func(r *require.Assertions, c *Cache[string, int]) {
				r.NoError(c.Set("a", 1))
				r.NoError(c.Set("b", 2))
				_, _ = c.Get("a")
				r.NoError(c.Set("c", 3))

				_, okA := c.Get("a")
				_, okB := c.Get("b")
				_, okC := c.Get("c")

				r.True(okA)
				r.False(okB)
				r.True(okC)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](2)
			r.NoError(err)
			defer cache.Close()

			tc.sequence(r, cache)
		})
	}
}

func TestCacheExpiration(t *testing.T) {
	tests := map[string]struct {
		ttl     time.Duration
		delay   time.Duration
		wantHit bool
	}{
		"before expiry": {
			ttl:     50 * time.Millisecond,
			delay:   20 * time.Millisecond,
			wantHit: true,
		},
		"after expiry": {
			ttl:     20 * time.Millisecond,
			delay:   40 * time.Millisecond,
			wantHit: false,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](1)
			r.NoError(err)
			defer cache.Close()

			r.NoError(cache.SetWithTTL("k", 99, tc.ttl))
			time.Sleep(tc.delay)

			val, ok := cache.Get("k")
			r.Equal(tc.wantHit, ok)
			if ok {
				r.Equal(99, val)
			}
		})
	}
}

func TestCacheAutomaticExpiration(t *testing.T) {
	tests := map[string]struct {
		ttl            time.Duration
		cleanup        time.Duration
		wait           time.Duration
		expectPresence bool
	}{
		"expired entry removed": {
			ttl:            30 * time.Millisecond,
			cleanup:        10 * time.Millisecond,
			wait:           80 * time.Millisecond,
			expectPresence: false,
		},
		"entry survives": {
			ttl:            100 * time.Millisecond,
			cleanup:        20 * time.Millisecond,
			wait:           40 * time.Millisecond,
			expectPresence: true,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](1, WithCleanupInterval(tc.cleanup))
			r.NoError(err)
			defer cache.Close()

			r.NoError(cache.SetWithTTL("key", 123, tc.ttl))
			time.Sleep(tc.wait)

			_, ok := cache.Get("key")
			r.Equal(tc.expectPresence, ok)
		})
	}
}

func TestCacheDelete(t *testing.T) {
	tests := map[string]struct {
		setup func(r *require.Assertions, c *Cache[string, int])
		key   string
		want  bool
	}{
		"delete existing": {
			setup: func(r *require.Assertions, c *Cache[string, int]) {
				r.NoError(c.Set("keep", 1))
				r.NoError(c.Set("drop", 2))
			},
			key:  "drop",
			want: true,
		},
		"delete missing": {
			setup: func(r *require.Assertions, c *Cache[string, int]) {},
			key:   "any",
			want:  false,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			cache, err := New[string, int](2)
			r.NoError(err)
			defer cache.Close()

			tc.setup(r, cache)
			ok := cache.Delete(tc.key)
			r.Equal(tc.want, ok)
		})
	}
}

func TestCacheLenIgnoresExpired(t *testing.T) {
	r := require.New(t)
	cache, err := New[string, int](2, WithCleanupInterval(5*time.Millisecond))
	r.NoError(err)
	defer cache.Close()

	r.NoError(cache.SetWithTTL("soon", 1, 20*time.Millisecond))
	r.NoError(cache.SetWithTTL("later", 2, 200*time.Millisecond))

	time.Sleep(60 * time.Millisecond)

	r.Equal(1, cache.Len())
}

func TestSetWithTTLValidation(t *testing.T) {
	r := require.New(t)
	cache, err := New[string, int](1)
	r.NoError(err)
	defer cache.Close()

	err = cache.SetWithTTL("a", 1, -time.Second)
	r.ErrorIs(err, ErrNegativeTTL)
}

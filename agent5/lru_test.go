package agent5

import (
	"testing"
	"time"
)

func TestCache_BasicOperations(t *testing.T) {
	tests := map[string]struct {
		capacity int
		ttl      time.Duration
		ops      func(t *testing.T, c *Cache)
	}{
		"set and get": {
			capacity: 2,
			ttl:      0,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				val, ok := c.Get("key1")
				if !ok {
					t.Fatal("expected key1 to exist")
				}
				if val != "value1" {
					t.Fatalf("want value1, got %v", val)
				}
			},
		},
		"get non-existent key": {
			capacity: 2,
			ttl:      0,
			ops: func(t *testing.T, c *Cache) {
				_, ok := c.Get("nonexistent")
				if ok {
					t.Fatal("expected key to not exist")
				}
			},
		},
		"update existing key": {
			capacity: 2,
			ttl:      0,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				c.Set("key1", "value2")
				val, ok := c.Get("key1")
				if !ok {
					t.Fatal("expected key1 to exist")
				}
				if val != "value2" {
					t.Fatalf("want value2, got %v", val)
				}
				if c.Len() != 1 {
					t.Fatalf("want len 1, got %d", c.Len())
				}
			},
		},
		"delete key": {
			capacity: 2,
			ttl:      0,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				c.Delete("key1")
				_, ok := c.Get("key1")
				if ok {
					t.Fatal("expected key1 to not exist after delete")
				}
			},
		},
		"clear cache": {
			capacity: 2,
			ttl:      0,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				c.Set("key2", "value2")
				c.Clear()
				if c.Len() != 0 {
					t.Fatalf("want len 0 after clear, got %d", c.Len())
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := New(tc.capacity, tc.ttl)
			tc.ops(t, c)
		})
	}
}

func TestCache_LRUEviction(t *testing.T) {
	c := New(2, 0)

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be evicted")
	}

	_, ok = c.Get("key2")
	if !ok {
		t.Fatal("expected key2 to exist")
	}

	_, ok = c.Get("key3")
	if !ok {
		t.Fatal("expected key3 to exist")
	}

	if c.Len() != 2 {
		t.Fatalf("want len 2, got %d", c.Len())
	}
}

func TestCache_LRUOrdering(t *testing.T) {
	c := New(2, 0)

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Get("key1")
	c.Set("key3", "value3")

	_, ok := c.Get("key2")
	if ok {
		t.Fatal("expected key2 to be evicted")
	}

	_, ok = c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}

	_, ok = c.Get("key3")
	if !ok {
		t.Fatal("expected key3 to exist")
	}
}

func TestCache_Expiration(t *testing.T) {
	tests := map[string]struct {
		ttl  time.Duration
		ops  func(t *testing.T, c *Cache)
		want bool
	}{
		"item expires after TTL": {
			ttl: 50 * time.Millisecond,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				time.Sleep(100 * time.Millisecond)
			},
			want: false,
		},
		"item accessible before TTL": {
			ttl: 200 * time.Millisecond,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				time.Sleep(50 * time.Millisecond)
			},
			want: true,
		},
		"no expiration when TTL is 0": {
			ttl: 0,
			ops: func(t *testing.T, c *Cache) {
				c.Set("key1", "value1")
				time.Sleep(50 * time.Millisecond)
			},
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := New(10, tc.ttl)
			tc.ops(t, c)
			_, ok := c.Get("key1")
			if ok != tc.want {
				t.Fatalf("want %v, got %v", tc.want, ok)
			}
		})
	}
}

func TestCache_Purge(t *testing.T) {
	c := New(10, 50*time.Millisecond)

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	time.Sleep(100 * time.Millisecond)

	count := c.Purge()
	if count != 3 {
		t.Fatalf("want 3 items purged, got %d", count)
	}

	if c.Len() != 0 {
		t.Fatalf("want len 0 after purge, got %d", c.Len())
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(100, 0)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				c.Set(id*100+j, j)
				c.Get(id*100 + j)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if c.Len() > 100 {
		t.Fatalf("want len <= 100, got %d", c.Len())
	}
}

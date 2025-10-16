package lrucache

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New(10, 0)
	if c.Len() != 0 {
		t.Errorf("expected empty cache, got length %d", c.Len())
	}
	c.Close()
}

func TestSetAndGet(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	if val, ok := c.Get("key1"); !ok || val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	if val, ok := c.Get("key2"); !ok || val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	if _, ok := c.Get("nonexistent"); ok {
		t.Error("expected false for nonexistent key")
	}
}

func TestLRUEviction(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")
	c.Set("key4", "value4")

	if _, ok := c.Get("key1"); ok {
		t.Error("key1 should have been evicted")
	}

	if _, ok := c.Get("key2"); !ok {
		t.Error("key2 should still exist")
	}

	if c.Len() != 3 {
		t.Errorf("expected length 3, got %d", c.Len())
	}
}

func TestLRUOrdering(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	c.Get("key1")

	c.Set("key4", "value4")

	if _, ok := c.Get("key2"); ok {
		t.Error("key2 should have been evicted")
	}

	if _, ok := c.Get("key1"); !ok {
		t.Error("key1 should still exist after being accessed")
	}
}

func TestExpiration(t *testing.T) {
	c := New(10, 100*time.Millisecond)
	defer c.Close()

	c.Set("key1", "value1")

	if _, ok := c.Get("key1"); !ok {
		t.Error("key1 should exist immediately after setting")
	}

	time.Sleep(150 * time.Millisecond)

	if _, ok := c.Get("key1"); ok {
		t.Error("key1 should have expired")
	}
}

func TestAutoCleanup(t *testing.T) {
	c := New(10, 100*time.Millisecond)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	if c.Len() != 2 {
		t.Errorf("expected length 2, got %d", c.Len())
	}

	time.Sleep(200 * time.Millisecond)

	if c.Len() != 0 {
		t.Errorf("expected length 0 after expiration, got %d", c.Len())
	}
}

func TestUpdate(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key1", "value2")

	if val, ok := c.Get("key1"); !ok || val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	if c.Len() != 1 {
		t.Errorf("expected length 1, got %d", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Delete("key1")

	if _, ok := c.Get("key1"); ok {
		t.Error("key1 should have been deleted")
	}

	if c.Len() != 0 {
		t.Errorf("expected length 0, got %d", c.Len())
	}
}

func TestClear(t *testing.T) {
	c := New(3, 0)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected length 0, got %d", c.Len())
	}
}

func TestConcurrency(t *testing.T) {
	c := New(100, 0)
	defer c.Close()

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
}

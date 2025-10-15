package lru

import (
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	cache, err := New[string, int](2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cache.Close)

	cache.Set("alpha", 1)
	cache.Set("beta", 2)

	if v, ok := cache.Get("alpha"); !ok || v != 1 {
		t.Fatalf("expected alpha=1, got %v, %t", v, ok)
	}

	if v, ok := cache.Get("beta"); !ok || v != 2 {
		t.Fatalf("expected beta=2, got %v, %t", v, ok)
	}
}

func TestLRUEviction(t *testing.T) {
	cache, err := New[string, int](2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cache.Close)

	cache.Set("a", 1)
	cache.Set("b", 2)
	if _, ok := cache.Get("a"); !ok {
		t.Fatalf("expected a to exist")
	}

	cache.Set("c", 3)

	if _, ok := cache.Get("b"); ok {
		t.Fatalf("expected b to be evicted")
	}
	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Fatalf("expected a to be retained, got %v, %t", v, ok)
	}
	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Fatalf("expected c=3, got %v, %t", v, ok)
	}
}

func TestExpiration(t *testing.T) {
	cache, err := New[string, int](2, WithDefaultTTL(40*time.Millisecond), WithCleanupInterval(20*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cache.Close)

	cache.Set("x", 10)

	time.Sleep(60 * time.Millisecond)

	if _, ok := cache.Get("x"); ok {
		t.Fatalf("expected x to be expired")
	}
}

func TestAutomaticCleanupRemovesExpiredEntries(t *testing.T) {
	cache, err := New[string, int](2, WithCleanupInterval(15*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cache.Close)

	cache.SetWithTTL("temp", 42, 20*time.Millisecond)
	time.Sleep(70 * time.Millisecond)

	if length := cache.Len(); length != 0 {
		t.Fatalf("expected cache to be empty after cleanup, got len=%d", length)
	}
}

func TestDelete(t *testing.T) {
	cache, err := New[string, int](1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(cache.Close)

	cache.Set("item", 7)
	if !cache.Delete("item") {
		t.Fatalf("expected delete to return true")
	}
	if _, ok := cache.Get("item"); ok {
		t.Fatalf("expected item to be removed")
	}
}

func TestNewInvalidCapacity(t *testing.T) {
	if _, err := New[int, int](0); err == nil {
		t.Fatalf("expected error for zero capacity")
	}
	if _, err := New[int, int](-1); err == nil {
		t.Fatalf("expected error for negative capacity")
	}
}

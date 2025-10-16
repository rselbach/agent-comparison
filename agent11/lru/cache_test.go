package lru_test

import (
	"testing"
	"time"

	"agent11/lru"
)

func TestLRUEviction(t *testing.T) {
	cache := lru.New[string, int](2)

	cache.Set("a", 1)
	cache.Set("b", 2)

	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Fatalf("expected to get a=1, got %v, %t", v, ok)
	}

	cache.Set("c", 3) // should evict key "b"

	if _, ok := cache.Get("b"); ok {
		t.Fatalf("expected key b to be evicted")
	}

	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Fatalf("expected key a to remain, got %v, %t", v, ok)
	}

	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Fatalf("expected key c to exist, got %v, %t", v, ok)
	}
}

func TestTTLExpiration(t *testing.T) {
	cache := lru.New[string, int](2, lru.WithTTL(50*time.Millisecond))

	cache.Set("a", 1)

	time.Sleep(70 * time.Millisecond)

	if _, ok := cache.Get("a"); ok {
		t.Fatalf("expected key a to expire")
	}

	if n := cache.Len(); n != 0 {
		t.Fatalf("expected len 0 after expiration, got %d", n)
	}
}

func TestSetWithTTLOverridesDefault(t *testing.T) {
	cache := lru.New[string, int](
		2,
		lru.WithTTL(50*time.Millisecond),
	)

	cache.Set("short", 1)
	cache.SetWithTTL("long", 2, 200*time.Millisecond)

	time.Sleep(70 * time.Millisecond)

	if _, ok := cache.Get("short"); ok {
		t.Fatalf("expected short to expire")
	}

	if v, ok := cache.Get("long"); !ok || v != 2 {
		t.Fatalf("expected long to remain, got %v, %t", v, ok)
	}
}

func TestCleanupIntervalRemovesExpired(t *testing.T) {
	cache := lru.New[string, int](
		2,
		lru.WithTTL(30*time.Millisecond),
		lru.WithCleanupInterval(10*time.Millisecond),
	)
	defer cache.Close()

	cache.Set("a", 1)

	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		if _, ok := cache.Get("a"); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("item did not expire within expected time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDelete(t *testing.T) {
	cache := lru.New[string, int](2)

	cache.Set("a", 1)

	if removed := cache.Delete("a"); !removed {
		t.Fatalf("expected delete to report removal")
	}

	if _, ok := cache.Get("a"); ok {
		t.Fatalf("expected key a to be removed")
	}

	if removed := cache.Delete("missing"); removed {
		t.Fatalf("expected delete on missing key to return false")
	}
}

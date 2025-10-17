package agent14

import (
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	cache := New(Config{Capacity: 2})
	defer cache.Close()

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 0)

	v, err := cache.Get("a")
	if err != nil || v.(int) != 1 {
		t.Fatalf("expected 1, got %v, err=%v", v, err)
	}

	v, err = cache.Get("b")
	if err != nil || v.(int) != 2 {
		t.Fatalf("expected 2, got %v, err=%v", v, err)
	}
}

func TestLRUEviction(t *testing.T) {
	cache := New(Config{Capacity: 2})
	defer cache.Close()

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 0)

	cache.Get("a")

	cache.Set("c", 3, 0)

	if _, err := cache.Get("b"); err == nil {
		t.Fatal("expected b to be evicted")
	}

	if v, err := cache.Get("a"); err != nil || v.(int) != 1 {
		t.Fatalf("expected a to remain, got %v, err=%v", v, err)
	}
}

func TestExpiration(t *testing.T) {
	cache := New(Config{Capacity: 2})
	defer cache.Close()

	cache.Set("a", 1, 50*time.Millisecond)
	cache.Set("b", 2, 0)

	if _, err := cache.Get("a"); err != nil {
		t.Fatalf("expected a before expiration, got err=%v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := cache.Get("a"); err == nil {
		t.Fatal("expected a to expire")
	}

	if v, err := cache.Get("b"); err != nil || v.(int) != 2 {
		t.Fatalf("expected b to remain, got %v, err=%v", v, err)
	}
}

func TestAutoCleanup(t *testing.T) {
	cache := New(Config{Capacity: 10, CleanupInterval: 30 * time.Millisecond})
	defer cache.Close()

	cache.Set("a", 1, 30*time.Millisecond)
	cache.Set("b", 2, 0)

	time.Sleep(80 * time.Millisecond)

	if _, err := cache.Get("a"); err == nil {
		t.Fatal("expected a to be cleaned up")
	}

	if v, err := cache.Get("b"); err != nil || v.(int) != 2 {
		t.Fatalf("expected b to remain, got %v, err=%v", v, err)
	}
}

func TestDelete(t *testing.T) {
	cache := New(Config{Capacity: 5})
	defer cache.Close()

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 0)

	if !cache.Delete("a") {
		t.Fatal("expected delete to succeed")
	}

	if _, err := cache.Get("a"); err == nil {
		t.Fatal("expected a to be removed")
	}

	if cache.Delete("missing") {
		t.Fatal("expected delete of missing key to fail")
	}
}

func TestClearLen(t *testing.T) {
	cache := New(Config{Capacity: 5})
	defer cache.Close()

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 0)

	if cache.Len() != 2 {
		t.Fatalf("expected len 2, got %d", cache.Len())
	}

	cache.Clear()

	if cache.Len() != 0 {
		t.Fatalf("expected len 0, got %d", cache.Len())
	}

	if _, err := cache.Get("a"); err == nil {
		t.Fatal("expected a to be cleared")
	}
}

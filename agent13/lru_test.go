package agent13

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cache := New(10, 0)
	if cache.capacity != 10 {
		t.Errorf("expected capacity 10, got %d", cache.capacity)
	}
	if cache.Len() != 0 {
		t.Errorf("expected empty cache, got len %d", cache.Len())
	}
	cache.Close()
}

func TestSetGet(t *testing.T) {
	cache := New(3, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	if val, ok := cache.Get("key1"); !ok || val != "value1" {
		t.Errorf("expected value1, got %v, ok=%v", val, ok)
	}

	if val, ok := cache.Get("key2"); !ok || val != "value2" {
		t.Errorf("expected value2, got %v, ok=%v", val, ok)
	}

	if _, ok := cache.Get("key3"); ok {
		t.Error("expected key3 to not exist")
	}
}

func TestLRUEviction(t *testing.T) {
	cache := New(3, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)

	cache.Get("key1")

	cache.Set("key4", "value4", 0)

	if _, ok := cache.Get("key2"); ok {
		t.Error("expected key2 to be evicted")
	}

	if _, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to still exist")
	}
}

func TestExpiration(t *testing.T) {
	cache := New(10, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 100*time.Millisecond)
	cache.Set("key2", "value2", 0)

	if _, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	}

	time.Sleep(150 * time.Millisecond)

	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be expired")
	}

	if _, ok := cache.Get("key2"); !ok {
		t.Error("expected key2 to still exist")
	}
}

func TestAutoCleanup(t *testing.T) {
	cache := New(10, 50*time.Millisecond)
	defer cache.Close()

	cache.Set("key1", "value1", 100*time.Millisecond)
	cache.Set("key2", "value2", 100*time.Millisecond)
	cache.Set("key3", "value3", 0)

	if cache.Len() != 3 {
		t.Errorf("expected len 3, got %d", cache.Len())
	}

	time.Sleep(200 * time.Millisecond)

	if cache.Len() != 1 {
		t.Errorf("expected len 1 after cleanup, got %d", cache.Len())
	}

	if _, ok := cache.Get("key3"); !ok {
		t.Error("expected key3 to still exist")
	}
}

func TestUpdate(t *testing.T) {
	cache := New(3, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key1", "updated", 0)

	if val, ok := cache.Get("key1"); !ok || val != "updated" {
		t.Errorf("expected updated, got %v", val)
	}

	if cache.Len() != 1 {
		t.Errorf("expected len 1, got %d", cache.Len())
	}
}

func TestDelete(t *testing.T) {
	cache := New(3, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	if !cache.Delete("key1") {
		t.Error("expected delete to return true")
	}

	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be deleted")
	}

	if !cache.Delete("key2") {
		t.Error("expected delete to return true")
	}

	if cache.Delete("key3") {
		t.Error("expected delete of non-existent key to return false")
	}
}

func TestClear(t *testing.T) {
	cache := New(10, 0)
	defer cache.Close()

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("expected len 0 after clear, got %d", cache.Len())
	}

	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to not exist after clear")
	}
}

func TestConcurrency(t *testing.T) {
	cache := New(100, 0)
	defer cache.Close()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a' + (id+j)%26))
				cache.Set(key, id*100+j, 0)
				cache.Get(key)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

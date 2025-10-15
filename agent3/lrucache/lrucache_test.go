package lrucache

import (
	"testing"
	"time"
)

func TestCache_Get(t *testing.T) {
	c := New(1)
	c.Add("key", "value", time.Second*1)
	v, ok := c.Get("key")
	if !ok {
		t.Fatal("expected to get a value")
	}
	if v.(string) != "value" {
		t.Fatalf("expected value to be 'value', got %v", v)
	}
}

func TestCache_Eviction(t *testing.T) {
	c := New(1)
	c.Add("key1", "value1", time.Second*1)
	c.Add("key2", "value2", time.Second*1)

	if _, ok := c.Get("key1"); ok {
		t.Fatal("key1 should have been evicted")
	}

	if v, ok := c.Get("key2"); !ok || v.(string) != "value2" {
		t.Fatal("key2 should be present")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := New(1)
	c.Add("key", "value", time.Millisecond*100)

	if _, ok := c.Get("key"); !ok {
		t.Fatal("key should be present")
	}

	time.Sleep(time.Millisecond * 200)

	if _, ok := c.Get("key"); ok {
		t.Fatal("key should have expired")
	}
}

func TestCache_Remove(t *testing.T) {
	c := New(1)
	c.Add("key", "value", time.Second*1)
	c.Remove("key")

	if _, ok := c.Get("key"); ok {
		t.Fatal("key should have been removed")
	}
}

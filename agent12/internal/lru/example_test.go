package lru_test

import (
	"fmt"
	"time"

	"github.com/rselbach/agent7/internal/lru"
)

func Example() {
	// create a new cache with max size of 3 and cleanup every 5 seconds
	cache := lru.New(3, 5*time.Second)
	defer cache.Close()

	// add items with no expiration
	cache.Set("user:1", "Alice", 0)
	cache.Set("user:2", "Bob", 0)

	// add item with 10 second TTL
	cache.Set("session:abc", "token123", 10*time.Second)

	// retrieve values
	if val, ok := cache.Get("user:1"); ok {
		fmt.Printf("Found: %s\n", val)
	}

	// update existing value
	cache.Set("user:1", "Alice Smith", 0)

	// cache will automatically evict least recently used items when full
	cache.Set("user:3", "Charlie", 0)
	cache.Set("user:4", "Diana", 0) // this will evict user:2

	// check cache size
	fmt.Printf("Cache size: %d\n", cache.Len())

	// delete an item
	cache.Delete("user:3")

	// clear all items
	cache.Clear()

	// Output:
	// Found: Alice
	// Cache size: 3
}

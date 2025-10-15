package lrucache_test

import (
	"fmt"
	"time"

	"github.com/rselbach/cc/lrucache"
)

func ExampleLRUCache() {
	// create a cache with capacity of 3
	cache := lrucache.New(3)

	// add items with different TTLs
	cache.Set("user:1", "alice", time.Hour)
	cache.Set("user:2", "bob", 30*time.Minute)
	cache.Set("session:abc", "data", 5*time.Minute)

	// get items
	if value, ok := cache.Get("user:1"); ok {
		fmt.Println("Found user 1:", value)
	}

	// check length
	fmt.Println("Cache size:", cache.Len())

	// delete an item
	cache.Delete("user:2")

	// clear all items
	cache.Clear()

	// close the cache (stops cleanup goroutine)
	cache.Close()

	// Output:
	// Found user 1: alice
	// Cache size: 3
}

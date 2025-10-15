# LRU Cache with Expiration

A thread-safe Least Recently Used (LRU) cache implementation in Go with automatic expiration support.

## Features

- **Thread-safe**: Safe for concurrent access using sync.RWMutex
- **LRU eviction**: Automatically evicts least recently used items when capacity is reached
- **Automatic expiration**: Items expire after specified TTL
- **Background cleanup**: Goroutine periodically removes expired items
- **O(1) operations**: All operations (Get, Set, Delete) are O(1) time complexity

## Installation

```bash
go get github.com/rselbach/cc/lrucache
```

## Usage

```go
package main

import (
    "fmt"
    "time"

    "github.com/rselbach/cc/lrucache"
)

func main() {
    // Create a cache with capacity of 100 items
    cache := lrucache.New(100)

    // Add an item with 1 hour TTL
    cache.Set("user:123", "alice", time.Hour)

    // Retrieve an item
    if value, ok := cache.Get("user:123"); ok {
        fmt.Println("Found:", value) // Output: Found: alice
    }

    // Delete an item
    cache.Delete("user:123")

    // Clear all items
    cache.Clear()

    // Important: Close the cache when done to stop cleanup goroutine
    defer cache.Close()
}
```

## API Reference

### New(capacity int) *LRUCache
Creates a new LRU cache with the specified capacity. If capacity is <= 0, it defaults to 1.

### Set(key string, value any, ttl time.Duration)
Adds or updates a key-value pair with the specified TTL (time to live).

### Get(key string) (any, bool)
Retrieves a value by key. Returns the value and a boolean indicating if the key was found and not expired.

### Delete(key string) bool
Removes a key from the cache. Returns true if the key was found and removed.

### Clear()
Removes all items from the cache.

### Len() int
Returns the number of items currently in the cache.

### Close()
Stops the cleanup goroutine and clears the cache. Should be called when the cache is no longer needed.

## Thread Safety

All operations on the LRUCache are thread-safe and can be used concurrently from multiple goroutines.

## Performance

- Set: O(1) time complexity
- Get: O(1) time complexity
- Delete: O(1) time complexity
- Background cleanup: O(n) where n is the number of items, runs periodically

## Testing

Run tests with:

```bash
go test -v
```

## License

MIT License
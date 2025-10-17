# agent13

An LRU (Least Recently Used) cache implementation in Go with automatic expiration support.

## Features

- **LRU eviction**: Automatically removes least recently used items when capacity is reached
- **TTL expiration**: Optional time-to-live for individual cache entries
- **Automatic cleanup**: Background goroutine to periodically remove expired items
- **Thread-safe**: Safe for concurrent use with sync.RWMutex
- **O(1) operations**: Get, Set, and Delete operations are O(1)

## Usage

```go
package main

import (
    "fmt"
    "time"
    "github.com/rselbach/agent13"
)

func main() {
    // Create cache with capacity of 100 items and cleanup every 1 minute
    cache := agent13.New(100, time.Minute)
    defer cache.Close()

    // Set a value without expiration
    cache.Set("key1", "value1", 0)

    // Set a value with 5 minute TTL
    cache.Set("key2", "value2", 5*time.Minute)

    // Get a value
    if val, ok := cache.Get("key1"); ok {
        fmt.Println(val)
    }

    // Delete a value
    cache.Delete("key1")

    // Clear all items
    cache.Clear()
}
```

## API

- `New(capacity int, cleanupInterval time.Duration) *Cache` - Creates a new cache
- `Set(key string, value interface{}, ttl time.Duration)` - Sets a value with optional TTL
- `Get(key string) (interface{}, bool)` - Gets a value
- `Delete(key string) bool` - Deletes a value
- `Clear()` - Removes all items
- `Len() int` - Returns the number of items
- `Close()` - Stops the cleanup goroutine

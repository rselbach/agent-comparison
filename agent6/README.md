# LRU Cache with Automatic Expiration

A thread-safe LRU (Least Recently Used) cache implementation in Go with automatic time-based expiration.

## Features

- **LRU Eviction**: Automatically removes least recently used items when capacity is reached
- **Time-based Expiration**: Items expire after a configurable TTL (time to live)
- **Thread-safe**: All operations are protected by mutex locks
- **Automatic Cleanup**: Background goroutine removes expired items
- **Generic Keys/Values**: Supports `interface{}` for both keys and values

## Installation

```bash
go get github.com/rselbach/lrucache
```

## Usage

```go
package main

import (
    "fmt"
    "time"
    "github.com/rselbach/lrucache"
)

func main() {
    // Create cache with capacity of 100 items and 5 minute TTL
    cache := lrucache.New(100, 5*time.Minute)
    defer cache.Close()

    // Set values
    cache.Set("user:123", map[string]string{"name": "Alice"})
    cache.Set("product:456", "Product Data")

    // Get values
    if val, ok := cache.Get("user:123"); ok {
        fmt.Println("Found:", val)
    }

    // Delete specific key
    cache.Delete("user:123")

    // Clear all items
    cache.Clear()

    // Check cache size
    fmt.Println("Cache size:", cache.Len())
}
```

## API

### `New(capacity int, ttl time.Duration) *Cache`
Creates a new LRU cache with the specified capacity and TTL. Set `ttl` to 0 to disable expiration.

### `Set(key, value interface{})`
Adds or updates a key-value pair in the cache.

### `Get(key interface{}) (interface{}, bool)`
Retrieves a value from the cache. Returns the value and true if found, nil and false otherwise.

### `Delete(key interface{})`
Removes a specific key from the cache.

### `Len() int`
Returns the current number of items in the cache.

### `Clear()`
Removes all items from the cache.

### `Close()`
Stops the background cleanup goroutine. Should be called when done with the cache.

## Testing

```bash
go test -v
```

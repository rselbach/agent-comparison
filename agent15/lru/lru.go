package lru

import (
	"container/list"
	"sync"
	"time"
)

// LRU implements a least recently used cache with automatic expiration.
type LRU struct {
	capacity int
	items    map[string]*list.Element
	l        *list.List
	mu       sync.RWMutex
}

type entry struct {
	key    string
	value  any
	expire time.Time
}

// New creates a new LRU cache with the given capacity.
func New(capacity int) *LRU {
	lru := &LRU{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		l:        list.New(),
	}
	go lru.cleanup()
	return lru
}

func (lru *LRU) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		lru.mu.Lock()
		for e := lru.l.Back(); e != nil; {
			ent := e.Value.(*entry)
			if time.Now().After(ent.expire) {
				delete(lru.items, ent.key)
				prev := e.Prev()
				lru.l.Remove(e)
				e = prev
			} else {
				break
			}
		}
		lru.mu.Unlock()
	}
}

// Get retrieves the value for the given key.
// It returns the value and true if found and not expired, otherwise nil and false.
func (lru *LRU) Get(key string) (any, bool) {
	lru.mu.RLock()
	elem, ok := lru.items[key]
	lru.mu.RUnlock()
	if !ok {
		return nil, false
	}
	lru.mu.Lock()
	ent := elem.Value.(*entry)
	if time.Now().After(ent.expire) {
		delete(lru.items, key)
		lru.l.Remove(elem)
		lru.mu.Unlock()
		return nil, false
	}
	lru.l.MoveToFront(elem)
	lru.mu.Unlock()
	return ent.value, true
}

// Put adds or updates the value for the given key with the specified TTL.
// If the key already exists, it updates the value and resets the expiration.
func (lru *LRU) Put(key string, value interface{}, ttl time.Duration) {
	expire := time.Now().Add(ttl)
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if elem, ok := lru.items[key]; ok {
		lru.l.MoveToFront(elem)
		ent := elem.Value.(*entry)
		ent.value = value
		ent.expire = expire
		return
	}
	ent := &entry{key: key, value: value, expire: expire}
	elem := lru.l.PushFront(ent)
	lru.items[key] = elem
	if lru.l.Len() > lru.capacity {
		elem = lru.l.Back()
		ent = elem.Value.(*entry)
		delete(lru.items, ent.key)
		lru.l.Remove(elem)
	}
}

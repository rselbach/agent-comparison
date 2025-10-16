package agent8

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key       string
	value     any
	expiresAt time.Time
}

type LRU struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	lruList  *list.List
	ttl      time.Duration
	stopCh   chan struct{}
}

func NewLRU(capacity int, ttl time.Duration) *LRU {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	lru := &LRU{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		lruList:  list.New(),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}

	if ttl > 0 {
		go lru.cleanupExpired()
	}

	return lru
}

func (l *LRU) Set(key string, value any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	expiresAt := time.Time{}
	if l.ttl > 0 {
		expiresAt = now.Add(l.ttl)
	}

	if elem, exists := l.items[key]; exists {
		l.lruList.MoveToFront(elem)
		e := elem.Value.(*entry)
		e.value = value
		e.expiresAt = expiresAt
		return
	}

	if l.lruList.Len() >= l.capacity {
		l.evictOldest()
	}

	e := &entry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := l.lruList.PushFront(e)
	l.items[key] = elem
}

func (l *LRU) Get(key string) (any, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	elem, exists := l.items[key]
	if !exists {
		return nil, false
	}

	e := elem.Value.(*entry)

	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		l.removeElement(elem)
		return nil, false
	}

	l.lruList.MoveToFront(elem)
	return e.value, true
}

func (l *LRU) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, exists := l.items[key]; exists {
		l.removeElement(elem)
	}
}

func (l *LRU) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lruList.Len()
}

func (l *LRU) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[string]*list.Element)
	l.lruList.Init()
}

func (l *LRU) Close() {
	close(l.stopCh)
}

func (l *LRU) evictOldest() {
	elem := l.lruList.Back()
	if elem != nil {
		l.removeElement(elem)
	}
}

func (l *LRU) removeElement(elem *list.Element) {
	l.lruList.Remove(elem)
	e := elem.Value.(*entry)
	delete(l.items, e.key)
}

func (l *LRU) cleanupExpired() {
	ticker := time.NewTicker(l.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.removeExpiredEntries()
		case <-l.stopCh:
			return
		}
	}
}

func (l *LRU) removeExpiredEntries() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	var toRemove []*list.Element

	for elem := l.lruList.Back(); elem != nil; elem = elem.Prev() {
		e := elem.Value.(*entry)
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			toRemove = append(toRemove, elem)
		}
	}

	for _, elem := range toRemove {
		l.removeElement(elem)
	}
}

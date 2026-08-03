package utils

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

type LRUCache struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	capacity int
	ttl      time.Duration
}

func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		capacity: capacity,
		ttl:      ttl,
	}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.order.Remove(elem)
		delete(c.items, key)
		return nil, false
	}

	c.order.MoveToFront(elem)
	return entry.value, true
}

func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = time.Now().Add(c.ttl)
		return
	}

	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

var FtsCache = NewLRUCache(500, 2*time.Hour)

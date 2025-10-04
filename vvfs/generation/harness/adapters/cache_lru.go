package adapters

import (
	"context"
	"sync"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// LRUCache implements a simple LRU cache with TTL support.
type LRUCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*cacheItem
	head     *cacheItem
	tail     *cacheItem
}

type cacheItem struct {
	key   string
	value []byte
	ttl   time.Time
	prev  *cacheItem
	next  *cacheItem
}

// NewLRUCache creates a new LRU cache with the specified capacity.
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[string]*cacheItem),
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(ctx context.Context, key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check TTL
	if time.Now().After(item.ttl) {
		// Expired, remove from cache
		c.removeItem(item)
		delete(c.items, key)
		return nil, false
	}

	// Move to front (most recently used)
	c.moveToFront(item)

	return item.value, true
}

// Set stores a value in the cache with TTL.
func (c *LRUCache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ttl := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	// Check if key exists
	if item, exists := c.items[key]; exists {
		// Update existing item
		item.value = value
		item.ttl = ttl
		c.moveToFront(item)
		return nil
	}

	// Create new item
	item := &cacheItem{
		key:   key,
		value: value,
		ttl:   ttl,
	}

	// Add to front
	c.addToFront(item)
	c.items[key] = item

	// Evict if over capacity
	if len(c.items) > c.capacity {
		c.evictLRU()
	}

	return nil
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil
	}

	c.removeItem(item)
	delete(c.items, key)
	return nil
}

// moveToFront moves an item to the front of the LRU list.
func (c *LRUCache) moveToFront(item *cacheItem) {
	if item == c.head {
		return
	}

	c.removeItem(item)
	c.addToFront(item)
}

// addToFront adds an item to the front of the LRU list.
func (c *LRUCache) addToFront(item *cacheItem) {
	item.next = c.head
	item.prev = nil

	if c.head != nil {
		c.head.prev = item
	}
	c.head = item

	if c.tail == nil {
		c.tail = item
	}
}

// removeItem removes an item from the LRU list.
func (c *LRUCache) removeItem(item *cacheItem) {
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		c.head = item.next
	}

	if item.next != nil {
		item.next.prev = item.prev
	} else {
		c.tail = item.prev
	}

	item.prev = nil
	item.next = nil
}

// evictLRU removes the least recently used item.
func (c *LRUCache) evictLRU() {
	if c.tail == nil {
		return
	}

	item := c.tail
	c.removeItem(item)
	delete(c.items, item.key)
}

// Ensure LRUCache implements the Cache interface.
var _ ports.Cache = (*LRUCache)(nil)

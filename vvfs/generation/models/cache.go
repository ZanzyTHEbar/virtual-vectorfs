package models

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"
)

// EmbeddingCache provides LRU cache for embeddings
type EmbeddingCache struct {
	cache   map[string]*cacheEntry
	order   []string
	maxSize int
	mu      sync.RWMutex
	ttl     time.Duration
}

// cacheEntry represents a cached embedding
type cacheEntry struct {
	embedding []float32
	timestamp time.Time
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	return &EmbeddingCache{
		cache:   make(map[string]*cacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     24 * time.Hour, // 24 hour TTL
	}
}

// Get retrieves an embedding from cache
func (c *EmbeddingCache) Get(key string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.timestamp) > c.ttl {
		// Expired, remove from cache
		delete(c.cache, key)
		c.removeFromOrder(key)
		return nil, false
	}

	return entry.embedding, true
}

// Set stores an embedding in cache
func (c *EmbeddingCache) Set(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it and move to end of order
	if entry, exists := c.cache[key]; exists {
		entry.embedding = embedding
		entry.timestamp = time.Now()
		c.moveToEnd(key)
		return
	}

	// Add new entry
	c.cache[key] = &cacheEntry{
		embedding: embedding,
		timestamp: time.Now(),
	}
	c.order = append(c.order, key)

	// Evict if over capacity
	if len(c.cache) > c.maxSize {
		c.evictOldest()
	}
}

// GenerateCacheKey generates a cache key for embedding
func (c *EmbeddingCache) GenerateCacheKey(text string, modelType ModelType) string {
	// Create hash of text + model type for cache key
	hash := md5.Sum([]byte(text + string(modelType)))
	return fmt.Sprintf("%x", hash)
}

// Clear removes all entries from cache
func (c *EmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
	c.order = make([]string, 0, c.maxSize)
}

// Size returns current cache size
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Stats returns cache statistics
func (c *EmbeddingCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"size":      len(c.cache),
		"max_size":  c.maxSize,
		"hit_rate":  "N/A", // Would need to track hits/misses
		"ttl_hours": c.ttl.Hours(),
	}
}

// evictOldest removes the oldest entry from cache
func (c *EmbeddingCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	oldestKey := c.order[0]
	delete(c.cache, oldestKey)
	c.order = c.order[1:]
}

// moveToEnd moves a key to the end of the order slice
func (c *EmbeddingCache) moveToEnd(key string) {
	// Find key in order
	for i, k := range c.order {
		if k == key {
			// Move to end
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

// removeFromOrder removes a key from the order slice
func (c *EmbeddingCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

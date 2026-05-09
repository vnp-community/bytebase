package cache

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// LRUCache wraps hashicorp/golang-lru to implement the Cache interface.
type LRUCache[K comparable, V any] struct {
	inner *lru.Cache[K, V]
}

// NewLRU creates an in-process LRU cache with the given capacity.
func NewLRU[K comparable, V any](capacity int) (*LRUCache[K, V], error) {
	inner, err := lru.New[K, V](capacity)
	if err != nil {
		return nil, err
	}
	return &LRUCache[K, V]{inner: inner}, nil
}

// Get retrieves a cached value. TTL is not applicable for LRU (eviction is size-based).
func (c *LRUCache[K, V]) Get(_ context.Context, key K) (V, bool, error) {
	v, ok := c.inner.Get(key)
	return v, ok, nil
}

// Set stores a value. TTL is ignored for LRU (eviction is size-based only).
func (c *LRUCache[K, V]) Set(_ context.Context, key K, value V, _ time.Duration) error {
	c.inner.Add(key, value)
	return nil
}

// Delete removes a single key.
func (c *LRUCache[K, V]) Delete(_ context.Context, key K) error {
	c.inner.Remove(key)
	return nil
}

// Purge removes all entries.
func (c *LRUCache[K, V]) Purge(_ context.Context) error {
	c.inner.Purge()
	return nil
}

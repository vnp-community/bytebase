package cache

import (
	"context"
	"time"
)

// NoopCache is a cache that never stores anything.
// Useful for testing and for disabling caching entirely.
type NoopCache[K comparable, V any] struct{}

// NewNoop creates a no-op cache that always returns misses.
func NewNoop[K comparable, V any]() *NoopCache[K, V] {
	return &NoopCache[K, V]{}
}

// Get always returns a miss.
func (c *NoopCache[K, V]) Get(_ context.Context, _ K) (V, bool, error) {
	var zero V
	return zero, false, nil
}

// Set does nothing.
func (c *NoopCache[K, V]) Set(_ context.Context, _ K, _ V, _ time.Duration) error {
	return nil
}

// Delete does nothing.
func (c *NoopCache[K, V]) Delete(_ context.Context, _ K) error {
	return nil
}

// Purge does nothing.
func (c *NoopCache[K, V]) Purge(_ context.Context) error {
	return nil
}

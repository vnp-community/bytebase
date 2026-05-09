// Package cache provides a pluggable cache interface for the Store layer.
// Supported backends: LRU (in-process), Noop (disabled), Redis (distributed).
package cache

import (
	"context"
	"time"
)

// Backend identifies the cache implementation to use.
type Backend string

const (
	// BackendLRU uses an in-process LRU cache (default, single-instance mode).
	BackendLRU Backend = "lru"
	// BackendRedis uses Redis/Valkey for distributed caching (HA mode).
	BackendRedis Backend = "redis"
	// BackendNoop disables caching entirely (testing or debug).
	BackendNoop Backend = "noop"
)

// Cache is a generic key-value cache with TTL support.
// Implementations must be safe for concurrent use.
type Cache[K comparable, V any] interface {
	// Get retrieves a value by key.
	// Returns (value, true, nil) on hit, (zero, false, nil) on miss.
	// Errors are reserved for backend failures (e.g., Redis network error).
	Get(ctx context.Context, key K) (V, bool, error)

	// Set stores a value with an optional TTL.
	// If ttl is 0, the entry never expires (backend-dependent).
	Set(ctx context.Context, key K, value V, ttl time.Duration) error

	// Delete removes a single key.
	Delete(ctx context.Context, key K) error

	// Purge removes all entries.
	Purge(ctx context.Context) error
}

// Codec serializes/deserializes values for backends that need wire encoding (e.g., Redis).
type Codec[V any] interface {
	Marshal(v V) ([]byte, error)
	Unmarshal(data []byte) (V, error)
}

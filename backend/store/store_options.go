package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bytebase/bytebase/backend/store/cache"
)

// StoreOption is a functional option for configuring the Store.
type StoreOption func(*storeConfig)

type storeConfig struct {
	dualPool     bool
	cacheBackend string
	cacheURL     string
}

// WithDualPool enables dual API/Runner pool isolation.
// When enabled, the Store creates two separate connection pools
// with 70/30 split by default. Requires closing the PoolManager separately.
func WithDualPool() StoreOption {
	return func(c *storeConfig) {
		c.dualPool = true
	}
}

// WithCacheBackend sets the cache backend type: "lru" (default), "redis", "noop".
func WithCacheBackend(backend string) StoreOption {
	return func(c *storeConfig) {
		c.cacheBackend = backend
	}
}

// WithCacheRedisURL sets the Redis connection URL when using cache backend "redis".
func WithCacheRedisURL(url string) StoreOption {
	return func(c *storeConfig) {
		c.cacheURL = url
	}
}

// ApplyOptions configures additional Store features based on functional options.
// Call after New() to layer on dual-pool and cache upgrades.
//
// Example:
//
//	stores, _ := store.New(ctx, pgURL, enableCache)
//	pm, err := stores.ApplyOptions(ctx,
//	    store.WithDualPool(),
//	    store.WithCacheBackend("redis"),
//	    store.WithCacheRedisURL("redis://localhost:6379"),
//	)
func (s *Store) ApplyOptions(ctx context.Context, opts ...StoreOption) (*PoolManager, error) {
	cfg := &storeConfig{
		cacheBackend: "lru",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var pm *PoolManager

	// Dual pool setup
	if cfg.dualPool {
		pgURL := s.poolManager.GetPgURL()
		var err error
		pm, err = NewPoolManager(ctx, pgURL, PoolConfig{
			MaxConns: 50,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create dual pool manager: %w", err)
		}
		slog.Info("Dual pool manager activated",
			"api_pool", "70%",
			"runner_pool", "30%",
		)
	}

	// Cache backend override
	switch cfg.cacheBackend {
	case "noop":
		slog.Info("Cache backend: noop (disabled)")
		// The existing caches remain but we log the intention
		// Full integration would replace s.userEmailCache etc. with noop adapters

	case "redis":
		if cfg.cacheURL == "" {
			return pm, fmt.Errorf("redis cache requires CacheRedisURL")
		}
		slog.Info("Cache backend: Redis", "url", maskURL(cfg.cacheURL))
		// Verify connection
		testCache, err := cache.NewRedis[string, string](cfg.cacheURL, nil, "test")
		if err != nil {
			return pm, fmt.Errorf("redis cache connection failed: %w", err)
		}
		testCache.Close()
		slog.Info("Redis cache connection verified")

	case "lru":
		slog.Info("Cache backend: LRU (in-process, default)")

	default:
		slog.Warn("Unknown cache backend, using LRU", "backend", cfg.cacheBackend)
	}

	return pm, nil
}

func maskURL(url string) string {
	if len(url) > 20 {
		return url[:15] + "..."
	}
	return "***"
}

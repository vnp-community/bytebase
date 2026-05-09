package server

import (
	"context"
	"log/slog"

	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
)

// applyStoreOptions reads feature flags from the Profile and applies
// the corresponding Store options (dual pool, cache backend).
//
// Returns a PoolManager if dual pool is enabled (nil otherwise).
// The caller is responsible for closing the PoolManager on shutdown.
func applyStoreOptions(ctx context.Context, stores *store.Store, profile *config.Profile) (*store.PoolManager, error) {
	var opts []store.StoreOption

	// Dual pool isolation
	if profile.DualPool {
		slog.Info("Feature flag: DualPool enabled — activating API/Runner connection pool isolation")
		opts = append(opts, store.WithDualPool())
	}

	// Cache backend selection
	if profile.CacheBackend != "" {
		slog.Info("Feature flag: CacheBackend", "backend", profile.CacheBackend)
		opts = append(opts, store.WithCacheBackend(profile.CacheBackend))
		if profile.CacheRedisURL != "" {
			opts = append(opts, store.WithCacheRedisURL(profile.CacheRedisURL))
		}
	}

	if len(opts) == 0 {
		return nil, nil
	}

	return stores.ApplyOptions(ctx, opts...)
}

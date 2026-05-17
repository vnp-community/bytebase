package store

import (
	"context"
	"log/slog"
	"time"
)

// WarmDatabaseCache pre-populates the database cache at startup to reduce cold-start latency.
func (s *Store) WarmDatabaseCache(ctx context.Context) {
	if !s.enableCache {
		return
	}
	start := time.Now()
	limit := 10000

	databases, err := s.ListDatabases(ctx, &FindDatabaseMessage{
		Limit: &limit,
	})
	if err != nil {
		slog.Warn("Cache warming failed", "error", err)
		return
	}

	slog.Info("Cache warming completed",
		"databases", len(databases),
		"duration", time.Since(start),
	)
}

package monitor

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

type PoolMonitor struct {
	db *sql.DB
}

func NewPoolMonitor(db *sql.DB) *PoolMonitor {
	return &PoolMonitor{db: db}
}

func (m *PoolMonitor) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			stats := m.db.Stats()
			if stats.MaxOpenConnections > 0 {
				util := float64(stats.InUse) / float64(stats.MaxOpenConnections)
				if util > 0.8 {
					slog.Warn("DB pool high utilization",
						slog.Float64("utilization", util),
						slog.Int("inUse", stats.InUse),
						slog.Int("maxOpen", stats.MaxOpenConnections))
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

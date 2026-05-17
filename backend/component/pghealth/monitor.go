// Package pghealth provides a background monitor for embedded PostgreSQL health metrics.
// It collects key database metrics (connections, size, WAL, long queries) and exports
// them as Prometheus gauges. The monitor is a no-op when using external PostgreSQL.
package pghealth

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
)

const (
	defaultCollectInterval = 30 * time.Second
)

// Monitor collects embedded PG health metrics and exports them to Prometheus.
type Monitor struct {
	db      *sql.DB
	profile *config.Profile

	activeConns    prometheus.Gauge
	dbSizeMB       prometheus.Gauge
	longestQueryS  prometheus.Gauge
	walSizeMB      prometheus.Gauge
}

// NewMonitor creates a PG health monitor. It registers Prometheus metrics.
func NewMonitor(db *sql.DB, profile *config.Profile, registerer prometheus.Registerer) *Monitor {
	m := &Monitor{
		db:      db,
		profile: profile,
		activeConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "pg",
			Name:      "active_connections",
			Help:      "Number of active connections to embedded PostgreSQL.",
		}),
		dbSizeMB: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "pg",
			Name:      "database_size_mb",
			Help:      "Size of the Bytebase database in megabytes.",
		}),
		longestQueryS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "pg",
			Name:      "longest_query_seconds",
			Help:      "Duration in seconds of the longest running query.",
		}),
		walSizeMB: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "pg",
			Name:      "wal_size_mb",
			Help:      "Size of WAL files in megabytes.",
		}),
	}

	if registerer != nil {
		registerer.MustRegister(m.activeConns, m.dbSizeMB, m.longestQueryS, m.walSizeMB)
	}

	return m
}

// Run starts the health monitor background loop. It collects metrics every 30 seconds.
// The monitor only runs when embedded PG is in use (profile.UseEmbedDB() == true).
func (m *Monitor) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	if !m.profile.UseEmbedDB() {
		slog.Info("PG health monitor disabled — using external PostgreSQL")
		return
	}

	slog.Info("PG health monitor started", "interval", defaultCollectInterval)
	ticker := time.NewTicker(defaultCollectInterval)
	defer ticker.Stop()

	// Collect immediately on start.
	m.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("PG health monitor stopped")
			return
		case <-ticker.C:
			m.collect(ctx)
		}
	}
}

func (m *Monitor) collect(ctx context.Context) {
	m.collectActiveConnections(ctx)
	m.collectDatabaseSize(ctx)
	m.collectLongestQuery(ctx)
	m.collectWALSize(ctx)
}

func (m *Monitor) collectActiveConnections(ctx context.Context) {
	var count float64
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'`).Scan(&count)
	if err != nil {
		slog.Debug("pghealth: failed to collect active connections", log.BBError(err))
		return
	}
	m.activeConns.Set(count)
}

func (m *Monitor) collectDatabaseSize(ctx context.Context) {
	var sizeMB float64
	err := m.db.QueryRowContext(ctx,
		`SELECT pg_database_size(current_database()) / (1024 * 1024)`).Scan(&sizeMB)
	if err != nil {
		slog.Debug("pghealth: failed to collect database size", log.BBError(err))
		return
	}
	m.dbSizeMB.Set(sizeMB)
}

func (m *Monitor) collectLongestQuery(ctx context.Context) {
	var seconds sql.NullFloat64
	err := m.db.QueryRowContext(ctx,
		`SELECT EXTRACT(EPOCH FROM MAX(now() - query_start))
		 FROM pg_stat_activity
		 WHERE state = 'active' AND pid != pg_backend_pid()`).Scan(&seconds)
	if err != nil {
		slog.Debug("pghealth: failed to collect longest query", log.BBError(err))
		return
	}
	if seconds.Valid {
		m.longestQueryS.Set(seconds.Float64)
	} else {
		m.longestQueryS.Set(0)
	}
}

func (m *Monitor) collectWALSize(ctx context.Context) {
	var sizeMB sql.NullFloat64
	err := m.db.QueryRowContext(ctx,
		`SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0') / (1024 * 1024)`).Scan(&sizeMB)
	if err != nil {
		// WAL functions may not be available in all PG versions or configurations.
		slog.Debug("pghealth: failed to collect WAL size", log.BBError(err))
		return
	}
	if sizeMB.Valid {
		m.walSizeMB.Set(sizeMB.Float64)
	}
}

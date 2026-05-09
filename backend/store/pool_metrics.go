package store

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PoolMetrics exposes database connection pool metrics via Prometheus.
type PoolMetrics struct {
	maxOpen    *prometheus.GaugeVec
	open       *prometheus.GaugeVec
	inUse      *prometheus.GaugeVec
	idle       *prometheus.GaugeVec
	waitCount  *prometheus.CounterVec
	waitTime   *prometheus.CounterVec
}

// NewPoolMetrics creates and registers Prometheus metrics for connection pools.
// The "pool" label distinguishes pools (e.g., "api", "runner", "metadata").
func NewPoolMetrics(registerer prometheus.Registerer) *PoolMetrics {
	m := &PoolMetrics{
		maxOpen: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "max_open_connections",
			Help:      "Maximum number of open connections to the database.",
		}, []string{"pool"}),
		open: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "open_connections",
			Help:      "Number of established connections (in-use + idle).",
		}, []string{"pool"}),
		inUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "in_use_connections",
			Help:      "Number of connections currently in use.",
		}, []string{"pool"}),
		idle: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "idle_connections",
			Help:      "Number of idle connections.",
		}, []string{"pool"}),
		waitCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "wait_count_total",
			Help:      "Total number of connections waited for.",
		}, []string{"pool"}),
		waitTime: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "wait_duration_seconds_total",
			Help:      "Total time blocked waiting for a new connection (seconds).",
		}, []string{"pool"}),
	}

	if registerer != nil {
		registerer.MustRegister(m.maxOpen, m.open, m.inUse, m.idle, m.waitCount, m.waitTime)
	}

	return m
}

// Collect snapshots pool stats and publishes them as Prometheus metrics.
func (m *PoolMetrics) Collect(poolName string, db *sql.DB) {
	if db == nil {
		return
	}
	stats := db.Stats()
	m.maxOpen.WithLabelValues(poolName).Set(float64(stats.MaxOpenConnections))
	m.open.WithLabelValues(poolName).Set(float64(stats.OpenConnections))
	m.inUse.WithLabelValues(poolName).Set(float64(stats.InUse))
	m.idle.WithLabelValues(poolName).Set(float64(stats.Idle))
	m.waitCount.WithLabelValues(poolName).Add(float64(stats.WaitCount))
	m.waitTime.WithLabelValues(poolName).Add(stats.WaitDuration.Seconds())
}

// RunCollector starts a background goroutine that periodically collects pool metrics.
func (m *PoolMetrics) RunCollector(ctx context.Context, interval time.Duration, pools map[string]*sql.DB) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for name, db := range pools {
					m.Collect(name, db)
				}
			}
		}
	}()
	slog.Info("Pool metrics collector started", "interval", interval, "pools", len(pools))
}

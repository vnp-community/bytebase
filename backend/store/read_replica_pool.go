package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/bytebase/bytebase/backend/common/log"
)

var replicaLagGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "bytebase",
	Subsystem: "db",
	Name:      "replica_lag_seconds",
	Help:      "Replication lag of the read replica in seconds.",
})

// ReadReplicaPool manages a primary (read-write) database connection and an
// optional read replica (read-only) with lag-aware routing.
//
// When a replica is configured:
//   - ForRead() returns the replica if lag is below the threshold, primary otherwise.
//   - ForWrite() always returns the primary.
//
// When no replica is configured, both ForRead() and ForWrite() return the primary.
type ReadReplicaPool struct {
	primary      *sql.DB
	replica      *sql.DB        // nil if no replica
	replicaLag   atomic.Int64   // microseconds
	lagThreshold time.Duration
	cancel       context.CancelFunc
}

// ReadReplicaConfig configures the read replica pool.
type ReadReplicaConfig struct {
	// PrimaryURL is the connection URL for the primary (read-write) database.
	PrimaryURL string
	// ReplicaURL is the connection URL for the read replica. Empty = no replica.
	ReplicaURL string
	// LagThreshold is the maximum acceptable replication lag.
	// If lag exceeds this, ForRead() falls back to primary. Default: 5s.
	LagThreshold time.Duration
}

// NewReadReplicaPool creates a pool with primary and optional read replica.
func NewReadReplicaPool(ctx context.Context, cfg ReadReplicaConfig) (*ReadReplicaPool, error) {
	if cfg.LagThreshold <= 0 {
		cfg.LagThreshold = 5 * time.Second
	}

	primary, err := createConnectionWithTracer(ctx, cfg.PrimaryURL)
	if err != nil {
		return nil, fmt.Errorf("read replica pool: failed to connect primary: %w", err)
	}

	pool := &ReadReplicaPool{
		primary:      primary,
		lagThreshold: cfg.LagThreshold,
	}

	if cfg.ReplicaURL != "" {
		replica, err := createConnectionWithTracer(ctx, cfg.ReplicaURL)
		if err != nil {
			slog.Warn("Read replica connection failed, using primary only",
				log.BBError(err),
			)
		} else {
			pool.replica = replica

			// Start lag monitoring
			monCtx, cancel := context.WithCancel(ctx)
			pool.cancel = cancel
			go pool.monitorReplicaLag(monCtx)

			slog.Info("Read replica pool initialized",
				"lagThreshold", cfg.LagThreshold,
			)
		}
	} else {
		slog.Info("Read replica pool: no replica URL, using primary only")
	}

	return pool, nil
}

// ForRead returns the appropriate database for read queries.
// If a healthy replica is available and lag is within threshold, returns replica.
// Otherwise returns primary.
func (p *ReadReplicaPool) ForRead() *sql.DB {
	if p.replica == nil {
		return p.primary
	}

	lagMicros := p.replicaLag.Load()
	lagDuration := time.Duration(lagMicros) * time.Microsecond

	if lagDuration > p.lagThreshold {
		slog.Debug("Replica lag exceeds threshold, routing to primary",
			"lag", lagDuration,
			"threshold", p.lagThreshold,
		)
		return p.primary
	}

	return p.replica
}

// ForWrite always returns the primary database.
func (p *ReadReplicaPool) ForWrite() *sql.DB {
	return p.primary
}

// ReplicaLag returns the current replication lag.
func (p *ReadReplicaPool) ReplicaLag() time.Duration {
	return time.Duration(p.replicaLag.Load()) * time.Microsecond
}

// Close closes both primary and replica connections.
func (p *ReadReplicaPool) Close() error {
	if p.cancel != nil {
		p.cancel()
	}

	var errs []error
	if p.replica != nil {
		if err := p.replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("replica: %w", err))
		}
	}
	if err := p.primary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("primary: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("read replica pool close errors: %v", errs)
	}
	return nil
}

// monitorReplicaLag periodically queries the replica for replication lag
// and updates the atomic lag counter.
func (p *ReadReplicaPool) monitorReplicaLag(ctx context.Context) {
	const monitorInterval = 5 * time.Second

	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	slog.Debug("Replica lag monitor started", "interval", monitorInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkReplicaLag(ctx)
		}
	}
}

// checkReplicaLag queries the replica for its replication lag.
func (p *ReadReplicaPool) checkReplicaLag(ctx context.Context) {
	if p.replica == nil {
		return
	}

	var lagSeconds float64
	err := p.replica.QueryRowContext(ctx,
		"SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))",
	).Scan(&lagSeconds)

	if err != nil {
		slog.Warn("Failed to check replica lag, falling back to max",
			log.BBError(err),
		)
		// Set to max int64 to force fallback to primary
		p.replicaLag.Store(int64(p.lagThreshold.Microseconds()) + 1)
		replicaLagGauge.Set(-1) // Indicate error
		return
	}

	lagMicros := int64(lagSeconds * 1_000_000)
	p.replicaLag.Store(lagMicros)
	replicaLagGauge.Set(lagSeconds)

	if time.Duration(lagMicros)*time.Microsecond > p.lagThreshold {
		slog.Warn("Replica lag exceeds threshold",
			"lag_seconds", lagSeconds,
			"threshold", p.lagThreshold,
		)
	}
}

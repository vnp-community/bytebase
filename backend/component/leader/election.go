// Package leader provides leader election using PostgreSQL advisory locks.
// Only one replica holds a given lock at any time — suitable for HA deployments
// where exclusive background runners must run on a single node.
package leader

import (
	"context"
	"database/sql"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/bytebase/bytebase/backend/common/log"
)

// Advisory lock IDs for exclusive runners.
const (
	LockIDTaskScheduler int64 = 100001
	LockIDPlanCheck     int64 = 100002
	LockIDSchemaSync    int64 = 100003
	LockIDApproval      int64 = 100004
	LockIDDataCleaner   int64 = 100005
)

var leaderGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "bytebase",
	Name:      "leader_status",
	Help:      "1 if this replica holds the leader lock, 0 otherwise.",
}, []string{"lock_id", "runner"})

// LeaderElector manages a single advisory lock for leader election.
// Only one replica can hold a lock ID at any time. If the holder crashes,
// PG automatically releases the session-level lock.
type LeaderElector struct {
	db        *sql.DB
	lockID    int64
	name      string
	isLeader  atomic.Bool
	renewTick time.Duration
	conn      *sql.Conn // dedicated connection holding the lock
}

// NewLeaderElector creates a new leader elector.
// renewTick controls how frequently leadership is checked/renewed.
func NewLeaderElector(db *sql.DB, lockID int64, renewTick time.Duration, name string) *LeaderElector {
	return &LeaderElector{
		db:        db,
		lockID:    lockID,
		name:      name,
		renewTick: renewTick,
	}
}

// Run starts the election loop. It periodically tries to acquire the advisory
// lock and updates the isLeader flag. Call from a goroutine.
func (le *LeaderElector) Run(ctx context.Context) {
	ticker := time.NewTicker(le.renewTick)
	defer ticker.Stop()

	slog.Info("Leader elector started",
		"runner", le.name,
		"lockID", le.lockID,
		"renewTick", le.renewTick,
	)

	// Try immediately on start
	le.tryAcquire(ctx)

	for {
		select {
		case <-ctx.Done():
			le.release()
			return
		case <-ticker.C:
			le.tryAcquire(ctx)
		}
	}
}

// IsLeader returns true if this replica currently holds the leader lock.
func (le *LeaderElector) IsLeader() bool {
	return le.isLeader.Load()
}

// tryAcquire attempts to acquire the advisory lock if not already held.
func (le *LeaderElector) tryAcquire(ctx context.Context) {
	if le.isLeader.Load() {
		// Already leader — verify connection is still alive
		if le.conn != nil {
			if err := le.conn.PingContext(ctx); err != nil {
				slog.Warn("Leader connection lost, releasing",
					"runner", le.name,
					log.BBError(err),
				)
				le.release()
			}
		}
		return
	}

	conn, err := le.db.Conn(ctx)
	if err != nil {
		slog.Warn("Leader elector: failed to get connection",
			"runner", le.name,
			log.BBError(err),
		)
		return
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx,
		"SELECT pg_try_advisory_lock($1)", le.lockID,
	).Scan(&acquired); err != nil {
		conn.Close()
		slog.Warn("Leader elector: lock query failed",
			"runner", le.name,
			log.BBError(err),
		)
		return
	}

	if !acquired {
		conn.Close()
		le.isLeader.Store(false)
		leaderGauge.WithLabelValues(formatLockID(le.lockID), le.name).Set(0)
		return
	}

	le.conn = conn
	le.isLeader.Store(true)
	leaderGauge.WithLabelValues(formatLockID(le.lockID), le.name).Set(1)
	slog.Info("Leader elected", "runner", le.name, "lockID", le.lockID)
}

// release releases the advisory lock and closes the dedicated connection.
func (le *LeaderElector) release() {
	le.isLeader.Store(false)
	leaderGauge.WithLabelValues(formatLockID(le.lockID), le.name).Set(0)

	if le.conn != nil {
		// Explicit unlock (connection close also releases, but explicit is cleaner)
		_, _ = le.conn.ExecContext(context.Background(),
			"SELECT pg_advisory_unlock($1)", le.lockID,
		)
		le.conn.Close()
		le.conn = nil
		slog.Info("Leader released", "runner", le.name, "lockID", le.lockID)
	}
}

func formatLockID(id int64) string {
	return slog.Int64("", id).Value.String()
}

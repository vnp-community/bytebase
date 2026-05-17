package replication

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
)

var (
	lagGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bytebase_replication_lag_seconds",
		Help: "Replication lag in seconds",
	})
	statusGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bytebase_replication_status",
		Help: "Replication status (2=SYNCED, 1=LAGGING, 0=BROKEN)",
	})
)

type Monitor struct {
	db      *sql.DB
	profile *config.Profile
}

func NewMonitor(db *sql.DB, profile *config.Profile) *Monitor {
	return &Monitor{
		db:      db,
		profile: profile,
	}
}

func (m *Monitor) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkReplication(ctx)
		}
	}
}

func (m *Monitor) checkReplication(ctx context.Context) {
	if m.profile.IsPrimary() {
		m.checkPrimary(ctx)
	} else {
		m.checkStandby(ctx)
	}
}

func (m *Monitor) checkPrimary(ctx context.Context) {
	// On primary, check pg_stat_replication
	rows, err := m.db.QueryContext(ctx, `
		SELECT application_name, client_addr, state, 
		       EXTRACT(EPOCH FROM (pg_current_wal_lsn() - replay_lsn)) AS lag_bytes
		FROM pg_stat_replication
	`)
	if err != nil {
		slog.Error("Failed to query pg_stat_replication", log.BBError(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var appName, clientAddr, state string
		var lagBytes sql.NullFloat64
		if err := rows.Scan(&appName, &clientAddr, &state, &lagBytes); err != nil {
			slog.Error("Failed to scan pg_stat_replication row", log.BBError(err))
			continue
		}
		slog.Debug("Standby status", 
			slog.String("app", appName), 
			slog.String("client", clientAddr), 
			slog.String("state", state), 
			slog.Float64("lag_bytes", lagBytes.Float64))
	}
}

func (m *Monitor) checkStandby(ctx context.Context) {
	// On standby, check replay timestamp
	var lagSeconds sql.NullFloat64
	err := m.db.QueryRowContext(ctx, `
		SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))
	`).Scan(&lagSeconds)
	
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to check standby replay lag", log.BBError(err))
		statusGauge.Set(0)
		return
	}

	if !lagSeconds.Valid {
		// Possibly no transactions replayed yet or not a standby
		statusGauge.Set(2)
		return
	}

	lag := lagSeconds.Float64
	lagGauge.Set(lag)

	if lag < 30 {
		statusGauge.Set(2) // SYNCED
	} else if lag <= 300 {
		statusGauge.Set(1) // LAGGING
		slog.Warn("Standby is lagging", slog.Float64("lag_seconds", lag))
	} else {
		statusGauge.Set(0) // BROKEN
		slog.Error("Standby replication is broken/stale", slog.Float64("lag_seconds", lag))
	}
}

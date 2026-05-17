package bus

import (
	"database/sql"
	"log/slog"

	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/prometheus/client_golang/prometheus"
)

// NewEventBus creates the appropriate EventBus implementation based on the profile.
//   - Single-node (HA=false): returns the channel-based Bus (volatile, fast)
//   - HA mode (HA=true): returns PGBus backed by bus_queue table (durable, cross-replica)
func NewEventBus(profile *config.Profile, db *sql.DB) (EventBus, error) {
	if profile.HA && db != nil {
		slog.Info("Bus factory: creating PGBus (HA mode)")
		metrics := NewBusMetrics(prometheus.DefaultRegisterer, db)
		return NewPGBus(db, metrics), nil
	}

	slog.Info("Bus factory: creating channel-based Bus (single-node mode)")
	return New()
}

package store

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// metadataDBQueryDuration tracks query latency for Bytebase's metadata database.
	metadataDBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bytebase_metadata_db_query_duration_seconds",
			Help:    "Duration of queries to Bytebase's internal metadata database",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
		},
		[]string{"operation", "status"},
	)
)

func (s *Store) RegisterPoolMetrics() {
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bytebase_db_pool_open_connections",
			Help: "Number of open connections to metadata DB",
		}, func() float64 { return float64(s.GetDB().Stats().OpenConnections) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bytebase_db_pool_in_use",
			Help: "Number of connections currently in use",
		}, func() float64 { return float64(s.GetDB().Stats().InUse) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bytebase_db_pool_idle",
			Help: "Number of idle connections",
		}, func() float64 { return float64(s.GetDB().Stats().Idle) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "bytebase_db_pool_wait_count",
			Help: "Total number of connections waited for",
		}, func() float64 { return float64(s.GetDB().Stats().WaitCount) }),
	)
}

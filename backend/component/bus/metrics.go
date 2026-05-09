package bus

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// BusMetrics tracks durable bus queue statistics.
type BusMetrics struct {
	pendingGauge    *prometheus.GaugeVec
	processingGauge *prometheus.GaugeVec
	failedGauge     *prometheus.GaugeVec
	publishedTotal  *prometheus.CounterVec
	consumedTotal   *prometheus.CounterVec
	db              *sql.DB
}

// NewBusMetrics creates and registers Prometheus metrics for the durable bus.
func NewBusMetrics(registerer prometheus.Registerer, db *sql.DB) *BusMetrics {
	m := &BusMetrics{
		db: db,
		pendingGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "bus",
			Name:      "pending_messages",
			Help:      "Number of pending messages in the bus queue.",
		}, []string{"channel"}),
		processingGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "bus",
			Name:      "processing_messages",
			Help:      "Number of messages currently being processed.",
		}, []string{"channel"}),
		failedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bytebase",
			Subsystem: "bus",
			Name:      "failed_messages",
			Help:      "Number of failed messages.",
		}, []string{"channel"}),
		publishedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bytebase",
			Subsystem: "bus",
			Name:      "published_total",
			Help:      "Total messages published.",
		}, []string{"channel"}),
		consumedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bytebase",
			Subsystem: "bus",
			Name:      "consumed_total",
			Help:      "Total messages consumed.",
		}, []string{"channel"}),
	}
	if registerer != nil {
		registerer.MustRegister(m.pendingGauge, m.processingGauge, m.failedGauge, m.publishedTotal, m.consumedTotal)
	}
	return m
}

// RecordPublish increments the publish counter.
func (m *BusMetrics) RecordPublish(channel string) {
	m.publishedTotal.WithLabelValues(channel).Inc()
}

// RecordConsume increments the consume counter.
func (m *BusMetrics) RecordConsume(channel string) {
	m.consumedTotal.WithLabelValues(channel).Inc()
}

// RunCollector starts a background goroutine that periodically queries queue stats.
func (m *BusMetrics) RunCollector(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collect(ctx)
			}
		}
	}()
	slog.Info("Bus metrics collector started", "interval", interval)
}

func (m *BusMetrics) collect(ctx context.Context) {
	if m.db == nil {
		return
	}
	rows, err := m.db.QueryContext(ctx,
		`SELECT channel, status, COUNT(*)
		 FROM bus_queue
		 GROUP BY channel, status`)
	if err != nil {
		slog.Debug("Bus metrics collection failed", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var channel, status string
		var count int64
		if err := rows.Scan(&channel, &status, &count); err != nil {
			continue
		}
		switch status {
		case "pending":
			m.pendingGauge.WithLabelValues(channel).Set(float64(count))
		case "processing":
			m.processingGauge.WithLabelValues(channel).Set(float64(count))
		case "failed":
			m.failedGauge.WithLabelValues(channel).Set(float64(count))
		}
	}
}

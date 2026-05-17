// Package metrics provides Prometheus metrics for Bytebase services.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ServiceMetrics holds per-service Prometheus metrics.
type ServiceMetrics struct {
	RequestTotal    *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ActiveRequests  *prometheus.GaugeVec
	ErrorTotal      *prometheus.CounterVec
	NATSPublished   *prometheus.CounterVec
	NATSProcessed   *prometheus.CounterVec
	CircuitState    *prometheus.GaugeVec
	PanicTotal      *prometheus.CounterVec
}

// NewServiceMetrics creates and registers all service-level metrics.
func NewServiceMetrics(registry prometheus.Registerer) *ServiceMetrics {
	factory := promauto.With(registry)

	return &ServiceMetrics{
		RequestTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bytebase_request_total",
			Help: "Total number of requests processed by service",
		}, []string{"service", "method", "status"}),

		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bytebase_request_duration_seconds",
			Help:    "Duration of requests in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"service", "method"}),

		ActiveRequests: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bytebase_active_requests",
			Help: "Number of currently active requests",
		}, []string{"service"}),

		ErrorTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bytebase_error_total",
			Help: "Total number of errors by service and error code",
		}, []string{"service", "error_code"}),

		NATSPublished: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bytebase_nats_published_total",
			Help: "Total NATS messages published",
		}, []string{"subject"}),

		NATSProcessed: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bytebase_nats_processed_total",
			Help: "Total NATS messages processed by subscribers",
		}, []string{"subject"}),

		CircuitState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bytebase_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		}, []string{"service"}),

		PanicTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bytebase_panic_total",
			Help: "Total panics recovered",
		}, []string{"service"}),
	}
}

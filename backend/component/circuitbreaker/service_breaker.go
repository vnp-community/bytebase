// Package circuitbreaker provides per-service circuit breaker logic.
// This file extends the existing circuitbreaker package with service-aware functionality.
package circuitbreaker

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bytebase/bytebase/backend/component/metrics"
	"github.com/sony/gobreaker/v2"
)

// ServiceCircuitBreakers holds per-service circuit breakers.
type ServiceCircuitBreakers struct {
	breakers map[string]*gobreaker.CircuitBreaker[any]
	metrics  *metrics.ServiceMetrics
}

// Settings configures the circuit breaker behavior.
type Settings struct {
	MaxRequests uint32        // Half-open: max concurrent test requests
	Interval    time.Duration // Counter reset interval when closed
	Timeout     time.Duration // Duration to stay open before half-open
	FailureRate float64       // Failure ratio threshold to trip (0.0-1.0)
}

// DefaultSettings returns conservative default settings.
func DefaultSettings() Settings {
	return Settings{
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		FailureRate: 0.5,
	}
}

// NewServiceCircuitBreakers creates circuit breakers for the given service names.
func NewServiceCircuitBreakers(serviceNames []string, cfg Settings, m *metrics.ServiceMetrics) *ServiceCircuitBreakers {
	scb := &ServiceCircuitBreakers{
		breakers: make(map[string]*gobreaker.CircuitBreaker[any]),
		metrics:  m,
	}
	for _, name := range serviceNames {
		scb.breakers[name] = gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
			Name:        name,
			MaxRequests: cfg.MaxRequests,
			Interval:    cfg.Interval,
			Timeout:     cfg.Timeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				if counts.Requests < 10 {
					return false
				}
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return failureRatio >= cfg.FailureRate
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				slog.Warn("circuit breaker state change",
					"service", name,
					"from", from.String(),
					"to", to.String(),
				)
				if m != nil {
					stateVal := float64(0)
					switch to {
					case gobreaker.StateHalfOpen:
						stateVal = 1
					case gobreaker.StateOpen:
						stateVal = 2
					}
					m.CircuitState.WithLabelValues(name).Set(stateVal)
				}
			},
		})
	}
	return scb
}

// Execute runs a function through the named circuit breaker.
func (scb *ServiceCircuitBreakers) Execute(serviceName string, fn func() (any, error)) (any, error) {
	cb, ok := scb.breakers[serviceName]
	if !ok {
		return nil, fmt.Errorf("no circuit breaker for service %q", serviceName)
	}
	return cb.Execute(fn)
}

// State returns the current state of a service's circuit breaker.
func (scb *ServiceCircuitBreakers) State(serviceName string) gobreaker.State {
	cb, ok := scb.breakers[serviceName]
	if !ok {
		return gobreaker.StateClosed
	}
	return cb.State()
}

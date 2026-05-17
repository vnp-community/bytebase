package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// State represents the current state of the circuit breaker.
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// ErrCircuitOpen is returned when the circuit is open and rejects requests.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Config holds the configuration for a CircuitBreaker.
type Config struct {
	Name             string
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
}

// CircuitBreaker is a state machine that prevents cascading failures.
type CircuitBreaker struct {
	config Config

	mu               sync.RWMutex
	state            State
	failures         int
	successes        int
	lastFailureTime  time.Time

	// Metrics
	stateGauge      prometheus.Gauge
	failuresCounter prometheus.Counter
	rejectedCounter prometheus.Counter
}

// New creates a new CircuitBreaker with the given config.
func New(cfg Config, registry prometheus.Registerer) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	cb := &CircuitBreaker{
		config: cfg,
		state:  StateClosed,
	}

	if registry != nil {
		cb.stateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "bytebase_circuit_breaker_state",
			Help:        "Current state of the circuit breaker (0=Closed, 1=HalfOpen, 2=Open)",
			ConstLabels: prometheus.Labels{"name": cfg.Name},
		})
		cb.failuresCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "bytebase_circuit_breaker_failures_total",
			Help:        "Total number of failed executions",
			ConstLabels: prometheus.Labels{"name": cfg.Name},
		})
		cb.rejectedCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "bytebase_circuit_breaker_rejected_total",
			Help:        "Total number of rejected requests (circuit open)",
			ConstLabels: prometheus.Labels{"name": cfg.Name},
		})
		registry.MustRegister(cb.stateGauge, cb.failuresCounter, cb.rejectedCounter)
		cb.updateMetrics()
	}

	return cb
}

func (b *CircuitBreaker) updateMetrics() {
	if b.stateGauge == nil {
		return
	}
	var val float64
	switch b.state {
	case StateClosed:
		val = 0
	case StateHalfOpen:
		val = 1
	case StateOpen:
		val = 2
	}
	b.stateGauge.Set(val)
}

func (b *CircuitBreaker) currentState() State {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.state == StateOpen {
		if time.Since(b.lastFailureTime) >= b.config.Timeout {
			return StateHalfOpen
		}
	}
	return b.state
}

// Execute runs the given function if the circuit is closed or half-open.
func (b *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	state := b.currentState()

	if state == StateOpen {
		if b.rejectedCounter != nil {
			b.rejectedCounter.Inc()
		}
		return ErrCircuitOpen
	}

	// Change state to HalfOpen if timeout passed
	if state == StateHalfOpen {
		b.mu.Lock()
		if b.state == StateOpen {
			b.state = StateHalfOpen
			b.updateMetrics()
		}
		b.mu.Unlock()
	}

	err := fn(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		if b.failuresCounter != nil {
			b.failuresCounter.Inc()
		}
		b.failures++
		b.successes = 0
		b.lastFailureTime = time.Now()

		if b.state == StateHalfOpen || b.failures >= b.config.FailureThreshold {
			b.state = StateOpen
		}
		b.updateMetrics()
		return err
	}

	if b.state == StateHalfOpen {
		b.successes++
		if b.successes >= b.config.SuccessThreshold {
			b.state = StateClosed
			b.failures = 0
			b.successes = 0
		}
	} else if b.state == StateClosed {
		b.failures = 0
		b.successes = 0
	}
	b.updateMetrics()

	return nil
}

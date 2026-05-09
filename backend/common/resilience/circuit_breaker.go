// Package resilience provides production-grade resilience patterns:
// circuit breaker, bulkhead (concurrency limiter), and structured retry.
package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	// StateClosed — normal operation, requests pass through.
	StateClosed State = iota
	// StateOpen — tripped, requests fail immediately (fast-fail).
	StateOpen
	// StateHalfOpen — recovery, allows one probe request.
	StateHalfOpen
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures a circuit breaker instance.
type CircuitBreakerConfig struct {
	// Name identifies this circuit breaker (used in logging and metrics).
	Name string
	// MaxFailures is the number of consecutive failures before circuit opens. Default: 5.
	MaxFailures int
	// ResetTimeout is the duration in Open state before transitioning to HalfOpen. Default: 30s.
	ResetTimeout time.Duration
}

// CircuitBreaker prevents cascading failures by stopping requests
// to a failing dependency after threshold consecutive failures.
//
// State transitions:
//
//	Closed → Open:     after MaxFailures consecutive failures
//	Open → HalfOpen:   after ResetTimeout
//	HalfOpen → Closed: if probe request succeeds
//	HalfOpen → Open:   if probe request fails
type CircuitBreaker struct {
	mu           sync.Mutex
	name         string
	state        State
	failures     int
	successes    int
	maxFailures  int
	resetTimeout time.Duration
	lastFailure  time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given configuration.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 30 * time.Second
	}

	return &CircuitBreaker{
		name:            cfg.Name,
		state:           StateClosed,
		maxFailures:     cfg.MaxFailures,
		resetTimeout:    cfg.ResetTimeout,
		lastStateChange: time.Now(),
	}
}

// ErrCircuitOpen is returned when the circuit breaker is in Open state.
type ErrCircuitOpen struct {
	Name string
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("circuit breaker [%s] is open", e.Name)
}

// Execute runs fn through the circuit breaker.
//
// If the circuit is Open, returns ErrCircuitOpen immediately.
// If HalfOpen, allows one probe request — success closes, failure re-opens.
// If Closed, counts consecutive failures and opens after MaxFailures.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cb.mu.Lock()
	state := cb.currentStateLocked()
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		return &ErrCircuitOpen{Name: cb.name}

	case StateHalfOpen:
		err := fn(ctx)
		cb.mu.Lock()
		defer cb.mu.Unlock()
		if err != nil {
			cb.toOpenLocked()
			return err
		}
		cb.toClosedLocked()
		return nil

	default: // StateClosed
		err := fn(ctx)
		cb.mu.Lock()
		defer cb.mu.Unlock()
		if err != nil {
			cb.failures++
			cb.lastFailure = time.Now()
			if cb.failures >= cb.maxFailures {
				cb.toOpenLocked()
			}
			return err
		}
		cb.failures = 0
		cb.successes++
		return nil
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentStateLocked()
}

// currentStateLocked evaluates timeouts and returns the current state.
// Caller must hold cb.mu.
func (cb *CircuitBreaker) currentStateLocked() State {
	if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
		cb.state = StateHalfOpen
		cb.lastStateChange = time.Now()
		slog.Info("Circuit breaker entering half-open",
			"name", cb.name,
			"after", cb.resetTimeout,
		)
	}
	return cb.state
}

func (cb *CircuitBreaker) toOpenLocked() {
	cb.state = StateOpen
	cb.lastStateChange = time.Now()
	slog.Warn("Circuit breaker opened",
		"name", cb.name,
		"failures", cb.failures,
	)
}

func (cb *CircuitBreaker) toClosedLocked() {
	cb.state = StateClosed
	cb.failures = 0
	cb.lastStateChange = time.Now()
	slog.Info("Circuit breaker closed (recovered)",
		"name", cb.name,
	)
}

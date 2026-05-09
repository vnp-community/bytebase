package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- CircuitBreaker Tests ----

func TestCircuitBreaker_NormalFlow(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 3,
	})

	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestCircuitBreaker_TripsAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "trip-test",
		MaxFailures:  3,
		ResetTimeout: 1 * time.Hour, // won't reset during test
	})

	testErr := errors.New("fail")
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after %d failures, got %v", 3, cb.State())
	}

	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	var circuitErr *ErrCircuitOpen
	if !errors.As(err, &circuitErr) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "recovery-test",
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
	})

	testErr := errors.New("fail")
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return testErr
		})
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Wait for reset
	time.Sleep(100 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected StateHalfOpen, got %v", cb.State())
	}

	// Successful probe → closes
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreaker_ContextCancelled(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Name: "ctx-test"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ---- Bulkhead Tests ----

func TestBulkhead_LimitsConcurrency(t *testing.T) {
	b := NewBulkhead("test", 3)
	var maxActive atomic.Int64
	var currentActive atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Execute(context.Background(), func(ctx context.Context) error {
				cur := currentActive.Add(1)
				for {
					oldMax := maxActive.Load()
					if cur <= oldMax || maxActive.CompareAndSwap(oldMax, cur) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				currentActive.Add(-1)
				return nil
			})
		}()
	}
	wg.Wait()

	if maxActive.Load() > 3 {
		t.Errorf("expected max active <= 3, got %d", maxActive.Load())
	}
}

func TestBulkhead_ContextCancellation(t *testing.T) {
	b := NewBulkhead("cancel-test", 1)

	// Fill the single slot
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := make(chan struct{})
	go func() {
		_ = b.Execute(context.Background(), func(ctx context.Context) error {
			close(started)
			time.Sleep(200 * time.Millisecond)
			return nil
		})
	}()
	<-started
	time.Sleep(5 * time.Millisecond)

	err := b.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Error("expected timeout error")
	}
}

// ---- Retry Tests ----

func TestRetry_SucceedsFirst(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), "test", RetryConfig{MaxRetries: 3, InitialDelay: 1 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), "retry-test", RetryConfig{MaxRetries: 3, InitialDelay: 1 * time.Millisecond, Jitter: false}, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_ExhaustsRetries(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), "exhaust-test", RetryConfig{MaxRetries: 2, InitialDelay: 1 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		return errors.New("always fail")
	})
	if err == nil {
		t.Error("expected error after exhausted retries")
	}
	if calls != 3 { // initial + 2 retries
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, "ctx-test", RetryConfig{MaxRetries: 5, InitialDelay: 100 * time.Millisecond}, func(ctx context.Context) error {
		return errors.New("fail")
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ---- RateLimiter Tests ----

func TestRateLimiter_AllowsBurst(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	for i := 0; i < 5; i++ {
		if !rl.Allow("key1") {
			t.Errorf("expected allow at burst %d", i)
		}
	}
	// 6th request should be denied (burst exhausted)
	if rl.Allow("key1") {
		t.Error("expected deny after burst exhausted")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(10, 2)

	rl.Allow("key1")
	rl.Allow("key1")
	// key1 exhausted, but key2 should still work
	if !rl.Allow("key2") {
		t.Error("expected allow for different key")
	}
}

package resilience

import (
	"context"
	"fmt"
	"sync/atomic"
)

// Bulkhead limits concurrency for resource-intensive operations using a semaphore.
// It prevents a single type of workload from consuming all available resources.
type Bulkhead struct {
	name          string
	sem           chan struct{}
	maxConcurrent int
	active        atomic.Int64
}

// NewBulkhead creates a concurrency limiter.
// maxConcurrent must be >= 1.
func NewBulkhead(name string, maxConcurrent int) *Bulkhead {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Bulkhead{
		name:          name,
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// ErrBulkheadTimeout is returned when the context is cancelled while waiting for a permit.
type ErrBulkheadTimeout struct {
	Name string
	Err  error
}

func (e *ErrBulkheadTimeout) Error() string {
	return fmt.Sprintf("bulkhead [%s]: %v", e.Name, e.Err)
}

func (e *ErrBulkheadTimeout) Unwrap() error {
	return e.Err
}

// Execute runs fn within the concurrency limit.
// Blocks if at capacity. Respects context cancellation.
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return &ErrBulkheadTimeout{Name: b.name, Err: err}
	}

	select {
	case b.sem <- struct{}{}:
		// Acquired permit
		b.active.Add(1)
		defer func() {
			b.active.Add(-1)
			<-b.sem
		}()
		return fn(ctx)
	case <-ctx.Done():
		return &ErrBulkheadTimeout{Name: b.name, Err: ctx.Err()}
	}
}

// Active returns the number of currently executing operations.
func (b *Bulkhead) Active() int64 {
	return b.active.Load()
}

// MaxConcurrent returns the maximum concurrency limit.
func (b *Bulkhead) MaxConcurrent() int {
	return b.maxConcurrent
}

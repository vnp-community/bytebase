package jobqueue

import (
	"context"
	"log/slog"
	"time"

	"github.com/sourcegraph/conc/pool"
)

// WorkerConfig configures the worker pool behavior.
type WorkerConfig struct {
	// JobTypes filters which job types this worker pool processes.
	JobTypes []JobType
	// MaxWorkers is the maximum concurrent job handlers.
	MaxWorkers int
	// PollInterval is the time between dequeue attempts when the queue is empty.
	PollInterval time.Duration
}

// Worker is a concurrent job processor that dequeues jobs from the queue
// and dispatches them to a handler function using a bounded goroutine pool.
type Worker struct {
	queue   *Queue
	config  WorkerConfig
	handler func(ctx context.Context, job *Job) error
}

// NewWorker creates a new worker pool for the given queue and configuration.
func NewWorker(queue *Queue, config WorkerConfig,
	handler func(ctx context.Context, job *Job) error) *Worker {
	return &Worker{queue: queue, config: config, handler: handler}
}

// Run starts the worker loop. It blocks until the context is cancelled.
// Jobs are processed concurrently up to MaxWorkers. When the queue is empty,
// it waits PollInterval before retrying.
func (w *Worker) Run(ctx context.Context) {
	p := pool.New().WithMaxGoroutines(w.config.MaxWorkers)
	for {
		select {
		case <-ctx.Done():
			p.Wait()
			return
		default:
		}

		job, err := w.queue.Dequeue(ctx, w.config.JobTypes)
		if err != nil {
			slog.Error("dequeue failed", "error", err)
			time.Sleep(w.config.PollInterval)
			continue
		}
		if job == nil {
			time.Sleep(w.config.PollInterval)
			continue
		}

		p.Go(func() {
			if err := w.handler(ctx, job); err != nil {
				if fErr := w.queue.Fail(ctx, job.ID, err.Error()); fErr != nil {
					slog.Error("failed to mark job as failed",
						"job_id", job.ID, "error", fErr)
				}
			} else {
				if cErr := w.queue.Complete(ctx, job.ID); cErr != nil {
					slog.Error("failed to mark job as complete",
						"job_id", job.ID, "error", cErr)
				}
			}
		})
	}
}

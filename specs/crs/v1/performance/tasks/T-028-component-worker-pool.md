# T-028: Component — Worker Pool

| Field | Value |
|-------|-------|
| **Task ID** | T-028 |
| **Solution** | SOL-PERF-006 |
| **Type** | New file |
| **Priority** | P1 |
| **Depends on** | T-027 |
| **Blocks** | None |

## Target File

`backend/component/jobqueue/worker.go` (new)

## Implementation

```go
package jobqueue

import (
    "context"
    "log/slog"
    "time"
    "github.com/sourcegraph/conc/pool"
)

type WorkerConfig struct {
    JobTypes     []JobType
    MaxWorkers   int
    PollInterval time.Duration
}

type Worker struct {
    queue   *Queue
    config  WorkerConfig
    handler func(ctx context.Context, job *Job) error
}

func NewWorker(queue *Queue, config WorkerConfig,
    handler func(ctx context.Context, job *Job) error) *Worker {
    return &Worker{queue: queue, config: config, handler: handler}
}

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
                w.queue.Fail(ctx, job.ID, err.Error())
            } else {
                w.queue.Complete(ctx, job.ID)
            }
        })
    }
}
```

## Usage Example

```go
worker := jobqueue.NewWorker(queue, jobqueue.WorkerConfig{
    JobTypes:     []jobqueue.JobType{jobqueue.JobTypePlanCheck},
    MaxWorkers:   100,
    PollInterval: 1 * time.Second,
}, handler)
go worker.Run(ctx)
```

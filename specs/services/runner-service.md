# Runner Service Specification (NATS Subscribers)

## 1. Overview

Runner Service quản lý background goroutines. **Key change**: runners nhận events từ **NATS** thay vì Go channels, thông qua `NATSBus` adapter implement `EventBus` interface.

## 2. Architecture

```
┌───────────────────────────────────────────────────────────────┐
│  RUNNER SERVICE                                                │
│                                                                 │
│  EventBus (NATSBus) ─── NATS Subjects ──→ Runner Goroutines   │
│                                                                 │
│  bytebase.plancheck.tickle  ──→  PlanCheck Scheduler           │
│  bytebase.taskrun.tickle    ──→  TaskRun Scheduler             │
│  bytebase.approval.check    ──→  Approval Runner               │
│  bytebase.rollout.create    ──→  Rollout Creator               │
│  bytebase.plan.completion   ──→  Webhook Manager               │
│                                                                 │
│  Always-On (timer-based, no NATS):                             │
│  • SchemaSync, DataCleaner, Heartbeat, MemoryMonitor           │
│  • PoolMonitor, LeaderRunner, SelfhealRunner                   │
│                                                                 │
│  Conditional: BackupScheduler, ReplicationMonitor              │
└───────────────────────────────────────────────────────────────┘
```

## 3. Key Design: Zero Runner Code Changes

```go
// Runners consume from EventBus interface — UNCHANGED
// The EventBus is now NATSBus instead of Bus, but interface is identical

// Example: PlanCheck Scheduler (UNCHANGED)
func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.bus.PlanCheckChan():   // ← This now reads from NATSBus
            s.processChecks(ctx)
        }
    }
}
```

`NATSBus.PlanCheckChan()` returns a Go channel that is populated by a NATS subscriber internally. The runner code **does not know** it's using NATS.

## 4. Runner Service Struct

```go
package runner

type Service struct {
    // Same runners as before
    taskScheduler      *taskrun.Scheduler
    planCheckScheduler *plancheck.Scheduler
    schemaSyncer       *schemasync.Syncer
    approvalRunner     *approval.Runner
    dataCleaner        *cleaner.DataCleaner
    heartbeatRunner    *heartbeat.Runner
    // ...

    wg sync.WaitGroup
}

// NewService — SAME constructors, but bus is now NATSBus
func NewService(deps *RunnerDeps) *Service {
    // deps.Bus is NATSBus implementing EventBus interface
    // All constructors unchanged
}

func (s *Service) Run(ctx context.Context) { /* same as before */ }
func (s *Service) Wait()                   { s.wg.Wait() }
```

## 5. Runner packages — ZERO changes

| Package | Change? | Reason |
|---------|---------|--------|
| `backend/runner/taskrun/` | ❌ No | Consumes EventBus interface |
| `backend/runner/plancheck/` | ❌ No | Consumes EventBus interface |
| `backend/runner/approval/` | ❌ No | Consumes EventBus interface |
| `backend/runner/schemasync/` | ❌ No | Timer-based, no bus |
| `backend/runner/cleaner/` | ❌ No | Timer-based |
| `backend/runner/heartbeat/` | ❌ No | Timer-based |
| All other runners | ❌ No | Same interfaces |

## 6. Production-Grade Runner Monitoring

### 6.1 Runner Health Reporting

```go
type RunnerHealth struct {
    Name       string        `json:"name"`
    Alive      bool          `json:"alive"`
    LastRun    time.Time     `json:"last_run"`
    RunCount   int64         `json:"run_count"`
    ErrorCount int64         `json:"error_count"`
    AvgLatency time.Duration `json:"avg_latency"`
}

// RunnerService exposes /internal/runners/health
func (s *Service) HealthCheck() []RunnerHealth {
    return []RunnerHealth{
        {Name: "plancheck", Alive: s.planCheckScheduler.IsAlive(), ...},
        {Name: "taskrun", Alive: s.taskScheduler.IsAlive(), ...},
        // ...
    }
}
```

### 6.2 Runner Error Recovery

```go
// Each runner goroutine wraps with panic recovery + restart
func (s *Service) runWithRecovery(ctx context.Context, name string, fn func(context.Context)) {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        for {
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        slog.Error("runner panic recovered",
                            "runner", name, "error", r,
                            "stack", string(debug.Stack()))
                        metrics.PanicTotal.WithLabelValues("runner." + name).Inc()
                    }
                }()
                fn(ctx)
            }()
            // If context cancelled, don't restart
            if ctx.Err() != nil { return }
            // Backoff before restart
            slog.Warn("runner restarting after crash", "runner", name)
            time.Sleep(5 * time.Second)
        }
    }()
}
```

### 6.3 NATS Delivery Guarantees

```go
// NATSBus monitors subscription health
func (b *NATSBus) MonitorSubscriptions() {
    // Track: pending messages, dropped messages, slow consumers
    // Alert if pending > threshold (backpressure)
}
```


# Solution: Durable Message Bus — CR-ARCH-002

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-002                                             |
| **CR Reference**   | CR-ARCH-002                                              |
| **Title**          | PG-Backed Queue with Channel Bridge + Observability      |
| **Affected Layers**| L5 (Component — Bus), L8 (Store), L6 (Runner)            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §5 (L5 — Component Layer):
- `Bus` struct uses buffered Go channels for inter-runner coordination
- 5 channels: ApprovalCheck(1000), PlanCheckTickle(1000), TaskRunTickle(1000), RolloutCreation(100), PlanCompletionCheck(1000)

Per [TDD.md](../../TDD.md) §5.1:
> "Buffered Go channels thay vì external message queue — đơn giản, low-latency, phù hợp monolith. Trade-off: messages mất khi server crash."

---

## 2. Current Implementation Analysis

### 2.1 Bus Struct (bus.go:33-54)

```go
type Bus struct {
    ApprovalCheckChan       chan IssueRef      // buffer: 1000
    RunningTaskRunsCancelFunc      sync.Map
    RunningPlanCheckRunsCancelFunc sync.Map
    PlanCheckTickleChan     chan int            // buffer: 1000
    TaskRunTickleChan       chan int            // buffer: 1000
    RolloutCreationChan     chan PlanRef        // buffer: 100
    PlanCompletionCheckChan chan PlanRef        // buffer: 1000
}
```

**Problems**: zero durability, zero backpressure, zero observability, HA-unsafe.

---

## 3. Solution Design

### 3.1 Phase 1 — PG Queue Table (Migration)

**New migration file**: `backend/migrator/migration/prod/NEXT_bus_queue.sql`

```sql
-- Bus queue table for durable message passing
CREATE TABLE bus_queue (
    id          BIGSERIAL PRIMARY KEY,
    channel     TEXT NOT NULL,
    payload     JSONB NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    priority    INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_by  TEXT,
    claimed_at  TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts    INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    error_msg   TEXT
);

-- Index for consumer polling: get pending messages per channel
CREATE INDEX idx_bus_queue_pending ON bus_queue (channel, priority DESC, id ASC)
    WHERE status = 'pending';

-- Index for stale claim detection
CREATE INDEX idx_bus_queue_processing ON bus_queue (claimed_at)
    WHERE status = 'processing';

-- Index for cleanup
CREATE INDEX idx_bus_queue_completed ON bus_queue (completed_at)
    WHERE status IN ('done', 'failed');

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION bus_queue_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bus_queue_updated_at
    BEFORE UPDATE ON bus_queue
    FOR EACH ROW
    EXECUTE FUNCTION bus_queue_updated_at();
```

### 3.2 Phase 2 — Durable Bus Implementation

**New file**: `backend/component/bus/durable_bus.go`

```go
package bus

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

// DurableBus implements persistent message queue backed by PostgreSQL.
// It maintains backward-compatible channel interfaces for existing consumers.
type DurableBus struct {
    db         *sql.DB
    instanceID string

    // Backward-compatible channels — consumers read from these
    ApprovalCheckChan       chan IssueRef
    PlanCheckTickleChan     chan int
    TaskRunTickleChan       chan int
    RolloutCreationChan     chan PlanRef
    PlanCompletionCheckChan chan PlanRef

    // Cancel functions (unchanged)
    RunningTaskRunsCancelFunc      sync.Map
    RunningPlanCheckRunsCancelFunc sync.Map

    // Metrics
    metrics *busMetrics

    // Config
    pollInterval time.Duration
    claimTimeout time.Duration
}

// NewDurable creates a persistent bus with PG-backed queue.
func NewDurable(db *sql.DB, instanceID string) (*DurableBus, error) {
    b := &DurableBus{
        db:         db,
        instanceID: instanceID,
        // Channels buffered for consumer goroutine bridge
        ApprovalCheckChan:       make(chan IssueRef, 100),
        PlanCheckTickleChan:     make(chan int, 100),
        TaskRunTickleChan:       make(chan int, 100),
        RolloutCreationChan:     make(chan PlanRef, 100),
        PlanCompletionCheckChan: make(chan PlanRef, 100),
        // Config
        pollInterval: 100 * time.Millisecond,
        claimTimeout: 5 * time.Minute,
        metrics:      newBusMetrics(),
    }
    return b, nil
}

// Enqueue writes a message to the persistent queue.
func (b *DurableBus) Enqueue(ctx context.Context, channel string, payload any) error {
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal payload: %w", err)
    }

    _, err = b.db.ExecContext(ctx,
        `INSERT INTO bus_queue (channel, payload) VALUES ($1, $2)`,
        channel, data,
    )
    if err != nil {
        b.metrics.enqueueFailed.WithLabelValues(channel).Inc()
        return fmt.Errorf("enqueue to %s: %w", channel, err)
    }

    b.metrics.enqueueTotal.WithLabelValues(channel).Inc()

    // Trigger PG NOTIFY for immediate consumer wakeup
    _, _ = b.db.ExecContext(ctx,
        fmt.Sprintf("NOTIFY bus_%s", channel),
    )
    return nil
}

// StartConsumers starts background goroutines that poll PG queue
// and push messages to backward-compatible channels.
func (b *DurableBus) StartConsumers(ctx context.Context, wg *sync.WaitGroup) {
    consumers := []struct {
        channel string
        handler func(ctx context.Context, payload []byte) error
    }{
        {"approval_check", b.handleApprovalCheck},
        {"plan_check_tickle", b.handlePlanCheckTickle},
        {"task_run_tickle", b.handleTaskRunTickle},
        {"rollout_creation", b.handleRolloutCreation},
        {"plan_completion_check", b.handlePlanCompletionCheck},
    }

    for _, c := range consumers {
        wg.Add(1)
        go func(ch string, handler func(ctx context.Context, payload []byte) error) {
            defer wg.Done()
            b.consumeLoop(ctx, ch, handler)
        }(c.channel, c.handler)
    }

    // Stale claim recovery goroutine
    wg.Add(1)
    go func() {
        defer wg.Done()
        b.recoverStaleClaims(ctx)
    }()
}

// consumeLoop polls PG queue and dispatches messages.
func (b *DurableBus) consumeLoop(ctx context.Context, channel string, handler func(context.Context, []byte) error) {
    ticker := time.NewTicker(b.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            b.processMessages(ctx, channel, handler)
        }
    }
}

// processMessages claims and processes pending messages.
// Uses SELECT FOR UPDATE SKIP LOCKED for HA-safe consumption.
func (b *DurableBus) processMessages(ctx context.Context, channel string, handler func(context.Context, []byte) error) {
    tx, err := b.db.BeginTx(ctx, nil)
    if err != nil {
        return
    }
    defer tx.Rollback()

    // Claim up to 10 messages at once (batched for efficiency)
    rows, err := tx.QueryContext(ctx, `
        SELECT id, payload FROM bus_queue
        WHERE channel = $1 AND status = 'pending'
        ORDER BY priority DESC, id ASC
        LIMIT 10
        FOR UPDATE SKIP LOCKED
    `, channel)
    if err != nil {
        return
    }
    defer rows.Close()

    var ids []int64
    for rows.Next() {
        var id int64
        var payload []byte
        if err := rows.Scan(&id, &payload); err != nil {
            continue
        }

        start := time.Now()
        if err := handler(ctx, payload); err != nil {
            // Mark failed, increment attempts
            tx.ExecContext(ctx, `
                UPDATE bus_queue SET status = CASE WHEN attempts + 1 >= max_retries THEN 'failed' ELSE 'pending' END,
                    attempts = attempts + 1, error_msg = $2, claimed_by = NULL
                WHERE id = $1
            `, id, err.Error())
            b.metrics.failedTotal.WithLabelValues(channel).Inc()
        } else {
            ids = append(ids, id)
        }
        b.metrics.dequeueDuration.WithLabelValues(channel).Observe(time.Since(start).Seconds())
    }

    // Mark successful messages as done
    if len(ids) > 0 {
        for _, id := range ids {
            tx.ExecContext(ctx, `
                UPDATE bus_queue SET status = 'done', completed_at = NOW() WHERE id = $1
            `, id)
        }
    }

    tx.Commit()
}

// recoverStaleClaims resets messages stuck in 'processing' > claimTimeout.
func (b *DurableBus) recoverStaleClaims(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            result, err := b.db.ExecContext(ctx, `
                UPDATE bus_queue SET status = 'pending', claimed_by = NULL, claimed_at = NULL
                WHERE status = 'processing' AND claimed_at < NOW() - INTERVAL '5 minutes'
            `)
            if err == nil {
                if n, _ := result.RowsAffected(); n > 0 {
                    slog.Warn("Recovered stale bus messages", "count", n)
                }
            }
        }
    }
}

// Channel bridge handlers — decode payload and push to backward-compatible channels
func (b *DurableBus) handleApprovalCheck(ctx context.Context, payload []byte) error {
    var ref IssueRef
    if err := json.Unmarshal(payload, &ref); err != nil {
        return err
    }
    select {
    case b.ApprovalCheckChan <- ref:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (b *DurableBus) handlePlanCheckTickle(ctx context.Context, payload []byte) error {
    var val int
    if err := json.Unmarshal(payload, &val); err != nil {
        return err
    }
    select {
    case b.PlanCheckTickleChan <- val:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (b *DurableBus) handleTaskRunTickle(ctx context.Context, payload []byte) error {
    var val int
    if err := json.Unmarshal(payload, &val); err != nil {
        return err
    }
    select {
    case b.TaskRunTickleChan <- val:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (b *DurableBus) handleRolloutCreation(ctx context.Context, payload []byte) error {
    var ref PlanRef
    if err := json.Unmarshal(payload, &ref); err != nil {
        return err
    }
    select {
    case b.RolloutCreationChan <- ref:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (b *DurableBus) handlePlanCompletionCheck(ctx context.Context, payload []byte) error {
    var ref PlanRef
    if err := json.Unmarshal(payload, &ref); err != nil {
        return err
    }
    select {
    case b.PlanCompletionCheckChan <- ref:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### 3.3 Phase 2b — Bus Metrics

**New file**: `backend/component/bus/metrics.go`

```go
package bus

import "github.com/prometheus/client_golang/prometheus"

type busMetrics struct {
    enqueueTotal   *prometheus.CounterVec
    enqueueFailed  *prometheus.CounterVec
    failedTotal    *prometheus.CounterVec
    dequeueDuration *prometheus.HistogramVec
    queueDepth     *prometheus.GaugeVec
}

func newBusMetrics() *busMetrics {
    m := &busMetrics{
        enqueueTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_enqueue_total",
            Help: "Total messages enqueued per channel",
        }, []string{"channel"}),
        enqueueFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_enqueue_failed_total",
            Help: "Total failed enqueue attempts",
        }, []string{"channel"}),
        failedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_failed_total",
            Help: "Total messages that exceeded max retries",
        }, []string{"channel"}),
        dequeueDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "bytebase_bus_dequeue_duration_seconds",
            Help:    "Processing time per message",
            Buckets: prometheus.DefBuckets,
        }, []string{"channel"}),
        queueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_bus_queue_depth",
            Help: "Number of pending messages per channel",
        }, []string{"channel"}),
    }
    prometheus.MustRegister(m.enqueueTotal, m.enqueueFailed, m.failedTotal, m.dequeueDuration, m.queueDepth)
    return m
}
```

### 3.4 Phase 3 — Server Integration

**Modified file**: `backend/server/server.go`

```go
func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
    // ...existing init...

    // Bus: durable (PG-backed) or volatile (in-memory)
    if profile.BusPersistent {
        durableBus, err := bus.NewDurable(stores.GetDB(), profile.ReplicaID)
        if err != nil {
            return nil, errors.Wrapf(err, "failed to create durable bus")
        }
        s.bus = &bus.Bus{
            ApprovalCheckChan:       durableBus.ApprovalCheckChan,
            PlanCheckTickleChan:     durableBus.PlanCheckTickleChan,
            TaskRunTickleChan:       durableBus.TaskRunTickleChan,
            RolloutCreationChan:     durableBus.RolloutCreationChan,
            PlanCompletionCheckChan: durableBus.PlanCompletionCheckChan,
        }
        s.durableBus = durableBus  // for starting consumers in Run()
    } else {
        s.bus, err = bus.New()  // existing volatile behavior
    }
}

func (s *Server) Run(ctx context.Context, port int) error {
    // Start durable bus consumers if enabled
    if s.durableBus != nil {
        s.durableBus.StartConsumers(ctx, &s.runnerWG)
    }
    // ...existing runner starts...
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/migrator/migration/prod/NEXT_bus_queue.sql` | L8 | **NEW** | Queue table DDL |
| `backend/component/bus/durable_bus.go` | L5 | **NEW** | PG-backed bus impl |
| `backend/component/bus/metrics.go` | L5 | **NEW** | Prometheus metrics |
| `backend/server/server.go` | L2 | **MODIFY** | Feature flag for durable bus |
| `backend/component/config/profile.go` | L5 | **MODIFY** | `BusPersistent` config |

---

## 5. Migration Strategy

Feature flag `BUS_PERSISTENT_ENABLED`:
1. `false` (default) → existing volatile channels (zero change)
2. `true` → PG-backed queue + channel bridge
3. After validation → make `true` default
4. After stability → remove volatile fallback

---

## 6. Rollback Plan

1. Set `BUS_PERSISTENT_ENABLED=false` → immediate fallback to volatile channels
2. `bus_queue` table can remain (no impact when not used)
3. No runner code changes needed — they still read from channels

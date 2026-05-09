# Solution: CR-LIM-002 — Persistent Message Bus

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-002                               |
| **Solution ID**| SOL-LIM-002                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L5 (Component — Bus), L6 (Runner), L8 (Store) |
| **TDD Refs**   | §5.1 Message Bus Design, §5.2 Task Execution Pipeline, §5.4 PG LISTEN/NOTIFY |

---

## 1. Solution Overview

### 1.1 Approach Summary

**PostgreSQL-native approach** thay vì NATS — tận dụng PG LISTEN/NOTIFY + outbox table đã tồn tại trong codebase để đạt durability mà **không thêm external dependency**.

1. **Phase A — Bus Interface Extraction + Outbox Pattern** (durability via PG)
2. **Phase B — Dead-Letter Queue + Metrics** (observability)
3. **Phase C — Optional NATS Adapter** (cho extreme throughput)

### 1.2 Design Rationale

Từ TDD §5.1, bus hiện dùng Go channels với buffer 100-1000. TDD §5.4 cho thấy `NotifyListener` đã bridge PG NOTIFY → Bus channels. **Key insight**: PG LISTEN/NOTIFY chỉ thiếu persistence — kết hợp với outbox table sẽ có durability.

Từ Architecture L5, Bus struct có 5 channels + 2 sync.Maps. Thay vì thay thế toàn bộ, **wrap existing channels** với persistence layer: ghi message vào outbox table TRƯỚC khi publish lên channel. Runners poll outbox nếu miss channel message.

**Tại sao PG outbox > NATS?**
- Không thêm external dependency → đơn giản vận hành
- Store đã sử dụng PG cho mọi thứ → transactional guarantee (outbox ghi cùng transaction với business data)
- NATS chỉ cần khi throughput > 10K msg/s (Bytebase typically < 100 msg/s)

---

## 2. Detailed Technical Design

### 2.1 Phase A — Bus Interface + PG Outbox

#### 2.1.1 Message Bus Interface

**File**: `backend/component/bus/interface.go` (new)

```go
// MessageBus defines the contract for inter-component messaging.
// Implementations: ChannelBus (in-memory), PGBus (durable), NATSBus (external).
type MessageBus interface {
    // Publish sends a message to a subject. Durable implementations persist
    // the message before returning.
    Publish(ctx context.Context, subject Subject, payload []byte) (MessageID, error)

    // Subscribe registers a handler for a subject. The handler is called
    // for each message. Return nil to ACK, error to NACK (triggers retry).
    Subscribe(subject Subject, handler Handler) error

    // Close gracefully shuts down the bus.
    Close() error
}

type Subject string

const (
    SubjectTaskRunTickle       Subject = "task.run.tickle"
    SubjectPlanCheckTickle     Subject = "plan.check.tickle"
    SubjectApprovalCheck       Subject = "approval.check"
    SubjectRolloutCreation     Subject = "rollout.creation"
    SubjectPlanCompletionCheck Subject = "plan.completion.check"
)

type MessageID string

type Handler func(ctx context.Context, msg *Message) error

type Message struct {
    ID        MessageID
    Subject   Subject
    Payload   []byte
    CreatedAt time.Time
    Attempt   int
}
```

#### 2.1.2 Channel Bus Adapter (Backward Compatible)

**File**: `backend/component/bus/channel_bus.go` (refactored from existing `bus.go`)

```go
// ChannelBus wraps the existing Go channel implementation behind MessageBus.
// Used in single-node mode. No durability guarantee.
type ChannelBus struct {
    channels map[Subject]chan *Message
    handlers map[Subject]Handler
    wg       sync.WaitGroup
    metrics  *busMetrics
}

func NewChannelBus(metrics *busMetrics) *ChannelBus {
    b := &ChannelBus{
        channels: map[Subject]chan *Message{
            SubjectTaskRunTickle:       make(chan *Message, 1000),
            SubjectPlanCheckTickle:     make(chan *Message, 1000),
            SubjectApprovalCheck:       make(chan *Message, 1000),
            SubjectRolloutCreation:     make(chan *Message, 100),
            SubjectPlanCompletionCheck: make(chan *Message, 1000),
        },
        handlers: make(map[Subject]Handler),
        metrics:  metrics,
    }
    return b
}

func (b *ChannelBus) Publish(ctx context.Context, subject Subject, payload []byte) (MessageID, error) {
    ch, ok := b.channels[subject]
    if !ok {
        return "", fmt.Errorf("unknown subject: %s", subject)
    }

    id := MessageID(fmt.Sprintf("%s-%d", subject, time.Now().UnixNano()))
    msg := &Message{ID: id, Subject: subject, Payload: payload, CreatedAt: time.Now()}

    select {
    case ch <- msg:
        b.metrics.published.WithLabelValues(string(subject)).Inc()
        return id, nil
    default:
        b.metrics.dropped.WithLabelValues(string(subject)).Inc()
        return "", fmt.Errorf("channel full for subject: %s", subject)
    }
}

func (b *ChannelBus) Subscribe(subject Subject, handler Handler) error {
    b.handlers[subject] = handler
    b.wg.Add(1)
    go b.consume(subject, handler)
    return nil
}

func (b *ChannelBus) consume(subject Subject, handler Handler) {
    defer b.wg.Done()
    ch := b.channels[subject]
    for msg := range ch {
        if err := handler(context.Background(), msg); err != nil {
            b.metrics.failed.WithLabelValues(string(subject)).Inc()
            slog.Warn("message handler failed", "subject", subject, "err", err)
        } else {
            b.metrics.consumed.WithLabelValues(string(subject)).Inc()
        }
    }
}
```

#### 2.1.3 PG Outbox Bus (Durable — HA Mode)

**File**: `backend/component/bus/pg_bus.go` (new)

```go
// PGBus implements MessageBus using PostgreSQL outbox table + LISTEN/NOTIFY.
// Messages are persisted in bus_outbox table before delivery.
// On crash recovery, unprocessed messages are redelivered.
type PGBus struct {
    db          *sql.DB
    handlers    map[Subject]Handler
    maxRetries  int
    pollTick    time.Duration
    metrics     *busMetrics
    cancelFuncs []context.CancelFunc
}

func NewPGBus(db *sql.DB, maxRetries int, metrics *busMetrics) *PGBus {
    return &PGBus{
        db:         db,
        handlers:   make(map[Subject]Handler),
        maxRetries: maxRetries,
        pollTick:   5 * time.Second,
        metrics:    metrics,
    }
}

func (b *PGBus) Publish(ctx context.Context, subject Subject, payload []byte) (MessageID, error) {
    id := MessageID(uuid.New().String())

    // Step 1: Persist to outbox (DURABLE — survives crash)
    _, err := b.db.ExecContext(ctx, `
        INSERT INTO bus_outbox (id, subject, payload, status, created_at, attempt)
        VALUES ($1, $2, $3, 'PENDING', NOW(), 0)
    `, string(id), string(subject), payload)
    if err != nil {
        return "", fmt.Errorf("outbox insert: %w", err)
    }

    // Step 2: Notify listeners for immediate pickup (low latency)
    _, _ = b.db.ExecContext(ctx, `SELECT pg_notify('bus_message', $1)`, string(id))

    b.metrics.published.WithLabelValues(string(subject)).Inc()
    return id, nil
}

func (b *PGBus) Subscribe(subject Subject, handler Handler) error {
    b.handlers[subject] = handler
    return nil
}

// StartConsumers launches consumer goroutines for all subscribed subjects.
// Call after all Subscribe() calls.
func (b *PGBus) StartConsumers(ctx context.Context) {
    // NOTIFY-driven consumer (low latency)
    go b.listenForNotifications(ctx)

    // Polling consumer (catch-up for missed NOTIFY)
    go b.pollOutbox(ctx)
}

func (b *PGBus) listenForNotifications(ctx context.Context) {
    // Uses pgx native connection for LISTEN (not database/sql)
    conn, err := pgx.Connect(ctx, b.db.(*stdlib.DB).ConnConfig().ConnString())
    if err != nil {
        slog.Error("PGBus LISTEN connect failed", "err", err)
        return
    }
    defer conn.Close(ctx)

    _, _ = conn.Exec(ctx, "LISTEN bus_message")

    for {
        notification, err := conn.WaitForNotification(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return
            }
            time.Sleep(time.Second)
            continue
        }
        b.processMessage(ctx, MessageID(notification.Payload))
    }
}

func (b *PGBus) pollOutbox(ctx context.Context) {
    ticker := time.NewTicker(b.pollTick)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            b.processPendingMessages(ctx)
        }
    }
}

func (b *PGBus) processPendingMessages(ctx context.Context) {
    rows, err := b.db.QueryContext(ctx, `
        SELECT id, subject, payload, attempt FROM bus_outbox
        WHERE status = 'PENDING' AND attempt < $1
        ORDER BY created_at ASC
        LIMIT 100
        FOR UPDATE SKIP LOCKED
    `, b.maxRetries)
    if err != nil {
        return
    }
    defer rows.Close()

    for rows.Next() {
        var id, subject string
        var payload []byte
        var attempt int
        if err := rows.Scan(&id, &subject, &payload, &attempt); err != nil {
            continue
        }
        msg := &Message{
            ID:      MessageID(id),
            Subject: Subject(subject),
            Payload: payload,
            Attempt: attempt,
        }
        b.handleMessage(ctx, msg)
    }
}

func (b *PGBus) processMessage(ctx context.Context, id MessageID) {
    var subject string
    var payload []byte
    var attempt int
    err := b.db.QueryRowContext(ctx, `
        SELECT subject, payload, attempt FROM bus_outbox
        WHERE id = $1 AND status = 'PENDING'
        FOR UPDATE SKIP LOCKED
    `, string(id)).Scan(&subject, &payload, &attempt)
    if err != nil {
        return // Already processed or doesn't exist
    }
    msg := &Message{ID: id, Subject: Subject(subject), Payload: payload, Attempt: attempt}
    b.handleMessage(ctx, msg)
}

func (b *PGBus) handleMessage(ctx context.Context, msg *Message) {
    handler, ok := b.handlers[msg.Subject]
    if !ok {
        return
    }

    err := handler(ctx, msg)
    if err == nil {
        // ACK — mark as DONE
        _, _ = b.db.ExecContext(ctx, `
            UPDATE bus_outbox SET status = 'DONE', processed_at = NOW()
            WHERE id = $1
        `, string(msg.ID))
        b.metrics.consumed.WithLabelValues(string(msg.Subject)).Inc()
    } else {
        // NACK — increment attempt
        newAttempt := msg.Attempt + 1
        if newAttempt >= b.maxRetries {
            // Move to DLQ
            _, _ = b.db.ExecContext(ctx, `
                UPDATE bus_outbox SET status = 'DLQ', attempt = $2, last_error = $3
                WHERE id = $1
            `, string(msg.ID), newAttempt, err.Error())
            b.metrics.dlq.WithLabelValues(string(msg.Subject)).Inc()
        } else {
            _, _ = b.db.ExecContext(ctx, `
                UPDATE bus_outbox SET attempt = $2, last_error = $3
                WHERE id = $1
            `, string(msg.ID), newAttempt, err.Error())
            b.metrics.failed.WithLabelValues(string(msg.Subject)).Inc()
        }
    }
}
```

#### 2.1.4 Outbox Table Migration

**File**: `backend/migrator/migration/<next_version>/0001_bus_outbox.sql`

```sql
-- Persistent message outbox for durable message bus
CREATE TABLE IF NOT EXISTS bus_outbox (
    id           TEXT        PRIMARY KEY,
    subject      TEXT        NOT NULL,
    payload      BYTEA       NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'PENDING',  -- PENDING, DONE, DLQ
    attempt      INT         NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

-- Index for pending message polling (SKIP LOCKED pattern)
CREATE INDEX idx_bus_outbox_pending ON bus_outbox (status, created_at)
    WHERE status = 'PENDING';

-- Index for DLQ inspection
CREATE INDEX idx_bus_outbox_dlq ON bus_outbox (status, created_at)
    WHERE status = 'DLQ';

-- Partial index for cleanup
CREATE INDEX idx_bus_outbox_done ON bus_outbox (processed_at)
    WHERE status = 'DONE';
```

#### 2.1.5 Bus Factory & Server Wiring

**File**: `backend/component/bus/factory.go` (new)

```go
func NewMessageBus(profile *config.Profile, db *sql.DB) MessageBus {
    metrics := newBusMetrics()

    if profile.HA {
        // HA mode: PG outbox for durability
        maxRetries, _ := strconv.Atoi(os.Getenv("BUS_MAX_RETRIES"))
        if maxRetries == 0 {
            maxRetries = 5
        }
        return NewPGBus(db, maxRetries, metrics)
    }

    // Single-node: Go channels (fast, no durability needed)
    return NewChannelBus(metrics)
}
```

**File**: `backend/server/server.go` (modify):

```go
// Replace direct Bus struct with MessageBus interface
func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
    // ... existing setup ...

    // Replace: bus := bus.New()
    messageBus := bus.NewMessageBus(profile, storeInstance.GetDB())

    // Wire into runners (they now consume via MessageBus.Subscribe)
    taskScheduler := taskrun.NewScheduler(messageBus, storeInstance, ...)
    approvalRunner := approval.NewRunner(messageBus, storeInstance, ...)
    // ...
}
```

### 2.2 Phase B — DLQ Management + Cleanup + Metrics

#### 2.2.1 DLQ Admin API

**File**: `backend/api/v1/bus_admin_service.go` (new)

```go
// DLQ Admin endpoints — inspect, replay, purge dead-letter messages
func (s *BusAdminService) ListDLQMessages(ctx context.Context, req *v1pb.ListDLQMessagesRequest) (*v1pb.ListDLQMessagesResponse, error) {
    // Query bus_outbox WHERE status = 'DLQ'
}

func (s *BusAdminService) ReplayDLQMessage(ctx context.Context, req *v1pb.ReplayDLQMessageRequest) (*v1pb.ReplayDLQMessageResponse, error) {
    // Reset status to 'PENDING', attempt to 0
    _, err := s.store.GetDB().ExecContext(ctx, `
        UPDATE bus_outbox SET status = 'PENDING', attempt = 0
        WHERE id = $1 AND status = 'DLQ'
    `, req.MessageId)
    return &v1pb.ReplayDLQMessageResponse{}, err
}

func (s *BusAdminService) PurgeDLQ(ctx context.Context, req *v1pb.PurgeDLQRequest) (*v1pb.PurgeDLQResponse, error) {
    // DELETE FROM bus_outbox WHERE status = 'DLQ' AND created_at < $1
}
```

#### 2.2.2 Outbox Cleanup (via DataCleaner runner — L6)

**File**: `backend/runner/cleaner/data_cleaner.go` (extend)

```go
func (c *DataCleaner) cleanBusOutbox(ctx context.Context) {
    retention := 24 * time.Hour
    if v := os.Getenv("BUS_DONE_RETENTION"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            retention = d
        }
    }

    // Clean DONE messages older than retention
    result, _ := c.store.GetDB().ExecContext(ctx, `
        DELETE FROM bus_outbox
        WHERE status = 'DONE' AND processed_at < $1
    `, time.Now().Add(-retention))

    if n, _ := result.RowsAffected(); n > 0 {
        slog.Info("Cleaned bus outbox", "deleted", n)
    }
}
```

#### 2.2.3 Bus Metrics

**File**: `backend/component/bus/metrics.go` (new)

```go
type busMetrics struct {
    published *prometheus.CounterVec
    consumed  *prometheus.CounterVec
    failed    *prometheus.CounterVec
    dropped   *prometheus.CounterVec
    dlq       *prometheus.CounterVec
    pending   *prometheus.GaugeVec
    duration  *prometheus.HistogramVec
}

func newBusMetrics() *busMetrics {
    return &busMetrics{
        published: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_messages_published_total",
        }, []string{"subject"}),
        consumed: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_messages_consumed_total",
        }, []string{"subject"}),
        failed: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_messages_failed_total",
        }, []string{"subject"}),
        dropped: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_messages_dropped_total",
        }, []string{"subject"}),
        dlq: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_bus_messages_dlq_total",
        }, []string{"subject"}),
        pending: promauto.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_bus_messages_pending",
        }, []string{"subject"}),
        duration: promauto.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "bytebase_bus_processing_duration_seconds",
            Buckets: prometheus.DefBuckets,
        }, []string{"subject"}),
    }
}
```

### 2.3 Runner Adaptation Pattern

Each runner currently reads directly from `bus.TaskRunTickleChan`. The migration pattern:

```go
// BEFORE (runner/taskrun/scheduler.go):
func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case taskRunUID := <-s.bus.TaskRunTickleChan:
            s.handleTaskRun(ctx, taskRunUID)
        }
    }
}

// AFTER:
func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    // Subscribe to MessageBus (works with both Channel and PG bus)
    s.messageBus.Subscribe(bus.SubjectTaskRunTickle, func(ctx context.Context, msg *bus.Message) error {
        var taskRunUID int
        if err := json.Unmarshal(msg.Payload, &taskRunUID); err != nil {
            return err
        }
        return s.handleTaskRun(ctx, taskRunUID)
    })
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L5 (Component — Bus) | **HIGH** | Interface extraction, PG outbox adapter, metrics |
| L6 (Runner) | **MEDIUM** | Adapt 5 runners to MessageBus.Subscribe pattern |
| L8 (Store) | **LOW** | New `bus_outbox` table, cleanup extension |
| L4 (Service) | **LOW** | DLQ admin API (new service) |
| L10 (Infra) | **LOW** | Migration script for outbox table |

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
Phase A (Sprint 1-2):
  1. Create MessageBus interface
  2. Wrap existing Bus as ChannelBus (no behavior change)
  3. Add bus_outbox migration
  4. Implement PGBus
  5. Adapt runners one-by-one (TaskRun first, then others)
  6. Test: single-node uses ChannelBus, HA uses PGBus

Phase B (Sprint 3):
  7. Add metrics instrumentation
  8. Add DLQ admin API
  9. Extend DataCleaner for outbox cleanup
  10. Test: DLQ flow, metrics accuracy
```

### 4.2 Rollback Plan

```
# Rollback to channel-only bus:
# 1. Revert server.go factory to always use ChannelBus
# 2. bus_outbox table remains (no data loss, just unused)
# 3. Runners work identically with ChannelBus
```

---

## 5. Performance Validation

| Metric                    | ChannelBus    | PGBus (HA)        | Notes                    |
|---------------------------|---------------|-------------------|--------------------------|
| Publish latency           | ~1μs          | ~2ms              | PG write + NOTIFY        |
| Consume latency           | ~1μs          | ~5ms              | PG read + FOR UPDATE     |
| Throughput (msg/s)        | ~100K         | ~5K               | PG bound, sufficient     |
| Durability                | None          | Full (PG WAL)     | Survives crash           |
| DLQ support               | None          | Yes               | Automatic after retries  |
| Cross-replica delivery    | None          | Yes (NOTIFY)      | All replicas see events  |

**Bytebase typical throughput**: < 100 msg/s → PGBus capacity (5K msg/s) is 50× headroom.

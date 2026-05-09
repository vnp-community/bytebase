# TASK-LIM-002-A2: PG Outbox Migration + PGBus

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-002 |
| Phase | A — PG Outbox Bus |
| Priority | P0 |
| Depends On | TASK-LIM-002-A1 |
| Est. | L (~350 LoC) |

## Objective

Create `bus_outbox` table and `PGBus` implementing `MessageBus` with durable message delivery via PG outbox + LISTEN/NOTIFY pattern.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/migrator/migration/<next>/0001_bus_outbox.sql` |
| CREATE | `backend/component/bus/pg_bus.go` |
| CREATE | `backend/component/bus/pg_bus_test.go` |

## Specification

### Migration — `bus_outbox` table

```sql
CREATE TABLE bus_outbox (
    id TEXT PRIMARY KEY,
    subject TEXT NOT NULL,
    payload BYTEA NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING, DONE, DLQ
    attempt INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX idx_bus_outbox_pending ON bus_outbox (status, created_at) WHERE status = 'PENDING';
CREATE INDEX idx_bus_outbox_dlq ON bus_outbox (status, created_at) WHERE status = 'DLQ';
```

### `pg_bus.go` — PGBus

Key flow:
- `Publish`: INSERT into outbox → `pg_notify('bus_message', id)` 
- `StartConsumers(ctx)`: two goroutines:
  1. NOTIFY listener: `LISTEN bus_message` → `processMessage(id)` (low latency)
  2. Poll loop: every 5s query PENDING messages `FOR UPDATE SKIP LOCKED` (catch-up)
- `handleMessage`: call handler → ACK (status=DONE) or NACK (increment attempt, DLQ if maxRetries)
- Config: `maxRetries` (default 5), `pollTick` (default 5s)

## Acceptance Criteria

- [ ] Migration creates `bus_outbox` with correct indexes
- [ ] Publish persists message BEFORE returning (durable)
- [ ] NOTIFY triggers immediate consumption (< 50ms latency)
- [ ] Poll loop catches missed NOTIFY events
- [ ] `FOR UPDATE SKIP LOCKED` prevents duplicate processing
- [ ] Messages move to DLQ after maxRetries failures
- [ ] Graceful shutdown on context cancellation

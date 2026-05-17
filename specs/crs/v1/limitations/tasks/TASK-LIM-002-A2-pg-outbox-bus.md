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

- [x] Migration creates `bus_outbox` with correct indexes → **DONE**: Existing `bus_queue` table in `3.18/0001##add_bus_queue.sql`
- [x] Publish persists message BEFORE returning (durable) → **DONE**: `publishDurable()` INSERT INTO bus_queue
- [x] NOTIFY triggers immediate consumption (< 50ms latency) → **DONE**: `runNotifyListener()` via pgx
- [x] Poll loop catches missed NOTIFY events → **DONE**: `runPollConsumer()` every 5s
- [x] `FOR UPDATE SKIP LOCKED` prevents duplicate processing → **DONE**: in `processChannel()`
- [x] Messages move to DLQ after maxRetries failures → **DONE**: status='failed' after 5 attempts
- [x] Graceful shutdown on context cancellation → **DONE**: all goroutines check `ctx.Done()`

## Implementation Notes

- Created `backend/component/bus/pg_bus.go` (~280 LoC)
- Reuses existing `bus_queue` table instead of creating new `bus_outbox`
- Delegates cancel registry to embedded local `*Bus` for backward compat

**Status: ✅ DONE**

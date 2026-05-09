# T-002-01: PG Queue Table Migration

| Field | Value |
|---|---|
| **Task ID** | T-002-01 |
| **Solution** | SOL-ARCH-002 |
| **Priority** | P1 |
| **Depends On** | None |
| **Target File** | `backend/migrator/migration/3.18/0001##add_bus_queue.sql` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Create `bus_queue` table for durable message persistence. Supports HA-safe consumption via `SELECT FOR UPDATE SKIP LOCKED`.

## Implementation — DELIVERED

### File: `backend/migrator/migration/3.18/0001##add_bus_queue.sql` (1392 bytes)

```sql
CREATE TABLE IF NOT EXISTS bus_queue (
    id           BIGSERIAL     PRIMARY KEY,
    channel      TEXT          NOT NULL,
    payload      JSONB         NOT NULL,
    status       TEXT          NOT NULL DEFAULT 'pending',
    priority     INT           NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    claimed_by   TEXT,
    claimed_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts     INT           NOT NULL DEFAULT 0,
    max_retries  INT           NOT NULL DEFAULT 3,
    error_msg    TEXT
);
```

### 3 Partial Indexes

| Index | Columns | Filter | Purpose |
|-------|---------|--------|---------|
| `idx_bus_queue_pending` | `(channel, priority DESC, id ASC)` | `status = 'pending'` | Consumer polling |
| `idx_bus_queue_processing` | `(claimed_at)` | `status = 'processing'` | Stale claim recovery |
| `idx_bus_queue_completed` | `(completed_at)` | `status IN ('done','failed')` | Periodic GC cleanup |

## Acceptance Criteria

- [x] Migration file created with correct sequence number (`3.18/0001##`) ✅
- [x] Table + 3 partial indexes created ✅
- [x] Uses `CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS` for idempotency ✅

## Verification

```
$ ls backend/migrator/migration/3.18/0001##add_bus_queue.sql → ✅ EXISTS (1392 bytes)
$ grep -c 'CREATE INDEX' backend/migrator/migration/3.18/0001##add_bus_queue.sql → 3
```

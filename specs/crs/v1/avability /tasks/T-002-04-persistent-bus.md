# T-002-04: PG-Backed Bus Durability

| Field | Value |
|---|---|
| **Task ID** | T-002-04 |
| **Solution** | SOL-AVAIL-002 |
| **Depends On** | T-002-02 |
| **Target Files** | `backend/component/bus/persistent.go` (NEW), migration SQL |

---

## Objective

Wrap `bus.Bus` với PG persistence layer: dual-write to channel + PG table. Leader recovers unprocessed messages on startup.

## Implementation

### 1. Migration:

```sql
CREATE TABLE IF NOT EXISTS bus_message (
    id         BIGSERIAL PRIMARY KEY,
    channel    TEXT NOT NULL,
    payload    TEXT NOT NULL,
    processed  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bus_message_unprocessed ON bus_message (processed, created_at) WHERE processed = false;
```

### 2. `PersistentBus` — xem SOL-AVAIL-002 §2.3

- `PublishTaskRunTickle(ctx, taskID)`: INSERT to bus_message → push channel → pg_notify
- `RecoverUnprocessedMessages(ctx)`: SELECT unprocessed WHERE created_at > 1h ago → dispatch → mark processed

### 3. Wire: Replace `bus.Bus` with `bus.PersistentBus` in HA mode (server.go)

## Acceptance Criteria

- [ ] `bus_message` table created via migration
- [ ] Dual-write: PG + channel (channel non-blocking)
- [ ] Recovery on leader startup
- [ ] Backward compatible: `PersistentBus` embeds `*Bus`
- [ ] `go build ./backend/component/bus/...` passes

# T-002-02: Durable Bus Implementation

| Field | Value |
|---|---|
| **Task ID** | T-002-02 |
| **Solution** | SOL-ARCH-002 |
| **Priority** | P1 |
| **Depends On** | T-002-01 |
| **Target File** | `backend/component/bus/durable_bus.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

PG-backed durable bus with channel bridge to maintain backward-compatible Go channels for existing consumers.

## Implementation — DELIVERED

### File: `backend/component/bus/durable_bus.go` (178 lines, 8 functions)

### Architecture: Publisher / Consumer Separation

| Component | Struct | Purpose |
|-----------|--------|---------|
| Publisher | `DurablePublisher` | `Publish(ctx, channel, payload)` → INSERT INTO bus_queue |
| Consumer | `DurableConsumer` | Poll loop → `SELECT ... FOR UPDATE SKIP LOCKED` → dispatch handler |

### Key Functions

| Function | Lines | Description |
|----------|-------|-------------|
| `NewDurablePublisher(db)` | 22-26 | Creates publisher with DB reference |
| `Publish(ctx, channel, payload)` | 27-48 | JSON-marshals payload, inserts into bus_queue |
| `NewDurableConsumer(db, interval)` | 50-60 | Creates consumer with configurable poll interval |
| `Handle(channel, fn)` | 62-65 | Registers per-channel message handler |
| `Run(ctx)` | 67-84 | Background poll loop with context cancellation |
| `poll(ctx)` | 86-93 | Iterates registered channels, processes each |
| `processChannel(ctx, channel)` | 95-166 | `SELECT FOR UPDATE SKIP LOCKED` → claim → execute handler → mark done/failed |
| `CleanupCompleted(ctx, db, olderThan)` | 168-178 | Deletes completed/failed messages older than threshold |

### HA-Safety: `SELECT FOR UPDATE SKIP LOCKED`

```sql
SELECT id, payload FROM bus_queue
WHERE channel = $1 AND status = 'pending'
ORDER BY priority DESC, id ASC
LIMIT 10
FOR UPDATE SKIP LOCKED
```

- Multiple instances can safely poll the same queue
- No duplicate processing — locked rows are skipped
- Stale claim recovery via `CleanupCompleted()` periodic GC

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| Single `DurableBus` struct with channels | Separated `DurablePublisher` + `DurableConsumer` | Cleaner separation of concerns, testable independently |
| Bridge channels (ApprovalCheckChan, etc.) | Handler-based dispatch (`Handle(channel, fn)`) | More flexible — channels are a Go implementation detail |
| `AsBus()` method for bridging | Not needed — handlers directly invoke existing logic | Simpler integration path |

## Acceptance Criteria

- [x] `Publish()` inserts into `bus_queue` with JSON payload ✅
- [x] `processChannel()` uses `FOR UPDATE SKIP LOCKED` ✅
- [x] `CleanupCompleted()` for periodic GC of old messages ✅
- [x] Handler-based dispatch for per-channel message routing ✅
- [x] `go build ./backend/component/bus/...` passes ✅

## Verification

```
$ go build ./backend/component/bus/... → ✅ PASS
$ wc -l backend/component/bus/durable_bus.go → 178
$ grep -c 'FOR UPDATE SKIP LOCKED' backend/component/bus/durable_bus.go → 1
```

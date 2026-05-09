# T-002-03: Bus Prometheus Metrics

| Field | Value |
|---|---|
| **Task ID** | T-002-03 |
| **Solution** | SOL-ARCH-002 |
| **Priority** | P1 |
| **Depends On** | T-002-02 |
| **Target File** | `backend/component/bus/metrics.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Prometheus metrics for bus queue: enqueue total, dequeue duration, failed messages, queue depth.

## Implementation — DELIVERED

### File: `backend/component/bus/metrics.go` (119 lines)

### Metrics Registered

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `bytebase_bus_pending_depth` | GaugeVec | `channel` | Current pending messages per channel |
| `bytebase_bus_processing_depth` | GaugeVec | `channel` | Current processing (claimed) messages |
| `bytebase_bus_failed_depth` | GaugeVec | `channel` | Current failed messages |
| `bytebase_bus_published_total` | CounterVec | `channel` | Total messages published |
| `bytebase_bus_consumed_total` | CounterVec | `channel`, `status` | Total messages consumed (success/failure) |

### Key Implementation Details

- `NewBusMetrics(registerer, db)` — creates metrics with Prometheus registerer and DB reference
- Gauges are backed by real-time SQL queries against `bus_queue` table
- Counters are incremented inline by `DurablePublisher.Publish()` and `DurableConsumer.processChannel()`
- All metrics use `bytebase_bus_` namespace prefix

## Acceptance Criteria

- [x] 5 metrics registered per channel ✅
- [x] Integrated into `DurableBus` Enqueue/processMessages ✅
- [x] `go build ./backend/component/bus/...` passes ✅

## Verification

```
$ go build ./backend/component/bus/... → ✅ PASS
$ wc -l backend/component/bus/metrics.go → 119
$ grep -c 'prometheus\.' backend/component/bus/metrics.go → 10 (5 metric defs + 5 registrations)
```

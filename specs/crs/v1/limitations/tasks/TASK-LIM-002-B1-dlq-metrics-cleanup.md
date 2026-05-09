# TASK-LIM-002-B1: DLQ Admin API + Outbox Cleanup

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-002 |
| Phase | B — Observability |
| Priority | P1 |
| Depends On | TASK-LIM-002-A2 |
| Est. | M (~180 LoC) |

## Objective

Add DLQ inspection/replay/purge API endpoints and extend DataCleaner runner to clean processed outbox messages.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/api/v1/bus_admin_service.go` |
| CREATE | `backend/component/bus/metrics.go` |
| MODIFY | `backend/runner/cleaner/data_cleaner.go` — add `cleanBusOutbox()` |

## Specification

### `bus_admin_service.go` — 3 RPC endpoints

- `ListDLQMessages` — SELECT from bus_outbox WHERE status='DLQ', with pagination
- `ReplayDLQMessage(id)` — UPDATE status='PENDING', attempt=0 WHERE id=$1 AND status='DLQ'
- `PurgeDLQ(before_date)` — DELETE WHERE status='DLQ' AND created_at < $1

### `metrics.go` — Prometheus counters

- `bytebase_bus_messages_published_total{subject}`
- `bytebase_bus_messages_consumed_total{subject}`
- `bytebase_bus_messages_failed_total{subject}`
- `bytebase_bus_messages_dlq_total{subject}`
- `bytebase_bus_messages_pending{subject}`

### DataCleaner extension

```go
func (c *DataCleaner) cleanBusOutbox(ctx context.Context) {
    retention := 24 * time.Hour  // configurable via BUS_DONE_RETENTION
    c.store.GetDB().ExecContext(ctx, `DELETE FROM bus_outbox WHERE status='DONE' AND processed_at < $1`, time.Now().Add(-retention))
}
```

## Acceptance Criteria

- [ ] DLQ list, replay, purge endpoints functional
- [ ] Replayed messages are re-processed by consumers
- [ ] DataCleaner removes DONE messages after retention period
- [ ] Prometheus metrics exported for all bus operations

# T-001-02: Enhanced Heartbeat Store Methods

| Field | Value |
|---|---|
| **Task ID** | T-001-02 |
| **Solution** | SOL-AVAIL-001 |
| **Depends On** | T-001-01 |
| **Target File** | `backend/store/replica_heartbeat.go` (Modify) |

---

## Objective

Mở rộng `UpsertReplicaHeartbeat` nhận `ReplicaNode`, thêm `MarkStaleReplicas`, `ListActiveReplicas`.

## Context — Current signature (line 13)

```go
func (s *Store) UpsertReplicaHeartbeat(ctx context.Context, replicaID string) error {
```

## Implementation

1. **Change signature**: `UpsertReplicaHeartbeat(ctx, *model.ReplicaNode) error`
   - INSERT with all new columns, ON CONFLICT update heartbeat + status + endpoint

2. **New**: `MarkStaleReplicas(ctx, threshold) (int64, error)`
   - UPDATE status='UNHEALTHY' WHERE status IN ('STARTING','READY') AND last_heartbeat < now()-threshold

3. **New**: `ListActiveReplicas(ctx, within) ([]*model.ReplicaNode, error)`
   - SELECT * WHERE last_heartbeat > now()-within ORDER BY replica_id

> **Breaking change**: `UpsertReplicaHeartbeat` signature changes — update heartbeat runner caller (T-001-03).

## Acceptance Criteria

- [x] `UpsertReplicaHeartbeat` accepts `*model.ReplicaNode`
- [x] `MarkStaleReplicas` updates status for stale nodes
- [x] `ListActiveReplicas` returns active nodes list
- [x] Existing `CountActiveReplicas` and `DeleteStaleReplicaHeartbeats` unchanged
- [x] `go build ./backend/store/...` passes

# T-002-01: Advisory Lock Keys Extension

| Field | Value |
|---|---|
| **Task ID** | T-002-01 |
| **Solution** | SOL-AVAIL-002 |
| **Priority** | P1 |
| **Depends On** | None |
| **Target File** | `backend/store/advisory_lock.go` (Modify) |

---

## Objective

Thêm 3 advisory lock keys mới cho leader election, backup, và health monitoring.

## Context — Current code (line 12-14)

```go
const (
    AdvisoryLockKeyPendingScheduler AdvisoryLockKey = 1001
    AdvisoryLockKeyMigration        AdvisoryLockKey = 1002
    AdvisoryLockKeySchemaSyncer     AdvisoryLockKey = 1003
)
```

## Implementation

Thêm sau line 14:

```go
    // Availability runners (SOL-AVAIL-002)
    AdvisoryLockKeyLeader        AdvisoryLockKey = 2001 // Cluster leader election
    AdvisoryLockKeyBackup        AdvisoryLockKey = 2002 // Backup coordinator
    AdvisoryLockKeyHealthMonitor AdvisoryLockKey = 2003 // Health monitor
```

## Acceptance Criteria

- [ ] 3 new constants added (2001-2003)
- [ ] Existing keys (1001-1003) unchanged
- [ ] `go build ./backend/store/...` passes

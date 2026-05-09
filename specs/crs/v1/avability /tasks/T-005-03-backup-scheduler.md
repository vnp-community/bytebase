# T-005-03: Backup Scheduler + RPO Monitor

| Field | Value |
|---|---|
| **Task ID** | T-005-03 |
| **Solution** | SOL-AVAIL-005 |
| **Depends On** | T-005-02, T-002-02 |
| **Target Files** | `backend/runner/backup/scheduler.go` (NEW), `backend/server/server.go` (Modify) |

---

## Objective

Cron-based backup scheduler: full backup (daily 02:00), RPO compliance check (every 5min). Leader-only via `isLeader()`.

## Implementation

Xem SOL-AVAIL-005 §2.1. Key design:

```go
type Scheduler struct {
    store      *store.Store
    profile    *config.Profile
    executor   *Executor
    cronEngine *cron.Cron
    isLeader   func() bool  // Injected from leader runner
}
```

- `Run(ctx, wg)`: if `!profile.BackupEnabled` → return immediately
- Cron full backup: `isLeader()` check → acquire advisory lock 2002 → execute → verify
- RPO check: every 5min → compare last backup timestamp vs `BB_TARGET_RPO_MINUTES`

### Config env vars:

| Var | Default |
|---|---|
| `BB_BACKUP_ENABLED` | false |
| `BB_BACKUP_SCHEDULE` | `0 2 * * *` |
| `BB_BACKUP_PATH` | `/data/backups` |
| `BB_TARGET_RPO_MINUTES` | 15 |

### Wire in `server.go`:

```go
if s.profile.BackupEnabled {
    s.runnerWG.Add(1)
    go s.backupScheduler.Run(ctx, &s.runnerWG)
}
```

## Acceptance Criteria

- [ ] Cron-based scheduling (robfig/cron)
- [ ] Leader-only execution
- [ ] RPO compliance monitoring with Prometheus metrics
- [ ] Disabled by default (`BB_BACKUP_ENABLED=false`)
- [ ] `go build ./backend/...` passes

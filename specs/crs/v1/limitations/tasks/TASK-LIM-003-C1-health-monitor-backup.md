# TASK-LIM-003-C1: PG Health Monitor + Backup Scheduler

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-003 |
| Phase | C — Operations |
| Priority | P1 |
| Depends On | — |
| Est. | M (~250 LoC) |

## Objective

Monitor embedded PG health metrics (connections, size, WAL, long queries) and schedule periodic backups with rotation.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/pghealth/monitor.go` |
| CREATE | `backend/component/pgbackup/scheduler.go` |

## Specification

### `monitor.go` — PGHealthMonitor

Runs every 30s, collects and exports Prometheus metrics:
- `bytebase_pg_active_connections` — `SELECT COUNT(*) FROM pg_stat_activity WHERE state='active'`
- `bytebase_pg_database_size_mb` — `SELECT pg_database_size(current_database())/1024/1024`
- `bytebase_pg_longest_query_seconds` — from `pg_stat_activity`
- `bytebase_pg_wal_size_mb` — `pg_wal_lsn_diff`

Only runs when `profile.UseEmbedDB() == true`.

### `scheduler.go` — BackupScheduler

- Runs as cron job (default: `0 2 * * *` = daily 2 AM)
- Creates backup: `pg_dump --format custom --compress 6`
- Rotation: keep last N backups (default 7), delete oldest
- Config: `PG_BACKUP_SCHEDULE`, `PG_BACKUP_RETENTION`, `PG_BACKUP_DIR`
- Only active for embedded PG mode

## Acceptance Criteria

- [ ] Health metrics exported every 30s
- [ ] Backup created on schedule with compression
- [ ] Old backups rotated (keep last N)
- [ ] Both components no-op for external PG

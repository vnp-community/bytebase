# T-005-01: Backup Registry Store

| Field | Value |
|---|---|
| **Task ID** | T-005-01 |
| **Solution** | SOL-AVAIL-005 |
| **Priority** | P1 |
| **Depends On** | None |
| **Target Files** | `backend/store/backup_registry.go` (NEW), migration SQL |

---

## Objective

Tạo `backup_registry` table và CRUD store methods cho backup records.

## Implementation

### 1. Migration:

```sql
CREATE TABLE IF NOT EXISTS backup_registry (
    id              BIGSERIAL PRIMARY KEY,
    backup_id       TEXT UNIQUE NOT NULL,
    backup_type     TEXT NOT NULL,          -- FULL, INCREMENTAL, WAL
    status          TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    size_bytes      BIGINT NOT NULL DEFAULT 0,
    checksum_sha256 TEXT,
    storage_path    TEXT NOT NULL,
    storage_type    TEXT NOT NULL DEFAULT 'LOCAL',
    encryption      TEXT NOT NULL DEFAULT 'NONE',
    data_timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at     TIMESTAMPTZ,
    verify_result   TEXT,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    completed_at    TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_backup_registry_status ON backup_registry (status);
CREATE INDEX idx_backup_registry_data_time ON backup_registry (data_timestamp DESC);
```

### 2. Store methods — xem SOL-AVAIL-005 §2.5:

- `CreateBackupRecord(ctx, *BackupRecord) error`
- `GetLatestSuccessfulBackup(ctx) (*BackupRecord, error)`
- `GetBackupRecord(ctx, backupID) (*BackupRecord, error)`
- `UpdateBackupVerification(ctx, backupID, status, result) error`

## Acceptance Criteria

- [x] `BackupRecord` struct matching table schema
- [x] 4 CRUD methods
- [x] Migration SQL idempotent
- [x] `go build ./backend/store/...` passes

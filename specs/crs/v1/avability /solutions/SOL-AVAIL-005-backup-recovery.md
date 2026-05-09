# Solution: Backup, Recovery & RPO/RTO Compliance

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-005                            |
| **CR ID**      | CR-AVAIL-005                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L6 (Runner), L8 (Store), L10 (Infra)     |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng

| Component | File | Capability |
|---|---|---|
| **Self-migration** | `backend/migrator/migrator.go` | Schema migration engine — understands PG schema |
| **LATEST.sql** | `backend/migrator/migration/LATEST.sql` | Complete current schema definition |
| **Data Cleaner** | `backend/runner/cleaner/` | Periodic cleanup pattern (runner pattern) |
| **Advisory Lock** | `backend/store/advisory_lock.go` | Prevents concurrent backup (key 2002) |
| **Embedded PG** | `backend/resources/postgres/` | Start/stop PG instances — can start restore targets |
| **Heartbeat** | `backend/runner/heartbeat/runner.go` | Runner lifecycle pattern for backup scheduler |
| **Existing backup** | `backend/api/v1/database_service.go` | Backup for managed databases (DCM-07 in PRD) |

### 1.2 Key Insight: Metadata PG URL

```go
// server.go:136-137
pgURL = profile.PgURL  // External PG URL for HA
// → We can use this URL for pg_dump/pg_restore tooling
```

The metadata database connection is a standard PostgreSQL URL → `pg_dump` and `pg_restore` tools work directly against it.

---

## 2. Giải pháp kỹ thuật

### 2.1 Backup Scheduler Runner

**File**: `backend/runner/backup/scheduler.go` (NEW)

```go
package backup

import (
    "context"
    "log/slog"
    "sync"
    "time"
    
    "github.com/robfig/cron/v3"
    
    "github.com/bytebase/bytebase/backend/store"
)

// Scheduler manages automated metadata backup operations.
type Scheduler struct {
    store       *store.Store
    profile     *config.Profile
    executor    *Executor
    verifier    *Verifier
    cronEngine  *cron.Cron
    isLeader    func() bool  // Injected from leader runner
}

func NewScheduler(store *store.Store, profile *config.Profile, isLeader func() bool) *Scheduler {
    return &Scheduler{
        store:    store,
        profile:  profile,
        executor: NewExecutor(store, profile),
        verifier: NewVerifier(store, profile),
        isLeader: isLeader,
    }
}

func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    
    if !s.profile.BackupEnabled {
        slog.Info("Backup scheduler disabled")
        return
    }
    
    s.cronEngine = cron.New()
    
    // Full backup schedule (default: daily 02:00 UTC)
    schedule := s.profile.BackupScheduleFull
    if schedule == "" {
        schedule = "0 2 * * *"
    }
    s.cronEngine.AddFunc(schedule, func() {
        if s.isLeader() {
            s.runFullBackup(ctx)
        }
    })
    
    // RPO monitoring (every 5 minutes)
    s.cronEngine.AddFunc("*/5 * * * *", func() {
        if s.isLeader() {
            s.checkRPOCompliance(ctx)
        }
    })
    
    s.cronEngine.Start()
    
    <-ctx.Done()
    s.cronEngine.Stop()
}

func (s *Scheduler) runFullBackup(ctx context.Context) {
    // Acquire backup advisory lock
    lock, acquired, err := store.TryAdvisoryLock(ctx, s.store.GetDB(), store.AdvisoryLockKeyBackup)
    if !acquired {
        slog.Info("Backup already running on another node")
        return
    }
    defer lock.Release()
    
    slog.Info("Starting full metadata backup")
    
    result, err := s.executor.ExecuteFullBackup(ctx)
    if err != nil {
        slog.Error("Backup failed", slog.String("error", err.Error()))
        // Record failure
        s.store.CreateBackupRecord(ctx, &store.BackupRecord{
            BackupType: "FULL",
            Status:     "FAILED",
            Error:      err.Error(),
        })
        return
    }
    
    slog.Info("Backup completed",
        slog.String("backupID", result.BackupID),
        slog.Int64("sizeBytes", result.SizeBytes),
        slog.Duration("duration", result.Duration))
    
    // Verify backup
    if s.profile.BackupVerifyEnabled {
        go s.verifier.VerifyBackup(context.Background(), result.BackupID)
    }
}

func (s *Scheduler) checkRPOCompliance(ctx context.Context) {
    lastBackup, err := s.store.GetLatestSuccessfulBackup(ctx)
    if err != nil || lastBackup == nil {
        return
    }
    
    rpoMinutes := time.Since(lastBackup.CompletedAt).Minutes()
    targetRPO := float64(s.profile.TargetRPOMinutes)
    
    // Update metrics
    rpoCurrentGauge.Set(rpoMinutes)
    rpoCompliantGauge.Set(boolToFloat(rpoMinutes <= targetRPO))
    
    if rpoMinutes > targetRPO {
        slog.Error("RPO VIOLATION",
            slog.Float64("actualMinutes", rpoMinutes),
            slog.Float64("targetMinutes", targetRPO))
    }
}
```

### 2.2 Backup Executor — pg_dump Wrapper

**File**: `backend/runner/backup/executor.go` (NEW)

```go
package backup

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "time"
)

type BackupResult struct {
    BackupID    string
    BackupPath  string
    SizeBytes   int64
    Checksum    string
    Duration    time.Duration
    TableCount  int
    Encrypted   bool
}

type Executor struct {
    store   *store.Store
    profile *config.Profile
}

func (e *Executor) ExecuteFullBackup(ctx context.Context) (*BackupResult, error) {
    backupID := fmt.Sprintf("bb-full-%s", time.Now().UTC().Format("20060102-150405"))
    backupDir := filepath.Join(e.profile.BackupLocalPath, time.Now().Format("2006-01-02"))
    os.MkdirAll(backupDir, 0755)
    
    dumpPath := filepath.Join(backupDir, backupID+".dump")
    
    start := time.Now()
    
    // pg_dump with custom format (parallel + compressed)
    cmd := exec.CommandContext(ctx, "pg_dump",
        e.profile.PgURL,
        "--format=custom",
        "--compress=9",
        "--no-owner",
        "--no-acl",
        "--exclude-table=replica_heartbeat",  // Transient data
        "--exclude-table=bus_message",         // Transient data
        "--file="+dumpPath,
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("pg_dump failed: %s: %w", string(output), err)
    }
    
    duration := time.Since(start)
    
    // Calculate checksum
    checksum, err := sha256File(dumpPath)
    if err != nil {
        return nil, fmt.Errorf("checksum failed: %w", err)
    }
    
    // Get file size
    info, _ := os.Stat(dumpPath)
    sizeBytes := info.Size()
    
    // Encrypt if key configured
    encrypted := false
    if e.profile.BackupEncryptionKey != "" {
        encPath := dumpPath + ".enc"
        if err := encryptFile(dumpPath, encPath, e.profile.BackupEncryptionKey); err != nil {
            return nil, fmt.Errorf("encryption failed: %w", err)
        }
        os.Remove(dumpPath)
        dumpPath = encPath
        encrypted = true
    }
    
    result := &BackupResult{
        BackupID:   backupID,
        BackupPath: dumpPath,
        SizeBytes:  sizeBytes,
        Checksum:   checksum,
        Duration:   duration,
        Encrypted:  encrypted,
    }
    
    // Record in backup registry
    e.store.CreateBackupRecord(ctx, &store.BackupRecord{
        BackupID:       backupID,
        BackupType:     "FULL",
        Status:         "COMPLETED",
        SizeBytes:      sizeBytes,
        ChecksumSHA256: checksum,
        StoragePath:    dumpPath,
        StorageType:    "LOCAL",
        Encryption:     boolStr(encrypted, "AES-256-GCM", "NONE"),
        DataTimestamp:  time.Now(),
        CompletedAt:    timePtr(time.Now()),
    })
    
    return result, nil
}

func sha256File(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func encryptFile(srcPath, dstPath, keyBase64 string) error {
    key, _ := base64.StdEncoding.DecodeString(keyBase64)
    
    src, _ := os.ReadFile(srcPath)
    
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    
    ciphertext := gcm.Seal(nonce, nonce, src, nil)
    return os.WriteFile(dstPath, ciphertext, 0600)
}
```

### 2.3 WAL Archiving Configuration

**Approach**: WAL archiving is a PostgreSQL-level configuration. For external PG (HA mode), provide configuration guidance. For potential future embedded PG scenarios, provide `archive_command`.

**File**: `deploy/postgres/postgresql.conf.ha` (NEW)

```ini
# WAL Archiving for RPO compliance
wal_level = replica
archive_mode = on
archive_command = 'wal-g wal-push %p'
archive_timeout = 60  # Archive every 60 seconds minimum → RPO ≤ 1 min

# Connection settings aligned with SOL-AVAIL-004
max_connections = 100
```

**Documentation**: Provide clear setup instructions for external PG WAL archiving. The Bytebase backup scheduler handles the application-level backup logic; WAL archiving is infrastructure-level managed by the PG admin.

### 2.4 Backup Verification

**File**: `backend/runner/backup/verifier.go` (NEW)

```go
package backup

// Verifier restores backups to a temp database and validates integrity.
type Verifier struct {
    store   *store.Store
    profile *config.Profile
}

func (v *Verifier) VerifyBackup(ctx context.Context, backupID string) error {
    record, err := v.store.GetBackupRecord(ctx, backupID)
    if err != nil {
        return err
    }
    
    slog.Info("Verifying backup", slog.String("backupID", backupID))
    
    // 1. Create temp verification database
    verifyDBName := "bb_verify_" + backupID[:8]
    _, err = v.store.GetDB().ExecContext(ctx,
        fmt.Sprintf("CREATE DATABASE %s", verifyDBName))
    if err != nil {
        return fmt.Errorf("create verify db: %w", err)
    }
    defer func() {
        v.store.GetDB().ExecContext(context.Background(),
            fmt.Sprintf("DROP DATABASE IF EXISTS %s", verifyDBName))
    }()
    
    // 2. Restore backup
    restorePath := record.StoragePath
    if record.Encryption == "AES-256-GCM" {
        restorePath, err = decryptToTemp(record.StoragePath, v.profile.BackupEncryptionKey)
        if err != nil {
            return err
        }
        defer os.Remove(restorePath)
    }
    
    cmd := exec.CommandContext(ctx, "pg_restore",
        "--dbname="+verifyDBName,
        "--no-owner",
        "--no-acl",
        restorePath,
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        v.store.UpdateBackupVerification(ctx, backupID, "FAILED", string(output))
        return fmt.Errorf("restore failed: %s", output)
    }
    
    // 3. Validate: check critical tables exist
    criticalTables := []string{
        "principal", "project", "instance", "db", "policy",
        "issue", "plan", "task", "setting", "audit_log",
    }
    
    verifyDB, _ := sql.Open("pgx", fmt.Sprintf("dbname=%s", verifyDBName))
    defer verifyDB.Close()
    
    for _, table := range criticalTables {
        var count int
        err := verifyDB.QueryRowContext(ctx,
            fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
        if err != nil {
            v.store.UpdateBackupVerification(ctx, backupID, "FAILED",
                fmt.Sprintf("table %s missing or unreadable", table))
            return err
        }
    }
    
    // 4. Mark verified
    v.store.UpdateBackupVerification(ctx, backupID, "VERIFIED", "")
    slog.Info("Backup verified successfully", slog.String("backupID", backupID))
    
    return nil
}
```

### 2.5 Backup Registry Store

**File**: `backend/store/backup_registry.go` (NEW)

```go
package store

type BackupRecord struct {
    ID             int64
    BackupID       string
    BackupType     string    // FULL, INCREMENTAL, WAL
    Status         string    // IN_PROGRESS, COMPLETED, FAILED, VERIFIED
    SizeBytes      int64
    ChecksumSHA256 string
    StoragePath    string
    StorageType    string    // LOCAL, S3
    Encryption     string
    DataTimestamp  time.Time
    VerifiedAt    *time.Time
    VerifyResult   string
    ExpiresAt      time.Time
    CompletedAt   *time.Time
    Error          string
    CreatedAt      time.Time
}

func (s *Store) CreateBackupRecord(ctx context.Context, record *BackupRecord) error {
    q := qb.Q().Space(`
        INSERT INTO backup_registry
            (backup_id, backup_type, status, size_bytes, checksum_sha256,
             storage_path, storage_type, encryption, data_timestamp, expires_at, error_message)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, record.BackupID, record.BackupType, record.Status, record.SizeBytes,
       record.ChecksumSHA256, record.StoragePath, record.StorageType,
       record.Encryption, record.DataTimestamp, record.ExpiresAt, record.Error)
    // ...
}

func (s *Store) GetLatestSuccessfulBackup(ctx context.Context) (*BackupRecord, error) {
    q := qb.Q().Space(`
        SELECT * FROM backup_registry
        WHERE status IN ('COMPLETED', 'VERIFIED')
        ORDER BY data_timestamp DESC
        LIMIT 1
    `)
    // ...
}
```

### 2.6 PITR Recovery API

**File**: `backend/api/v1/recovery_service.go` (NEW)

```go
// Recovery API — Admin only
func (s *RecoveryService) PointInTimeRecovery(ctx context.Context, req *v1pb.PITRRequest) (*v1pb.PITRResponse, error) {
    // 1. Validate admin permission
    // 2. Find latest full backup before target timestamp
    // 3. pg_restore full backup
    // 4. pg_wal_replay to target timestamp (requires WAL archive access)
    // 5. Verify data integrity
    // 6. Log recovery event
}
```

---

## 3. Database Migration

```sql
-- Backup registry table
CREATE TABLE IF NOT EXISTS backup_registry (
    id              BIGSERIAL   PRIMARY KEY,
    backup_id       TEXT        UNIQUE NOT NULL,
    backup_type     TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    size_bytes      BIGINT      NOT NULL DEFAULT 0,
    checksum_sha256 TEXT,
    storage_path    TEXT        NOT NULL,
    storage_type    TEXT        NOT NULL DEFAULT 'LOCAL',
    encryption      TEXT        NOT NULL DEFAULT 'NONE',
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

---

## 4. Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `BB_BACKUP_ENABLED` | `false` | Enable backup scheduler |
| `BB_BACKUP_SCHEDULE` | `0 2 * * *` | Full backup cron schedule |
| `BB_BACKUP_PATH` | `/data/backups` | Local backup directory |
| `BB_BACKUP_ENCRYPTION_KEY` | _(empty)_ | AES-256 key (base64), empty = no encryption |
| `BB_BACKUP_VERIFY` | `true` | Automated verification after backup |
| `BB_BACKUP_RETENTION_DAYS` | `30` | Backup retention period |
| `BB_TARGET_RPO_MINUTES` | `15` | RPO target for compliance monitoring |

---

## 5. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L6 | `backend/runner/backup/scheduler.go` | **New** | Backup scheduler with cron |
| L6 | `backend/runner/backup/executor.go` | **New** | pg_dump wrapper with encryption |
| L6 | `backend/runner/backup/verifier.go` | **New** | Automated restore verification |
| L8 | `backend/store/backup_registry.go` | **New** | Backup record CRUD |
| L8 | `backend/store/advisory_lock.go` | **Modify** | Add `AdvisoryLockKeyBackup` (2002) |
| L4 | `backend/api/v1/recovery_service.go` | **New** | PITR recovery API |
| L10 | `backend/migrator/migration/X.Y.Z/` | **New** | backup_registry table |
| L10 | `deploy/postgres/postgresql.conf.ha` | **New** | WAL archive config reference |
| L2 | `backend/server/server.go` | **Modify** | Wire backup scheduler runner |
| L5 | `backend/component/config/` | **Modify** | Backup-related profile fields |

---

## 6. Backward Compatibility

| Scenario | Behavior |
|---|---|
| `BB_BACKUP_ENABLED=false` | Backup runner starts but exits immediately (no overhead) |
| No `pg_dump` binary | Backup fails gracefully with clear error log |
| Non-HA (embedded PG) | Works, but warns that embedded PG should migrate to external |
| Existing managed DB backup | Unchanged — this addresses metadata PG backup only |

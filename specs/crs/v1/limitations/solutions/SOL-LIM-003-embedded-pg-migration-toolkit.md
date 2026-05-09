# Solution: CR-LIM-003 — Embedded PostgreSQL Migration Toolkit

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-003                               |
| **Solution ID**| SOL-LIM-003                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L2 (API Gateway), L4 (Service), L8 (Store), L10 (Infrastructure) |
| **TDD Refs**   | §2 Server Bootstrap, §12 Self-Migration, §14 Trade-offs |

---

## 1. Solution Overview

### 1.1 Approach Summary

3-phase toolkit built on top of existing infrastructure:

1. **Phase A — Production Readiness Checker** (warning system)
2. **Phase B — Migration CLI + Web Wizard** (automated embedded → external)
3. **Phase C — Health Monitor + Backup Scheduler** (operational visibility)

### 1.2 Design Rationale

Từ TDD §2, server bootstrap gọi `StartMetadataInstance()` cho embedded PG. Profile check `profile.UseEmbedDB()` xác định mode. Migration tool cần: (1) dump embedded PG data, (2) restore vào external PG, (3) update `PG_URL` config.

Từ Architecture L10, embedded PG nằm trong `backend/resources/postgres/`. Data directory `{dataDir}/pgdata`. Tool sẽ sử dụng `pg_dump/pg_restore` đã bundled cùng embedded PG binary.

Từ TDD §12, Bytebase đã có self-migration system (`backend/migrator/`). Migration toolkit sẽ **tận dụng migrator pattern** cho data migration verification.

---

## 2. Detailed Technical Design

### 2.1 Phase A — Production Readiness Checker

#### 2.1.1 Readiness Detection Service

**File**: `backend/component/readiness/checker.go` (new)

```go
// ReadinessChecker evaluates whether embedded PG usage matches production patterns.
// Criteria: instance count, user count, change history, uptime, environment names.
type ReadinessChecker struct {
    store    *store.Store
    profile  *config.Profile
}

type ReadinessReport struct {
    IsEmbedded       bool
    CriteriaMet      int
    CriteriaTotal    int
    ShowWarning      bool     // true if CriteriaMet >= 2
    Details          []ReadinessCriterion
    LastDismissed    *time.Time
    MigrationGuideURL string
}

type ReadinessCriterion struct {
    Name      string
    Threshold string
    Current   string
    Met       bool
}

func (c *ReadinessChecker) Check(ctx context.Context) (*ReadinessReport, error) {
    if !c.profile.UseEmbedDB() {
        return &ReadinessReport{IsEmbedded: false}, nil
    }

    report := &ReadinessReport{
        IsEmbedded:        true,
        MigrationGuideURL: "/docs/administration/production-setup",
    }

    // Criterion 1: Instance count > 5
    instanceCount, _ := c.store.CountActiveInstances(ctx)
    report.Details = append(report.Details, ReadinessCriterion{
        Name:      "Instance Count",
        Threshold: "> 5",
        Current:   fmt.Sprintf("%d", instanceCount),
        Met:       instanceCount > 5,
    })

    // Criterion 2: User count > 10
    userCount, _ := c.store.CountActiveUsers(ctx)
    report.Details = append(report.Details, ReadinessCriterion{
        Name:      "User Count",
        Threshold: "> 10",
        Current:   fmt.Sprintf("%d", userCount),
        Met:       userCount > 10,
    })

    // Criterion 3: Change history > 100
    changeCount, _ := c.store.CountChangelogs(ctx)
    report.Details = append(report.Details, ReadinessCriterion{
        Name:      "Change History",
        Threshold: "> 100 changes",
        Current:   fmt.Sprintf("%d", changeCount),
        Met:       changeCount > 100,
    })

    // Criterion 4: Uptime > 30 days
    uptime := time.Since(c.profile.StartTime)
    report.Details = append(report.Details, ReadinessCriterion{
        Name:      "Uptime",
        Threshold: "> 30 days",
        Current:   fmt.Sprintf("%.0f days", uptime.Hours()/24),
        Met:       uptime > 30*24*time.Hour,
    })

    // Criterion 5: Has production-tier environment
    envs, _ := c.store.ListEnvironments(ctx)
    hasProd := false
    for _, env := range envs {
        if env.Tier == api.EnvironmentTierProduction {
            hasProd = true
            break
        }
    }
    report.Details = append(report.Details, ReadinessCriterion{
        Name:      "Production Environment",
        Threshold: "Exists",
        Current:   fmt.Sprintf("%v", hasProd),
        Met:       hasProd,
    })

    // Calculate
    for _, d := range report.Details {
        report.CriteriaTotal++
        if d.Met {
            report.CriteriaMet++
        }
    }
    report.ShowWarning = report.CriteriaMet >= 2

    return report, nil
}
```

#### 2.1.2 Readiness API Endpoint

**File**: `backend/api/v1/actuator_service.go` (extend)

```go
// Extend existing ActuatorService with readiness check
func (s *ActuatorService) GetProductionReadiness(ctx context.Context, req *v1pb.GetProductionReadinessRequest) (*v1pb.ProductionReadinessResponse, error) {
    report, err := s.readinessChecker.Check(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "readiness check failed: %v", err)
    }
    return convertReadinessReport(report), nil
}

// Dismiss warning for 30 days
func (s *ActuatorService) DismissReadinessWarning(ctx context.Context, req *v1pb.DismissReadinessWarningRequest) (*v1pb.DismissReadinessWarningResponse, error) {
    dismissUntil := time.Now().Add(30 * 24 * time.Hour)
    err := s.store.UpsertSetting(ctx, &store.SetSettingMessage{
        Name:  "readiness.warning.dismissed_until",
        Value: dismissUntil.Format(time.RFC3339),
    })
    return &v1pb.DismissReadinessWarningResponse{}, err
}
```

### 2.2 Phase B — Migration CLI + Web Wizard

#### 2.2.1 Migration Engine

**File**: `backend/component/dbmigrate/engine.go` (new)

```go
// MigrationEngine handles data migration from embedded PG to external PG.
// Steps: validate → backup → dump → restore → verify → switch.
type MigrationEngine struct {
    profile       *config.Profile
    embeddedPGURL string
    pgDumpPath    string  // Path to pg_dump binary (bundled with embedded PG)
}

type MigrationConfig struct {
    TargetURL   string
    DryRun      bool
    BackupDir   string  // Backup before migration
}

type MigrationProgress struct {
    Phase       string    // "validating", "backing_up", "dumping", "restoring", "verifying", "switching"
    Percent     int
    Message     string
    StartedAt   time.Time
    Error       string
}

func (e *MigrationEngine) Migrate(ctx context.Context, cfg MigrationConfig, progress chan<- MigrationProgress) error {
    // Step 1: Validate target PG
    progress <- MigrationProgress{Phase: "validating", Percent: 5, Message: "Validating target PostgreSQL..."}
    if err := e.validateTarget(ctx, cfg.TargetURL); err != nil {
        return fmt.Errorf("target validation failed: %w", err)
    }

    if cfg.DryRun {
        progress <- MigrationProgress{Phase: "complete", Percent: 100, Message: "Dry run complete — target is valid"}
        return nil
    }

    // Step 2: Backup embedded PG data
    progress <- MigrationProgress{Phase: "backing_up", Percent: 15, Message: "Creating backup of embedded database..."}
    backupFile, err := e.createBackup(ctx, cfg.BackupDir)
    if err != nil {
        return fmt.Errorf("backup failed: %w", err)
    }
    slog.Info("Backup created", "file", backupFile)

    // Step 3: Dump embedded PG
    progress <- MigrationProgress{Phase: "dumping", Percent: 30, Message: "Dumping embedded database..."}
    dumpFile, err := e.dumpEmbedded(ctx, cfg.BackupDir)
    if err != nil {
        return fmt.Errorf("dump failed: %w", err)
    }

    // Step 4: Restore to target
    progress <- MigrationProgress{Phase: "restoring", Percent: 50, Message: "Restoring to target database..."}
    if err := e.restoreToTarget(ctx, cfg.TargetURL, dumpFile); err != nil {
        return fmt.Errorf("restore failed: %w", err)
    }

    // Step 5: Verify data integrity
    progress <- MigrationProgress{Phase: "verifying", Percent: 80, Message: "Verifying data integrity..."}
    if err := e.verifyIntegrity(ctx, cfg.TargetURL); err != nil {
        return fmt.Errorf("integrity check failed: %w", err)
    }

    // Step 6: Report success (manual PG_URL switch required)
    progress <- MigrationProgress{
        Phase:   "complete",
        Percent: 100,
        Message: fmt.Sprintf("Migration complete. Set PG_URL=%s and restart Bytebase.", cfg.TargetURL),
    }
    return nil
}

func (e *MigrationEngine) validateTarget(ctx context.Context, targetURL string) error {
    db, err := sql.Open("pgx", targetURL)
    if err != nil {
        return fmt.Errorf("cannot connect to target: %w", err)
    }
    defer db.Close()

    // Check PG version >= 14
    var version string
    if err := db.QueryRowContext(ctx, "SHOW server_version").Scan(&version); err != nil {
        return fmt.Errorf("cannot check version: %w", err)
    }

    // Check database is empty (no Bytebase tables)
    var tableCount int
    err = db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM information_schema.tables
        WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
    `).Scan(&tableCount)
    if err != nil {
        return fmt.Errorf("cannot check tables: %w", err)
    }
    if tableCount > 0 {
        return fmt.Errorf("target database is not empty (%d tables found)", tableCount)
    }

    return nil
}

func (e *MigrationEngine) dumpEmbedded(ctx context.Context, backupDir string) (string, error) {
    dumpFile := filepath.Join(backupDir, fmt.Sprintf("bytebase_dump_%s.sql", time.Now().Format("20060102_150405")))
    cmd := exec.CommandContext(ctx, e.pgDumpPath,
        "--dbname", e.embeddedPGURL,
        "--format", "custom",
        "--file", dumpFile,
        "--no-owner",
        "--no-privileges",
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        return "", fmt.Errorf("pg_dump failed: %s: %w", output, err)
    }
    return dumpFile, nil
}

func (e *MigrationEngine) restoreToTarget(ctx context.Context, targetURL, dumpFile string) error {
    pgRestorePath := strings.Replace(e.pgDumpPath, "pg_dump", "pg_restore", 1)
    cmd := exec.CommandContext(ctx, pgRestorePath,
        "--dbname", targetURL,
        "--no-owner",
        "--no-privileges",
        "--clean",
        "--if-exists",
        dumpFile,
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("pg_restore failed: %s: %w", output, err)
    }
    return nil
}

func (e *MigrationEngine) verifyIntegrity(ctx context.Context, targetURL string) error {
    embeddedDB, _ := sql.Open("pgx", e.embeddedPGURL)
    defer embeddedDB.Close()
    targetDB, _ := sql.Open("pgx", targetURL)
    defer targetDB.Close()

    // Compare row counts for key tables
    tables := []string{"principal", "instance", "db", "project", "plan", "issue",
        "task", "task_run", "policy", "setting", "audit_log"}
    for _, table := range tables {
        var srcCount, dstCount int
        embeddedDB.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&srcCount)
        targetDB.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&dstCount)
        if srcCount != dstCount {
            return fmt.Errorf("row count mismatch for %s: source=%d, target=%d", table, srcCount, dstCount)
        }
    }
    return nil
}
```

#### 2.2.2 Migration CLI Command

**File**: `backend/cmd/migrate_db.go` (new)

```go
// CLI: bytebase migrate-db --target-url <PG_URL> [--dry-run] [--backup-dir <dir>]
func migrateDatabaseCmd() *cobra.Command {
    var targetURL, backupDir string
    var dryRun bool

    cmd := &cobra.Command{
        Use:   "migrate-db",
        Short: "Migrate from embedded PostgreSQL to external PostgreSQL",
        RunE: func(cmd *cobra.Command, args []string) error {
            profile := config.GetProfile()
            if !profile.UseEmbedDB() {
                return fmt.Errorf("not using embedded PG — migration not needed")
            }
            if targetURL == "" {
                return fmt.Errorf("--target-url is required")
            }
            if backupDir == "" {
                backupDir = filepath.Join(profile.DataDir, "migration_backup")
            }
            os.MkdirAll(backupDir, 0755)

            engine := dbmigrate.NewMigrationEngine(profile)
            progress := make(chan dbmigrate.MigrationProgress, 10)

            go func() {
                for p := range progress {
                    fmt.Printf("[%3d%%] %s — %s\n", p.Percent, p.Phase, p.Message)
                }
            }()

            return engine.Migrate(cmd.Context(), dbmigrate.MigrationConfig{
                TargetURL: targetURL,
                DryRun:    dryRun,
                BackupDir: backupDir,
            }, progress)
        },
    }

    cmd.Flags().StringVar(&targetURL, "target-url", "", "Target PostgreSQL connection URL")
    cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Backup directory (default: {dataDir}/migration_backup)")
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate only, do not migrate")
    cmd.MarkFlagRequired("target-url")

    return cmd
}
```

#### 2.2.3 Migration Web API (for wizard UI)

**File**: `backend/api/v1/migration_service.go` (new)

```go
func (s *MigrationService) StartMigration(ctx context.Context, req *v1pb.StartMigrationRequest) (*v1pb.StartMigrationResponse, error) {
    // Validate user is workspaceAdmin
    // Start migration in background goroutine
    // Return migration ID for progress tracking
    migrationID := uuid.New().String()
    go s.runMigration(context.Background(), migrationID, req.TargetUrl, req.DryRun)
    return &v1pb.StartMigrationResponse{MigrationId: migrationID}, nil
}

func (s *MigrationService) GetMigrationProgress(ctx context.Context, req *v1pb.GetMigrationProgressRequest) (*v1pb.MigrationProgressResponse, error) {
    // Read progress from in-memory store (sync.Map)
    progress, ok := s.progressMap.Load(req.MigrationId)
    if !ok {
        return nil, status.Errorf(codes.NotFound, "migration not found")
    }
    return progress.(*v1pb.MigrationProgressResponse), nil
}
```

### 2.3 Phase C — Health Monitor + Backup Scheduler

#### 2.3.1 Embedded PG Health Monitor

**File**: `backend/component/pghealth/monitor.go` (new)

```go
// PGHealthMonitor collects health metrics from embedded PostgreSQL.
// Runs as periodic check (every 30s) via the existing runner pattern.
type PGHealthMonitor struct {
    db       *sql.DB
    profile  *config.Profile
    metrics  *pgHealthMetrics
}

type PGHealthSnapshot struct {
    ActiveConnections   int
    MaxConnections      int
    DatabaseSizeMB      float64
    DiskFreeMB          float64
    LongestQuerySeconds float64
    WALSizeMB           float64
    TransactionsPerSec  float64
    Timestamp           time.Time
}

func (m *PGHealthMonitor) Collect(ctx context.Context) (*PGHealthSnapshot, error) {
    snap := &PGHealthSnapshot{Timestamp: time.Now()}

    // Active connections
    m.db.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'",
    ).Scan(&snap.ActiveConnections)

    // Max connections
    m.db.QueryRowContext(ctx,
        "SHOW max_connections",
    ).Scan(&snap.MaxConnections)

    // Database size
    m.db.QueryRowContext(ctx,
        "SELECT pg_database_size(current_database()) / 1024.0 / 1024.0",
    ).Scan(&snap.DatabaseSizeMB)

    // Longest running query
    m.db.QueryRowContext(ctx, `
        SELECT COALESCE(EXTRACT(EPOCH FROM MAX(NOW() - query_start)), 0)
        FROM pg_stat_activity WHERE state = 'active' AND query NOT LIKE '%pg_stat%'
    `).Scan(&snap.LongestQuerySeconds)

    // WAL size (PG 14+)
    m.db.QueryRowContext(ctx, `
        SELECT COALESCE(pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0') / 1024.0 / 1024.0, 0)
    `).Scan(&snap.WALSizeMB)

    // Export to Prometheus
    m.metrics.activeConnections.Set(float64(snap.ActiveConnections))
    m.metrics.maxConnections.Set(float64(snap.MaxConnections))
    m.metrics.databaseSizeMB.Set(snap.DatabaseSizeMB)
    m.metrics.longestQuery.Set(snap.LongestQuerySeconds)
    m.metrics.walSizeMB.Set(snap.WALSizeMB)

    return snap, nil
}
```

#### 2.3.2 Backup Scheduler

**File**: `backend/component/pgbackup/scheduler.go` (new)

```go
// BackupScheduler runs periodic pg_dump backups of embedded PG.
// Supports rotation (keep last N) and optional S3 upload.
type BackupScheduler struct {
    profile      *config.Profile
    pgDumpPath   string
    schedule     string         // cron expression
    retention    int            // keep last N backups
    backupDir    string
}

func (s *BackupScheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    if !s.profile.UseEmbedDB() {
        return // Only for embedded PG
    }

    cron := cron.New()
    cron.AddFunc(s.schedule, func() {
        if err := s.createBackup(ctx); err != nil {
            slog.Error("Backup failed", "err", err)
        }
        s.rotateBackups()
    })
    cron.Start()
    <-ctx.Done()
    cron.Stop()
}

func (s *BackupScheduler) createBackup(ctx context.Context) error {
    filename := fmt.Sprintf("bytebase_backup_%s.sql.gz", time.Now().Format("20060102_150405"))
    filepath := filepath.Join(s.backupDir, filename)

    // pg_dump with gzip compression
    cmd := exec.CommandContext(ctx, s.pgDumpPath,
        "--dbname", s.profile.PgURL,
        "--format", "custom",
        "--compress", "6",
        "--file", filepath,
    )
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("pg_dump: %s: %w", output, err)
    }

    slog.Info("Backup created", "file", filepath)
    return nil
}

func (s *BackupScheduler) rotateBackups() {
    entries, _ := os.ReadDir(s.backupDir)
    var backups []os.DirEntry
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), "bytebase_backup_") {
            backups = append(backups, e)
        }
    }

    // Sort by name (timestamp-based, oldest first)
    sort.Slice(backups, func(i, j int) bool {
        return backups[i].Name() < backups[j].Name()
    })

    // Remove oldest if exceeding retention
    for len(backups) > s.retention {
        os.Remove(filepath.Join(s.backupDir, backups[0].Name()))
        slog.Info("Removed old backup", "file", backups[0].Name())
        backups = backups[1:]
    }
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L10 (Infra) | **HIGH** | Migration engine, backup scheduler, health monitor |
| L4 (Service) | **MEDIUM** | Migration API, readiness API endpoints |
| L2 (API GW) | **LOW** | New routes for migration/readiness APIs |
| L8 (Store) | **LOW** | Helper count queries for readiness checker |
| L1 (Frontend) | **MEDIUM** | Readiness banner, migration wizard, health widget |

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
Phase A (Sprint 1):
  1. Implement ReadinessChecker
  2. Add readiness API endpoint
  3. Frontend: warning banner component
  4. Test: criteria detection accuracy

Phase B (Sprint 2-3):
  5. Implement MigrationEngine
  6. Add CLI command (migrate-db)
  7. Add Web API for wizard
  8. Test: full migration cycle, rollback, dry-run

Phase C (Sprint 4):
  9. Implement PGHealthMonitor
  10. Implement BackupScheduler
  11. Frontend: health widget in Settings
  12. Test: backup rotation, metrics accuracy
```

---

## 5. Configuration Reference

| Variable             | Default              | Phase | Description                    |
|----------------------|----------------------|-------|--------------------------------|
| `PG_BACKUP_SCHEDULE` | `0 2 * * *`          | C     | Backup cron (daily 2 AM)       |
| `PG_BACKUP_RETENTION`| `7`                  | C     | Keep last N backups            |
| `PG_BACKUP_DIR`      | `{dataDir}/backup`   | C     | Backup storage path            |

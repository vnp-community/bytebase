# Solution: CR-PERF-001 — Metadata Store Scalability

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-001                              |
| **Solution ID**| SOL-PERF-001                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L8 (Data Access Layer), L10 (Infrastructure) |
| **TDD Refs**   | §4 Data Access Layer, §12 Self-Migration |

---

## 1. Solution Overview

### 1.1 Approach Summary

Thay vì full table partitioning (phức tạp, rủi ro cao), giải pháp sử dụng **3-phase approach**:

1. **Phase A — Denormalization + Indexes** (zero downtime, high impact)
2. **Phase B — Materialized columns** (low risk, removes COALESCE)
3. **Phase C — Conditional partitioning** (chỉ khi Phase A/B chưa đủ)

### 1.2 Design Rationale

Từ TDD §4.1, Store sử dụng `pgx/v5` qua `database/sql` wrapper. Connection pool hiện hardcoded `maxOpenConns = min(maxConns - reservedConns, 50)` trong `db_connection.go:243`. Điều này là bottleneck chính — không phải partitioning.

Từ Architecture L8, `db` table JOIN `instance` table cho mỗi `ListDatabases` call. Denormalize `workspace` + `engine` vào `db` table sẽ **loại bỏ JOIN** cho 90%+ queries.

---

## 2. Detailed Technical Design

### 2.1 Phase A — Workspace Denormalization + Index Optimization

#### 2.1.1 Migration Script

**File**: `backend/migrator/migration/<next_version>/0001_db_workspace_denorm.sql`

```sql
-- Step 1: Add workspace column (nullable first for safe rollout)
ALTER TABLE db ADD COLUMN IF NOT EXISTS workspace TEXT;

-- Step 2: Backfill from instance table
UPDATE db SET workspace = instance.workspace
FROM instance WHERE instance.resource_id = db.instance
AND db.workspace IS NULL;

-- Step 3: Set NOT NULL after backfill
ALTER TABLE db ALTER COLUMN workspace SET NOT NULL;
ALTER TABLE db ALTER COLUMN workspace SET DEFAULT '';

-- Step 4: Add engine column (avoid JOIN for engine filter)
ALTER TABLE db ADD COLUMN IF NOT EXISTS engine TEXT;

UPDATE db SET engine = instance.metadata->>'engine'
FROM instance WHERE instance.resource_id = db.instance
AND db.engine IS NULL;

-- Step 5: Create optimized indexes (CONCURRENTLY for zero downtime)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_workspace_not_deleted
    ON db (workspace) WHERE deleted = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_workspace_project_instance_name
    ON db (workspace, project, instance, name) WHERE deleted = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_instance_name
    ON db (instance, name);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_engine
    ON db (engine) WHERE deleted = false;
```

#### 2.1.2 Store Layer Changes

**File**: `backend/store/database.go`

Current `ListDatabases` (line 124-257) always JOINs `instance`:
```go
// BEFORE: Always JOIN
from.Space("LEFT JOIN instance ON db.instance = instance.resource_id")
```

Changed to conditional JOIN — only when needed:
```go
// AFTER: Conditional JOIN
needsInstanceJoin := false
if find.Engine != nil && find.Engine != nil {
    // engine now on db table, no JOIN needed
}
if find.Workspace != "" {
    // workspace now on db table, no JOIN needed
    where.And("db.workspace = ?", find.Workspace)
} else {
    // Runners use cross-workspace — still no JOIN since column exists
}

// Only JOIN instance when truly needed (e.g., instance-specific metadata)
if needsInstanceJoin {
    from.Space("LEFT JOIN instance ON db.instance = instance.resource_id")
}
```

**Key query change**:
```go
// BEFORE (line 160-162):
if v := find.Engine; v != nil {
    where.And("instance.metadata->>'engine' = ?", v.String())
}

// AFTER:
if v := find.Engine; v != nil {
    where.And("db.engine = ?", v.String())
}
```

#### 2.1.3 Trigger for Consistency

```sql
-- Trigger: Keep db.workspace in sync when instance.workspace changes
CREATE OR REPLACE FUNCTION sync_db_workspace()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.workspace IS DISTINCT FROM NEW.workspace THEN
        UPDATE db SET workspace = NEW.workspace
        WHERE instance = NEW.resource_id;
    END IF;
    IF OLD.metadata->>'engine' IS DISTINCT FROM NEW.metadata->>'engine' THEN
        UPDATE db SET engine = NEW.metadata->>'engine'
        WHERE instance = NEW.resource_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_instance_workspace_sync
    AFTER UPDATE ON instance
    FOR EACH ROW EXECUTE FUNCTION sync_db_workspace();
```

#### 2.1.4 `CreateDatabaseDefault` / `UpsertDatabase` Changes

**File**: `backend/store/database.go` line 260-338

```go
// In CreateDatabaseDefault — add workspace + engine from instance
func (s *Store) CreateDatabaseDefault(ctx context.Context, create *DatabaseMessage) (*DatabaseMessage, error) {
    // Lookup instance to get workspace + engine
    instance, err := s.GetInstanceByResourceID(ctx, create.InstanceID)
    if err != nil || instance == nil {
        return nil, errors.Errorf("instance %q not found", create.InstanceID)
    }

    q := qb.Q().Space(`
        INSERT INTO db (instance, project, name, deleted, workspace, engine)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT (instance, name) DO UPDATE SET deleted = EXCLUDED.deleted`,
        create.InstanceID, create.ProjectID, create.DatabaseName, false,
        instance.Workspace, instance.Metadata.GetEngine().String(),
    )
    // ...
}
```

### 2.2 Phase B — Effective Environment Materialization

#### 2.2.1 Migration

```sql
ALTER TABLE db ADD COLUMN IF NOT EXISTS effective_environment TEXT;

UPDATE db SET effective_environment = COALESCE(
    db.environment,
    (SELECT environment FROM instance WHERE resource_id = db.instance)
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_effective_env
    ON db (effective_environment) WHERE deleted = false;

-- Trigger: Keep effective_environment in sync
CREATE OR REPLACE FUNCTION sync_effective_environment()
RETURNS TRIGGER AS $$
BEGIN
    NEW.effective_environment := COALESCE(
        NEW.environment,
        (SELECT environment FROM instance WHERE resource_id = NEW.instance)
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_db_effective_env
    BEFORE INSERT OR UPDATE ON db
    FOR EACH ROW EXECUTE FUNCTION sync_effective_environment();

-- Also sync when instance environment changes
CREATE OR REPLACE FUNCTION sync_instance_env_to_db()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.environment IS DISTINCT FROM NEW.environment THEN
        UPDATE db SET effective_environment = COALESCE(db.environment, NEW.environment)
        WHERE instance = NEW.resource_id AND db.environment IS NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_instance_env_sync
    AFTER UPDATE ON instance
    FOR EACH ROW EXECUTE FUNCTION sync_instance_env_to_db();
```

#### 2.2.2 Store Layer Change

**File**: `backend/store/database.go` line 146-149

```go
// BEFORE:
if v := find.EffectiveEnvironmentID; v != nil {
    where.And(`COALESCE(db.environment, instance.environment) = ?`, *v)
}

// AFTER:
if v := find.EffectiveEnvironmentID; v != nil {
    where.And("db.effective_environment = ?", *v)
}
```

### 2.3 Phase C — Connection Pool Scaling

#### 2.3.1 Pool Manager

**File**: `backend/store/db_connection.go` (modify `createConnectionWithTracer`)

```go
func createConnectionWithTracer(ctx context.Context, pgURL string) (*sql.DB, error) {
    // ... existing pgx config ...

    db := stdlib.OpenDB(*pgxConfig)
    if err := db.PingContext(ctx); err != nil {
        db.Close()
        return nil, errors.Wrap(err, "failed to ping database")
    }

    // Dynamic pool sizing
    maxConns, reservedConns := getServerPoolLimits(ctx, db)
    availableConns := maxConns - reservedConns

    // Scale based on env var or auto-detect
    maxOpenConns := getConfiguredPoolSize(availableConns)

    db.SetMaxOpenConns(maxOpenConns)
    db.SetMaxIdleConns(maxOpenConns / 2)          // 50% idle
    db.SetConnMaxLifetime(30 * time.Minute)        // Recycle every 30min
    db.SetConnMaxIdleTime(5 * time.Minute)         // Close idle after 5min

    return db, nil
}

func getConfiguredPoolSize(availableConns int) int {
    // Check env var first
    if v := os.Getenv("PG_MAX_POOL_SIZE"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return min(n, availableConns)
        }
    }
    // Auto: use 80% of available, capped at 200
    target := availableConns * 80 / 100
    return max(min(target, 200), 10)
}
```

#### 2.3.2 Pool Metrics (Prometheus)

**File**: `backend/store/db_metrics.go` (new or extend)

```go
func (s *Store) RegisterPoolMetrics() {
    prometheus.MustRegister(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "bytebase_db_pool_open_connections",
            Help: "Number of open connections to metadata DB",
        },
        func() float64 {
            return float64(s.GetDB().Stats().OpenConnections)
        },
    ))
    // Similar for InUse, Idle, WaitCount, WaitDuration
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L8 (Store) | **HIGH** | `database.go` query changes, new columns, trigger maintenance |
| L10 (Infra) | **MEDIUM** | New migration scripts, pool configuration |
| L6 (Runner) | **LOW** | Schema syncer benefits from faster queries |
| L4 (Service) | **NONE** | API unchanged — store interface unchanged |
| L1 (Frontend) | **NONE** | No UI changes needed |

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
1. Deploy migration (ADD COLUMN, no constraints)
2. Run backfill (UPDATE ... SET workspace = ...)
3. Verify: SELECT COUNT(*) FROM db WHERE workspace IS NULL → 0
4. Deploy code: ALTER NOT NULL + new store queries
5. Create indexes CONCURRENTLY (no lock)
6. Create triggers
7. Monitor for 72h
8. Drop old index paths if unused
```

### 4.2 Rollback Plan

```
-- Rollback: revert store code to use JOINs
-- Columns remain (nullable), no data loss
-- Remove triggers
DROP TRIGGER IF EXISTS trg_instance_workspace_sync ON instance;
DROP TRIGGER IF EXISTS trg_db_effective_env ON db;
```

---

## 5. Performance Validation

### 5.1 Benchmark Queries

```sql
-- Before: ListDatabases with workspace filter
EXPLAIN ANALYZE SELECT ... FROM db
LEFT JOIN instance ON db.instance = instance.resource_id
WHERE instance.workspace = 'bank-a' AND db.deleted = false
ORDER BY db.project, db.instance, db.name
LIMIT 1001;

-- After: No JOIN needed
EXPLAIN ANALYZE SELECT ... FROM db
WHERE db.workspace = 'bank-a' AND db.deleted = false
ORDER BY db.project, db.instance, db.name
LIMIT 1001;
```

### 5.2 Expected Improvement

| Operation | Before (200K) | After (200K) | Improvement |
|-----------|---------------|--------------|-------------|
| ListDatabases (workspace) | ~80ms (JOIN + seq scan) | ~15ms (index scan) | **5x** |
| GetDatabase (cached) | ~0.5ms | ~0.5ms | Same |
| GetDatabase (uncached) | ~8ms (JOIN) | ~3ms (direct) | **2.5x** |
| Filter by engine | ~60ms (JSON extract) | ~10ms (column index) | **6x** |
| Filter by environment | ~50ms (COALESCE) | ~8ms (direct column) | **6x** |

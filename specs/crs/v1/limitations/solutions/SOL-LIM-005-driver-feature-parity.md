# Solution: CR-LIM-005 — Database Driver Feature Parity Enhancement

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-005                               |
| **Solution ID**| SOL-LIM-005                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L7 (Plugin Layer), L4 (Service Layer), L8 (Store) |
| **TDD Refs**   | §6 Plugin System Design, §8 Schema Migration Pipeline, §14 Trade-offs |

---

## 1. Solution Overview

### 1.1 Approach Summary

**Proposed Architecture Enhancement**: Introduce **Driver Capability Registry** (L7 extension) — a declarative system where each driver declares its capabilities, replacing implicit feature checks scattered across L4/L6.

3-phase approach:

1. **Phase A — Capability Registry + Feature Matrix API** (transparency)
2. **Phase B — SQL Advisor Expansion** (ClickHouse, BigQuery, Spanner, StarRocks)
3. **Phase C — PostgreSQL Online Schema Change + Parser Fixes** (gap closing)

### 1.2 Architectural Change Proposal

> **⚠️ PROPOSED ARCHITECTURE CHANGE — L7 Plugin Interface Extension**

**Current Architecture** (TDD §6.1):
```go
// Driver interface — flat, no capability declaration
type Driver interface {
    Open(...) (Driver, error)
    Close(ctx) error
    Ping(ctx) error
    Execute(ctx, statement, opts) (int64, error)
    QueryConn(ctx, conn, statement, queryContext) ([]*QueryResult, error)
    SyncInstance(ctx) (*InstanceMetadata, error)
    SyncDBSchema(ctx) (*DatabaseSchemaMetadata, error)
    Dump(ctx, out, dbMetadata) error
}
```

Feature support checks are **scattered** across codebase:
```go
// backend/common/engine.go — hardcoded engine checks
func EngineSupportPriorBackup(engine storepb.Engine) bool { ... }
func EngineSupportOnlineSchemaChange(engine storepb.Engine) bool { ... }

// backend/api/v1/sql_service.go — inline engine checks
if engine != storepb.Engine_POSTGRES && engine != storepb.Engine_MYSQL { ... }
```

**Proposed Architecture** — Capability Registry at L7:
```go
// Each driver declares its capabilities via Capabilities() method
type Driver interface {
    // ... existing methods ...
    Capabilities() DriverCapabilities
}

type DriverCapabilities struct {
    SQLAdvisor         bool     // Has SQL review rules
    AdvisorRuleCount   int      // Number of lint rules
    SchemaDump         DumpLevel  // Full, Partial, None
    PriorBackup        bool     // Supports pre-change backup
    OnlineSchemaChange bool     // Supports zero-downtime DDL
    DataMasking        MaskingLevel // Column, Document, None
    SchemaSync         bool     // Full schema introspection
    ChangeHistory      bool     // Tracks change history
    BatchQuery         bool     // Supports multi-db batch
    ReadOnlyConnection bool     // Supports read-only datasource
}
```

**Benefits**:
- **Single source of truth** — no more scattered `EngineSupportX()` functions
- **Runtime queryable** — API can expose capabilities per engine
- **Self-documenting** — new driver authors know exactly what to implement
- **Feature matrix UI** — auto-generated from registry

---

## 2. Detailed Technical Design

### 2.1 Phase A — Capability Registry + Feature Matrix

#### 2.1.1 Capability Types

**File**: `backend/plugin/db/capability.go` (new)

```go
// DumpLevel indicates the level of schema dump support.
type DumpLevel int
const (
    DumpNone    DumpLevel = iota // No schema dump
    DumpPartial                  // Partial metadata only
    DumpFull                     // Full schema DDL export
)

// MaskingLevel indicates data masking support.
type MaskingLevel int
const (
    MaskingNone     MaskingLevel = iota // No masking
    MaskingDocument                      // JSON path masking (NoSQL)
    MaskingColumn                        // Column-level masking (SQL)
)

// DriverCapabilities declares what features a driver supports.
// Returned by Driver.Capabilities() — each driver implementation fills this.
type DriverCapabilities struct {
    // Core
    SQLAdvisor         bool
    AdvisorRuleCount   int
    SchemaDump         DumpLevel
    PriorBackup        bool
    OnlineSchemaChange bool
    DataMasking        MaskingLevel
    SchemaSync         bool
    ChangeHistory      bool

    // Advanced
    BatchQuery         bool
    ReadOnlyConnection bool
    StreamingExport    bool

    // Parser info
    ParserEngine       string   // "antlr4", "custom", "none"
    KnownParserGaps    []string // Documented parser limitations
}

// CapabilityRegistry is a global registry mapping engine → capabilities.
// Populated at init() time by each driver.
var capabilityRegistry = map[storepb.Engine]DriverCapabilities{}

func RegisterCapabilities(engine storepb.Engine, caps DriverCapabilities) {
    capabilityRegistry[engine] = caps
}

func GetCapabilities(engine storepb.Engine) DriverCapabilities {
    return capabilityRegistry[engine]
}

func ListAllCapabilities() map[storepb.Engine]DriverCapabilities {
    return capabilityRegistry
}
```

#### 2.1.2 Driver Registration Pattern (Example: PostgreSQL)

**File**: `backend/plugin/db/pg/driver.go` (modify)

```go
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })

    // NEW: Declare capabilities
    db.RegisterCapabilities(storepb.Engine_POSTGRES, db.DriverCapabilities{
        SQLAdvisor:         true,
        AdvisorRuleCount:   200,
        SchemaDump:         db.DumpFull,
        PriorBackup:        true,
        OnlineSchemaChange: false, // Will be true after Phase C
        DataMasking:        db.MaskingColumn,
        SchemaSync:         true,
        ChangeHistory:      true,
        BatchQuery:         true,
        ReadOnlyConnection: true,
        StreamingExport:    true,
        ParserEngine:       "antlr4",
        KnownParserGaps:    nil,
    })
}
```

**File**: `backend/plugin/db/redis/driver.go` (modify)

```go
func init() {
    db.Register(storepb.Engine_REDIS, func() db.Driver { return &Driver{} })

    db.RegisterCapabilities(storepb.Engine_REDIS, db.DriverCapabilities{
        SQLAdvisor:         false,
        SchemaDump:         db.DumpNone,
        PriorBackup:        false,
        OnlineSchemaChange: false,
        DataMasking:        db.MaskingNone,
        SchemaSync:         true,
        ChangeHistory:      false,
        ParserEngine:       "none",
    })
}
```

#### 2.1.3 Refactor Scattered Checks

**File**: `backend/common/engine.go` (modify — deprecate hardcoded functions)

```go
// DEPRECATED: Use db.GetCapabilities(engine).PriorBackup instead
func EngineSupportPriorBackup(engine storepb.Engine) bool {
    return db.GetCapabilities(engine).PriorBackup
}

// DEPRECATED: Use db.GetCapabilities(engine).OnlineSchemaChange instead
func EngineSupportOnlineSchemaChange(engine storepb.Engine) bool {
    return db.GetCapabilities(engine).OnlineSchemaChange
}
```

#### 2.1.4 Capability API

**File**: `backend/api/v1/engine_service.go` (new)

```go
// EngineService exposes database engine capabilities.
func (s *EngineService) GetEngineCapabilities(
    ctx context.Context,
    req *connect.Request[v1pb.GetEngineCapabilitiesRequest],
) (*connect.Response[v1pb.GetEngineCapabilitiesResponse], error) {
    engine := req.Msg.Engine
    caps := db.GetCapabilities(engine)
    return connect.NewResponse(&v1pb.GetEngineCapabilitiesResponse{
        Engine:             engine,
        SqlAdvisor:         caps.SQLAdvisor,
        AdvisorRuleCount:   int32(caps.AdvisorRuleCount),
        SchemaDump:         convertDumpLevel(caps.SchemaDump),
        PriorBackup:        caps.PriorBackup,
        OnlineSchemaChange: caps.OnlineSchemaChange,
        DataMasking:        convertMaskingLevel(caps.DataMasking),
        SchemaSync:         caps.SchemaSync,
        ChangeHistory:      caps.ChangeHistory,
        KnownParserGaps:    caps.KnownParserGaps,
    }), nil
}

// List all engine capabilities (for feature matrix UI)
func (s *EngineService) ListEngineCapabilities(
    ctx context.Context,
    req *connect.Request[v1pb.ListEngineCapabilitiesRequest],
) (*connect.Response[v1pb.ListEngineCapabilitiesResponse], error) {
    all := db.ListAllCapabilities()
    var items []*v1pb.EngineCapabilityItem
    for engine, caps := range all {
        items = append(items, &v1pb.EngineCapabilityItem{
            Engine: engine,
            // ... convert caps ...
        })
    }
    return connect.NewResponse(&v1pb.ListEngineCapabilitiesResponse{Items: items}), nil
}
```

### 2.2 Phase B — SQL Advisor Expansion

#### 2.2.1 Advisor Rule Architecture (per engine)

Following existing pattern from TDD §6.2:

```
backend/plugin/advisor/
  ├── pg/          (200+ rules — existing)
  ├── mysql/       (200+ rules — existing)
  ├── tidb/        (existing)
  ├── oracle/      (existing)
  ├── mssql/       (existing)
  ├── snowflake/   (existing)
  ├── clickhouse/  (NEW — Phase B)
  ├── bigquery/    (NEW — Phase B)
  ├── spanner/     (NEW — Phase B)
  └── starrocks/   (NEW — Phase B)
```

#### 2.2.2 ClickHouse Advisor Rules

**File**: `backend/plugin/advisor/clickhouse/advisor.go` (new)

```go
func init() {
    advisor.Register(storepb.Engine_CLICKHOUSE, &ClickHouseAdvisor{})
}

type ClickHouseAdvisor struct{}

func (a *ClickHouseAdvisor) Check(ctx context.Context, req *advisor.CheckRequest) ([]*advisor.Advice, error) {
    var advices []*advisor.Advice

    // Parse ClickHouse SQL (ANTLR4 grammar)
    tree, err := parser.ParseClickHouseSQL(req.Statement)
    if err != nil {
        return nil, err
    }

    // Run rule checks
    for _, rule := range a.enabledRules(req.Config) {
        results := rule.Check(tree)
        advices = append(advices, results...)
    }
    return advices, nil
}
```

**Key ClickHouse rules** (30+ target):

| Category | Rule | Rationale |
|----------|------|-----------|
| Naming | `naming.table` | Enforce lowercase_snake_case |
| Engine | `schema.require-engine` | Every CREATE TABLE must specify engine |
| Engine | `schema.prefer-mergetree` | Warn if not using MergeTree family |
| Partition | `schema.partition-key` | Require partition key for large tables |
| Query | `query.no-select-star` | Forbid SELECT * (columnar penalty) |
| Query | `query.require-limit` | Require LIMIT on SELECT queries |
| Query | `query.no-join-on-distributed` | Warn about distributed table JOINs |
| Type | `type.prefer-low-cardinality` | Suggest LowCardinality for string enums |
| Type | `type.no-nullable-aggregate` | Nullable columns slow aggregation |
| Index | `index.order-by-usage` | Verify ORDER BY key aligns with queries |

#### 2.2.3 BigQuery Advisor Rules (25+ target)

**Key rules**:

| Category | Rule | Rationale |
|----------|------|-----------|
| Cost | `cost.no-select-star` | SELECT * scans all columns (cost) |
| Cost | `cost.require-partition-filter` | Must filter on partition column |
| Cost | `cost.prefer-clustering` | Suggest clustering keys |
| Schema | `schema.prefer-nested` | Prefer STRUCT over JOINs |
| Schema | `schema.partition-required` | Require time-based partitioning |
| Query | `query.no-cross-join` | Forbid CROSS JOIN (full scan) |

### 2.3 Phase C — PostgreSQL Online Schema Change

#### 2.3.1 pgroll Integration

**Proposed Architecture Change**: Add `pgroll` as a new L7 plugin component alongside `gh-ost` (MySQL).

```
backend/component/
  ├── ghost/    (MySQL online schema change — existing)
  └── pgroll/   (NEW — PostgreSQL online schema change)
```

**File**: `backend/component/pgroll/pgroll.go` (new)

```go
// PGRoll wraps the pgroll CLI for PostgreSQL online schema change.
// Similar pattern to ghost/ component for MySQL (TDD §8.2).
type PGRoll struct {
    binaryPath string // Path to pgroll binary
}

type MigrationConfig struct {
    DatabaseURL string
    Migration   PGRollMigration
    Complete    bool // false = start (expand), true = complete (contract)
}

// PGRollMigration defines the schema change in pgroll format
type PGRollMigration struct {
    Name       string                `json:"name"`
    Operations []PGRollOperation     `json:"operations"`
}

type PGRollOperation struct {
    Type   string      `json:"type"`   // "add_column", "alter_column", "create_index", "drop_column"
    Config interface{} `json:"config"`
}

func (p *PGRoll) Execute(ctx context.Context, cfg MigrationConfig) error {
    // Convert to pgroll JSON migration file
    migrationJSON, err := json.Marshal(cfg.Migration)
    if err != nil {
        return fmt.Errorf("marshal migration: %w", err)
    }

    tmpFile, _ := os.CreateTemp("", "pgroll-*.json")
    tmpFile.Write(migrationJSON)
    tmpFile.Close()
    defer os.Remove(tmpFile.Name())

    action := "start"
    if cfg.Complete {
        action = "complete"
    }

    cmd := exec.CommandContext(ctx, p.binaryPath,
        action,
        "--postgres-url", cfg.DatabaseURL,
        tmpFile.Name(),
    )
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("pgroll %s failed: %s: %w", action, output, err)
    }
    return nil
}

// ConvertDDLToPGRoll converts standard ALTER TABLE to pgroll operations.
func ConvertDDLToPGRoll(statement string, engine storepb.Engine) (*PGRollMigration, error) {
    if engine != storepb.Engine_POSTGRES {
        return nil, fmt.Errorf("pgroll only supports PostgreSQL")
    }

    tree, err := parser.ParsePostgreSQL(statement)
    if err != nil {
        return nil, err
    }

    // Convert parsed DDL to pgroll operations
    migration := &PGRollMigration{
        Name: fmt.Sprintf("migration_%d", time.Now().Unix()),
    }

    // Analyze ALTER TABLE statements
    for _, stmt := range tree.Statements {
        ops := convertAlterTableToPGRollOps(stmt)
        migration.Operations = append(migration.Operations, ops...)
    }

    return migration, nil
}
```

#### 2.3.2 Integration with TaskRun Executor

**File**: `backend/runner/taskrun/database_migrate_executor.go` (modify)

```go
func (e *DatabaseMigrateExecutor) RunOnce(ctx context.Context, driverCtx context.Context,
    task *store.TaskMessage, taskRunUID int64) (*storepb.TaskRunResult, error) {

    // Check if online schema change is requested and supported
    if task.Payload.OnlineSchemaChange && db.GetCapabilities(task.Engine).OnlineSchemaChange {
        switch task.Engine {
        case storepb.Engine_MYSQL:
            return e.runWithGhost(ctx, driverCtx, task, taskRunUID)  // existing
        case storepb.Engine_POSTGRES:
            return e.runWithPGRoll(ctx, driverCtx, task, taskRunUID) // NEW
        }
    }

    // Standard execution (existing path)
    return e.runStandard(ctx, driverCtx, task, taskRunUID)
}

func (e *DatabaseMigrateExecutor) runWithPGRoll(ctx context.Context, driverCtx context.Context,
    task *store.TaskMessage, taskRunUID int64) (*storepb.TaskRunResult, error) {

    // Convert DDL to pgroll migration
    migration, err := pgroll.ConvertDDLToPGRoll(task.Statement, task.Engine)
    if err != nil {
        // Fallback to standard execution if conversion fails
        slog.Warn("pgroll conversion failed, using standard DDL", "err", err)
        return e.runStandard(ctx, driverCtx, task, taskRunUID)
    }

    pgrollClient := pgroll.New(e.pgrollBinaryPath)

    // Phase 1: Start (expand) — creates new schema version
    e.logTaskRun(taskRunUID, "Starting online schema change (pgroll expand)...")
    if err := pgrollClient.Execute(ctx, pgroll.MigrationConfig{
        DatabaseURL: task.ConnectionURL,
        Migration:   *migration,
        Complete:    false,
    }); err != nil {
        return nil, fmt.Errorf("pgroll start: %w", err)
    }

    // Phase 2: Complete (contract) — drops old schema version
    e.logTaskRun(taskRunUID, "Completing online schema change (pgroll contract)...")
    if err := pgrollClient.Execute(ctx, pgroll.MigrationConfig{
        DatabaseURL: task.ConnectionURL,
        Migration:   *migration,
        Complete:    true,
    }); err != nil {
        return nil, fmt.Errorf("pgroll complete: %w", err)
    }

    return &storepb.TaskRunResult{Detail: "Online schema change completed via pgroll"}, nil
}
```

#### 2.3.3 Update PG Capabilities

```go
// After Phase C, update PG driver capabilities:
db.RegisterCapabilities(storepb.Engine_POSTGRES, db.DriverCapabilities{
    // ... existing ...
    OnlineSchemaChange: true, // NOW TRUE
})
```

### 2.4 Driver Conformance Test Suite

**File**: `backend/plugin/db/conformance_test.go` (new)

```go
// TestDriverConformance runs conformance tests against all registered drivers.
// Uses testcontainers to spin up actual database instances.
func TestDriverConformance(t *testing.T) {
    engines := db.ListRegisteredEngines()
    for _, engine := range engines {
        t.Run(engine.String(), func(t *testing.T) {
            caps := db.GetCapabilities(engine)
            container := startTestContainer(t, engine) // testcontainers
            driver := openTestDriver(t, engine, container)

            // Base conformance (all drivers)
            t.Run("Ping", func(t *testing.T) {
                require.NoError(t, driver.Ping(context.Background()))
            })
            t.Run("SyncInstance", func(t *testing.T) {
                meta, err := driver.SyncInstance(context.Background())
                require.NoError(t, err)
                require.NotNil(t, meta)
            })

            // Conditional tests based on capabilities
            if caps.SchemaDump == db.DumpFull {
                t.Run("Dump", func(t *testing.T) {
                    var buf bytes.Buffer
                    require.NoError(t, driver.Dump(context.Background(), &buf, nil))
                    require.NotEmpty(t, buf.String())
                })
            }
            if caps.SQLAdvisor {
                t.Run("Advisor", func(t *testing.T) {
                    adv := advisor.GetAdvisor(engine)
                    require.NotNil(t, adv)
                    results, err := adv.Check(context.Background(), &advisor.CheckRequest{
                        Statement: "SELECT * FROM test",
                    })
                    require.NoError(t, err)
                    require.NotEmpty(t, results) // SELECT * should trigger rule
                })
            }
        })
    }
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| **L7 (Plugin)** | **HIGH** | Capability registry, 4 new advisor packages, pgroll component |
| **L4 (Service)** | **MEDIUM** | New EngineService API, capability-based checks |
| **L6 (Runner)** | **LOW** | pgroll integration in TaskRun executor |
| L1 (Frontend) | **LOW** | Feature matrix UI component |
| L8 (Store) | **NONE** | No data model changes |

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
Phase A (Sprint 1-2):
  1. Add DriverCapabilities type + registry
  2. Register capabilities for all 22 existing drivers
  3. Refactor EngineSupportX() to use registry
  4. Add EngineService API + feature matrix endpoint
  5. Frontend: feature matrix component

Phase B (Sprint 3-6):
  6. ClickHouse advisor (30+ rules)
  7. BigQuery advisor (25+ rules)
  8. Spanner advisor (20+ rules)
  9. StarRocks advisor (15+ rules)
  Each: ANTLR grammar extension + rule implementation + tests

Phase C (Sprint 7-9):
  10. pgroll integration component
  11. TaskRun executor pgroll path
  12. Parser FIXME resolution (Oracle, Spanner, BigQuery)
  13. Conformance test suite
```

---

## 5. Configuration Reference

| Variable                 | Default   | Phase | Description                    |
|--------------------------|-----------|-------|--------------------------------|
| `PGROLL_BINARY_PATH`    | bundled   | C     | Path to pgroll binary          |
| `PGROLL_ENABLED`        | `false`   | C     | Enable PG online schema change |

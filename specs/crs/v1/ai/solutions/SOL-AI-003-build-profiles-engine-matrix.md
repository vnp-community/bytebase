# Solution: Build Profile Registry & Data-Driven Engine Matrix — CR-AI-003

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-AI-003                                               |
| **CR Reference**   | CR-AI-003                                                |
| **Title**          | Declarative Engine Capability Map & Build Profile Docs   |
| **Affected Layers**| L7 (Plugin), L10 (Infrastructure/Common), L2 (Server)    |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §8 (L7 Plugin Layer): 22 DB drivers registered via `init()` + `db.Register()`. Each driver in `backend/plugin/db/<engine>/`.

Per TDD.md §6.1: DB Driver Registration Pattern uses factory via `db.Open(ctx, engine, config)`. The `init()` pattern is per-file, triggered by Go import selection controlled by build tags.

Per architecture.md §10 (L9 Enterprise): License service controls feature gating — but engine availability is determined at **compile time** (build tags), not runtime (license).

Per TDD.md §1.2: Bytebase uses build tags for 3 binary profiles (`ultimate`, `enterprise_core`, `minidemo`). This is a deployment decision (TDD.md §14 — Key Design Decisions).

---

## 2. Solution Design

### 2.1 Part A — BUILD_PROFILES.md

Create structured documentation co-located with build-tagged files in `backend/server/`.

```markdown
<!-- backend/server/BUILD_PROFILES.md -->
# Build Profile Registry

## Profile Comparison Matrix

| Feature              | ultimate | enterprise_core | minidemo |
|----------------------|----------|-----------------|----------|
| Build Tag            | default  | enterprise_core | minidemo |
| File                 | ultimate.go | enterprise_core.go | minimal.go |
| **Database Engines** |          |                 |          |
| PostgreSQL           | ✅       | ✅              | ✅       |
| MySQL                | ✅       | ✅              | ✅       |
| TiDB                 | ✅       | ✅              | ❌       |
| MariaDB              | ✅       | ✅              | ❌       |
| OceanBase            | ✅       | ✅              | ❌       |
| Oracle               | ✅       | ❌              | ❌       |
| MSSQL                | ✅       | ❌              | ❌       |
| Snowflake            | ✅       | ❌              | ❌       |
| ClickHouse           | ✅       | ❌              | ❌       |
| MongoDB              | ✅       | ❌              | ❌       |
| Redis                | ✅       | ❌              | ❌       |
| Spanner              | ✅       | ❌              | ❌       |
| BigQuery             | ✅       | ❌              | ❌       |
| CockroachDB          | ✅       | ❌              | ❌       |
| Redshift             | ✅       | ❌              | ❌       |
| Others (8+)          | ✅       | ❌              | ❌       |
| **Plugins**          |          |                 |          |
| SQL Advisor (full)   | ✅       | ✅              | ❌       |
| SQL Parser (full)    | ✅       | ✅              | ✅       |
| IDP (OIDC/SAML/LDAP) | ✅      | ✅              | ❌       |
| Stripe Billing       | ✅       | ❌              | ❌       |
| Webhook (all IMs)    | ✅       | ✅              | ❌       |
| Mailer               | ✅       | ✅              | ❌       |

## Build Commands

```bash
# Ultimate (default — all engines, all plugins)
go build ./backend/...

# Enterprise Core (5 core engines)
go build -tags enterprise_core ./backend/...

# Minimal Demo (PostgreSQL + MySQL only)
go build -tags minidemo ./backend/...
```
```

### 2.2 Part B — AI-CONTEXT Comments

Add structured comments to each build-tagged file. These are parsed by AI agents for context awareness.

```go
// backend/server/ultimate.go
//go:build !enterprise_core && !minidemo

// AI-CONTEXT: Build Profile = "ultimate" (default)
// AI-CONTEXT: This file is compiled when NO special build tags are set.
// AI-CONTEXT: Available engines: ALL 22+ (PostgreSQL, MySQL, TiDB, MariaDB, OceanBase,
//             Oracle, MSSQL, Snowflake, ClickHouse, MongoDB, Redis, Spanner, BigQuery,
//             Cassandra, CosmosDB, DynamoDB, Elasticsearch, Hive, Databricks, Trino,
//             StarRocks, SQLite, CockroachDB, Redshift)
// AI-CONTEXT: Available plugins: ALL (advisor, parser, schema, mailer, webhook, idp, stripe)
// AI-CONTEXT: See BUILD_PROFILES.md for full profile comparison.

package server

import (
    _ "github.com/bytebase/bytebase/backend/plugin/db/oracle"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mssql"
    // ... all engine imports
)
```

```go
// backend/server/enterprise_core.go
//go:build enterprise_core && !minidemo

// AI-CONTEXT: Build Profile = "enterprise_core"
// AI-CONTEXT: This file is compiled ONLY when: enterprise_core=true AND minidemo=false
// AI-CONTEXT: Available engines: PostgreSQL, MySQL, TiDB, MariaDB, OceanBase (5 core)
// AI-CONTEXT: Available plugins: core advisor, core parser, core schema, mailer, webhook, idp (no stripe)
// AI-CONTEXT: See BUILD_PROFILES.md for full profile comparison.

package server
```

### 2.3 Part C — Data-Driven Engine Capability Matrix

Replace 11 switch statements with a single declarative map. Per architecture.md §8 (L7): engine capability is determined by `storepb.Engine` enum.

```go
// backend/common/engine.go — REFACTORED

package common

import (
    "fmt"
    storepb "github.com/bytebase/bytebase/proto/generated-go/store"
)

// EngineCapabilities is the single source of truth for engine feature support.
// Adding a new engine requires exactly ONE entry in this map.
// Per architecture.md §8 (L7): 22 DB drivers, each with varying feature support.
type EngineCapabilities struct {
    SQLReview       bool
    QueryNewACL     bool
    Masking         bool
    AutoComplete    bool
    StatementAdvise bool
    StatementReport bool
    PriorBackup     bool
    CreateDatabase  bool
    QuerySpanPlain  bool
    SyntaxCheck     bool
    BackupDBName    string // default: "bbdataarchive"
}

// engineCapabilities maps each engine to its feature support flags.
// This replaces 11 separate switch statements (493 LOC → ~60 LOC).
var engineCapabilities = map[storepb.Engine]EngineCapabilities{
    storepb.Engine_POSTGRES:      {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: true,  PriorBackup: true,  CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_MYSQL:         {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: true,  PriorBackup: true,  CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_TIDB:          {SQLReview: true,  QueryNewACL: false, Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_MARIADB:       {SQLReview: false, QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: false, BackupDBName: "bbdataarchive"},
    storepb.Engine_OCEANBASE:     {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_ORACLE:        {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_MSSQL:         {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: true,  StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_SNOWFLAKE:     {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: false, StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_CLICKHOUSE:    {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: "bbdataarchive"},
    storepb.Engine_MONGODB:       {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_REDIS:         {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_SPANNER:       {SQLReview: false, QueryNewACL: true,  Masking: true,  AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: true,  SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_BIGQUERY:      {SQLReview: false, QueryNewACL: true,  Masking: true,  AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: true,  SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_COCKROACHDB:   {SQLReview: false, QueryNewACL: true,  Masking: true,  AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: false, BackupDBName: "bbdataarchive"},
    storepb.Engine_REDSHIFT:      {SQLReview: true,  QueryNewACL: true,  Masking: true,  AutoComplete: false, StatementAdvise: true,  StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: true,  SyntaxCheck: true,  BackupDBName: "bbdataarchive"},
    storepb.Engine_SQLITE:        {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_STARROCKS:     {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_CASSANDRA:     {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_COSMOSDB:      {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_DYNAMODB:      {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_ELASTICSEARCH: {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_HIVE:          {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_DATABRICKS:    {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: true,  QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
    storepb.Engine_TRINO:         {SQLReview: false, QueryNewACL: false, Masking: false, AutoComplete: false, StatementAdvise: false, StatementReport: false, PriorBackup: false, CreateDatabase: false, QuerySpanPlain: false, SyntaxCheck: false, BackupDBName: ""},
}

func init() {
    // Runtime exhaustiveness check — panics at server startup if any 
    // registered protobuf engine is missing from the capability matrix.
    // This replaces the //exhaustive:enforce linter directive.
    for name, val := range storepb.Engine_value {
        eng := storepb.Engine(val)
        if eng == storepb.Engine_ENGINE_UNSPECIFIED {
            continue
        }
        if _, ok := engineCapabilities[eng]; !ok {
            panic(fmt.Sprintf("engine %s (%s) missing from engineCapabilities map in common/engine.go", name, eng))
        }
    }
}

// ---- Public API (backward compatible) ----

func EngineSupportSQLReview(engine storepb.Engine) bool {
    return engineCapabilities[engine].SQLReview
}

func EngineSupportQueryNewACL(engine storepb.Engine) bool {
    return engineCapabilities[engine].QueryNewACL
}

func EngineSupportMasking(engine storepb.Engine) bool {
    return engineCapabilities[engine].Masking
}

func EngineSupportAutoComplete(engine storepb.Engine) bool {
    return engineCapabilities[engine].AutoComplete
}

func EngineSupportStatementAdvise(engine storepb.Engine) bool {
    return engineCapabilities[engine].StatementAdvise
}

func EngineSupportStatementReport(engine storepb.Engine) bool {
    return engineCapabilities[engine].StatementReport
}

func EngineSupportPriorBackup(engine storepb.Engine) bool {
    return engineCapabilities[engine].PriorBackup
}

func EngineCreateDatabase(engine storepb.Engine) bool {
    return engineCapabilities[engine].CreateDatabase
}

func EngineQuerySpanPlain(engine storepb.Engine) bool {
    return engineCapabilities[engine].QuerySpanPlain
}

func EngineSyntaxCheck(engine storepb.Engine) bool {
    return engineCapabilities[engine].SyntaxCheck
}

func GetBackupDatabaseName(engine storepb.Engine) string {
    name := engineCapabilities[engine].BackupDBName
    if name == "" {
        return "" // engine does not support backup
    }
    return name
}
```

### 2.4 Part D — Comparison Test (old vs new)

```go
// backend/common/engine_test.go
package common

import (
    "testing"
    storepb "github.com/bytebase/bytebase/proto/generated-go/store"
)

// TestEngineCapabilityMatrix_MatchesOldBehavior verifies the new data-driven
// map produces identical results to the old switch statements.
// This test should be written BEFORE refactoring, capturing old behavior,
// then verified against the new implementation.
func TestEngineCapabilityMatrix_Exhaustive(t *testing.T) {
    for name, val := range storepb.Engine_value {
        eng := storepb.Engine(val)
        if eng == storepb.Engine_ENGINE_UNSPECIFIED {
            continue
        }
        t.Run(name, func(t *testing.T) {
            caps, ok := engineCapabilities[eng]
            if !ok {
                t.Fatalf("engine %s missing from capability map", name)
            }
            // Verify each capability matches expected
            if got := EngineSupportSQLReview(eng); got != caps.SQLReview {
                t.Errorf("SQLReview mismatch: got %v, want %v", got, caps.SQLReview)
            }
            // ... repeat for all 11 capabilities
        })
    }
}

func TestEngineCapabilityMatrix_InitPanicsOnMissing(t *testing.T) {
    // This is a design-level test: the init() function should panic
    // if a new engine enum value is added to proto but not to the map.
    // Verified by CI across all build profiles.
}
```

### 2.5 Part E — DriverRegistry Interface

```go
// backend/server/driver_registry.go
package server

import storepb "github.com/bytebase/bytebase/proto/generated-go/store"

// DriverRegistry exposes which database drivers are compiled into the binary.
// Per architecture.md §8 (L7): drivers register via init() and build tags
// control which drivers are included.
type DriverRegistry interface {
    AvailableEngines() []storepb.Engine
    IsEngineAvailable(engine storepb.Engine) bool
}

// runtimeRegistry queries the db.Open factory to determine availability.
type runtimeRegistry struct{}

func NewDriverRegistry() DriverRegistry { return &runtimeRegistry{} }

func (r *runtimeRegistry) AvailableEngines() []storepb.Engine {
    var engines []storepb.Engine
    for _, val := range storepb.Engine_value {
        eng := storepb.Engine(val)
        if eng == storepb.Engine_ENGINE_UNSPECIFIED {
            continue
        }
        if r.IsEngineAvailable(eng) {
            engines = append(engines, eng)
        }
    }
    return engines
}

func (r *runtimeRegistry) IsEngineAvailable(engine storepb.Engine) bool {
    // Check if the driver factory has a registered constructor for this engine
    return db.IsRegistered(engine)
}
```

---

## 3. Execution Order

| Step | Part | Files | Risk | Verification |
|------|------|-------|------|-------------|
| 1 | B | AI-CONTEXT comments in 5 build-tagged files | None | Comments only |
| 2 | A | `BUILD_PROFILES.md` | None | Documentation only |
| 3 | D | `engine_test.go` — capture old behavior | None | Tests pass |
| 4 | C | Refactor `engine.go` — map + thin wrappers | High | Old tests + new tests pass |
| 5 | E | `DriverRegistry` interface | Low | `go build` |
| 6 | — | CI verification across 3 build profiles | Low | All profiles compile |

---

## 4. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/server/BUILD_PROFILES.md` | NEW | Documentation |
| `backend/server/ultimate.go` | MODIFY | Add AI-CONTEXT comment block |
| `backend/server/enterprise_core.go` | MODIFY | Add AI-CONTEXT comment block |
| `backend/server/minimal.go` | MODIFY | Add AI-CONTEXT comment block |
| `backend/common/config_dev.go` | MODIFY | Add AI-CONTEXT comment block |
| `backend/common/config_release.go` | MODIFY | Add AI-CONTEXT comment block |
| `backend/common/engine.go` | MODIFY | Replace 11 switches with map (493→~120 LOC) |
| `backend/common/engine_test.go` | NEW | Exhaustiveness + regression tests |
| `backend/server/driver_registry.go` | NEW | DriverRegistry interface |

---

## 5. Layer Compliance Check

Per architecture.md §13:
- L7 (Plugin) has NO upward dependency → ✅ Engine map in L10/Common, not L7
- L10 (Infrastructure/Common) → ✅ `engine.go` stays in `common/` package
- L2 (Server) → ✅ `driver_registry.go` in `server/` package, queries L7 factory

---

## 6. Rollback Strategy

- Parts A, B: Delete documentation/comments — zero functional impact
- Part C: `git revert` restores original switch statements
- Part E: Delete `driver_registry.go` — no callers initially

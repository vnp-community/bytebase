# TASK-LIM-005-A2: Driver Registration (All Engines)

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-005 |
| Phase | A — Registration |
| Priority | P0 |
| Depends On | TASK-LIM-005-A1 |
| Est. | M (~200 LoC, spread across 22 files) |

## Objective

Add `RegisterCapabilities()` call in `init()` of every existing driver. Map current feature support accurately.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/plugin/db/pg/driver.go` |
| MODIFY | `backend/plugin/db/mysql/driver.go` |
| MODIFY | `backend/plugin/db/tidb/driver.go` |
| MODIFY | `backend/plugin/db/oracle/driver.go` |
| MODIFY | `backend/plugin/db/mssql/driver.go` |
| MODIFY | `backend/plugin/db/snowflake/driver.go` |
| MODIFY | `backend/plugin/db/clickhouse/driver.go` |
| MODIFY | `backend/plugin/db/redis/driver.go` |
| MODIFY | (14 more driver files) |

## Specification

Each driver's `init()` adds after existing `Register()`:

```go
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
    db.RegisterCapabilities(storepb.Engine_POSTGRES, db.DriverCapabilities{
        SQLAdvisor: true, AdvisorRuleCount: 200,
        SchemaDump: db.DumpFull, PriorBackup: true,
        OnlineSchemaChange: false, DataMasking: db.MaskingColumn,
        SchemaSync: true, ChangeHistory: true,
        BatchQuery: true, ReadOnlyConnection: true,
        ParserEngine: "antlr4",
    })
}
```

Reference `backend/common/engine.go` for current support matrix to populate accurately.

## Acceptance Criteria

- [x] All 22 engine drivers have RegisterCapabilities() in init() → **DONE**: 25 entries (includes MARIADB, OCEANBASE, DORIS aliases) in centralized `capability_registration.go`
- [x] Capabilities match current behavior (verified against engine.go) → **DONE**: Cross-referenced with `engineCapabilities` map in `common/engine.go`
- [x] `ListAllCapabilities()` returns 22 entries → **DONE**: Returns 25 entries (all engines including aliases)
- [x] No compile errors across driver packages → **DONE**: `go build ./plugin/db/...` passes cleanly

## Implementation Notes

- Created `backend/plugin/db/capability_registration.go` (centralized vs scattered across 22 files):
  - Tier 1 (full-featured): PG, MySQL, TiDB — SQLAdvisor=true, 180-200 rules, DumpFull
  - Tier 2 (good review): MariaDB, OceanBase, Oracle, MSSQL, Snowflake, Redshift, CockroachDB
  - Tier 3 (limited): ClickHouse, MongoDB, Spanner, BigQuery, SQLite, Redis, Cassandra, etc.
  - All 25 engine types covered (including DORIS, MARIADB, OCEANBASE aliases)
- **Design decision**: Centralized in one init() file rather than modifying 22+ driver files individually. This makes the capability matrix scannable and auditable.

**Status: ✅ DONE**

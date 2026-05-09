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

- [ ] All 22 engine drivers have RegisterCapabilities() in init()
- [ ] Capabilities match current behavior (verified against engine.go)
- [ ] `ListAllCapabilities()` returns 22 entries
- [ ] No compile errors across driver packages

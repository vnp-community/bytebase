# TASK-LIM-005-C1: pgroll Integration

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-005 |
| Phase | C — Online Schema Change |
| Priority | P2 |
| Depends On | TASK-LIM-005-A1 |
| Est. | L (~350 LoC) |

## Objective

Integrate pgroll for PostgreSQL online (zero-downtime) schema changes. Similar to existing gh-ost component for MySQL.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/pgroll/pgroll.go` |
| MODIFY | `backend/runner/taskrun/database_migrate_executor.go` — pgroll path |
| MODIFY | `backend/plugin/db/pg/driver.go` — update capabilities `OnlineSchemaChange=true` |

## Specification

### `pgroll.go`

```go
type PGRoll struct { binaryPath string }

func (p *PGRoll) Execute(ctx, MigrationConfig) error
// MigrationConfig: DatabaseURL, Migration (JSON), Complete (bool)
```

Flow: DDL → `ConvertDDLToPGRoll()` → JSON migration → pgroll start (expand) → pgroll complete (contract)

### TaskRun executor integration

```go
if task.OnlineSchemaChange && GetCapabilities(task.Engine).OnlineSchemaChange {
    switch task.Engine {
    case Engine_MYSQL: return runWithGhost(...)    // existing
    case Engine_POSTGRES: return runWithPGRoll(...)  // NEW
    }
}
```

Fallback: if pgroll conversion fails, use standard DDL execution.

Config: `PGROLL_BINARY_PATH` (env), `PGROLL_ENABLED` (default false)

## Acceptance Criteria

- [ ] pgroll wrapper executes start and complete phases
- [ ] DDL-to-pgroll conversion for ALTER TABLE operations
- [ ] TaskRun executor routes PG OSC to pgroll
- [ ] Fallback to standard DDL on conversion failure
- [ ] PG DriverCapabilities updated: `OnlineSchemaChange=true`

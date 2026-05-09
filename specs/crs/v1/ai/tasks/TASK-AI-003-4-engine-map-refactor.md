# TASK-AI-003-4: engine.go Map Refactor (11 switches → map)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | TASK-AI-003-3 |
| Est. | M (replace 493 LOC with ~120 LOC) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Replace 11 separate switch statements in `backend/common/engine.go` with a single `EngineCapabilities` struct and `map[storepb.Engine]EngineCapabilities`. Add `init()` exhaustiveness check.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/common/engine.go` — replace switches with map + thin wrappers |

## Specification

### Step 1: Define struct

```go
type EngineCapabilities struct {
    SQLReview, QueryNewACL, Masking, AutoComplete    bool
    StatementAdvise, StatementReport, PriorBackup    bool
    CreateDatabase, QuerySpanPlain, SyntaxCheck      bool
    BackupDBName string
}
```

### Step 2: Define map

```go
var engineCapabilities = map[storepb.Engine]EngineCapabilities{
    storepb.Engine_POSTGRES: {SQLReview: true, QueryNewACL: true, ...},
    // ... one entry per engine
}
```

### Step 3: Add init() exhaustiveness check

```go
func init() {
    for name, val := range storepb.Engine_value {
        eng := storepb.Engine(val)
        if eng == storepb.Engine_ENGINE_UNSPECIFIED { continue }
        if _, ok := engineCapabilities[eng]; !ok {
            panic(fmt.Sprintf("engine %s missing from engineCapabilities", name))
        }
    }
}
```

### Step 4: Replace functions with thin map lookups

```go
func EngineSupportSQLReview(engine storepb.Engine) bool {
    return engineCapabilities[engine].SQLReview
}
```

### Verification

```bash
go test ./backend/common/... -run TestEngine -v -count=1
go build ./backend/...
go build -tags enterprise_core ./backend/...
go build -tags minidemo ./backend/...
```

## Acceptance Criteria

- [ ] All 11 switch statements replaced with map lookups
- [ ] `init()` panics if engine missing from map
- [ ] engine_test.go (from 003-3) passes unchanged
- [ ] All 3 build profiles compile
- [ ] LOC reduced from ~493 to ~120

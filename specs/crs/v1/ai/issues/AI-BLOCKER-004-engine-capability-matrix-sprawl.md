# AI-BLOCKER-004: Engine Capability Matrix Sprawl in `common/engine.go`

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-004 |
| **Severity** | 🟠 High |
| **Category** | Code Duplication / Maintenance Risk |
| **Layer** | L6 Common (`backend/common/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

`common/engine.go` (493 LOC) contains 11 nearly identical exhaustive switch statements, each mapping the same 22 `storepb.Engine` values to `true/false` for different capabilities. When a new database engine is added, all 11 functions must be updated simultaneously. AI agents editing one function are likely to miss others, causing inconsistent capability maps.

## Impact on AI Operations

- **Partial Update Hazard**: AI adds `storepb.Engine_QUESTDB` to `EngineSupportSQLReview()` but forgets to add it to `EngineSupportMasking()`, `EngineSupportAutoComplete()`, etc. The `//exhaustive:enforce` linter catches missing cases but only for the function being edited.
- **Context Waste**: 493 lines of repetitive switch statements consume ~3K tokens of context for no informational value beyond a simple lookup table.
- **Error Amplification**: Each switch has a `default: return false` clause that silently swallows new engines — AI-added engines work in some capabilities but fail in others without clear errors.

## Evidence

```go
// 11 functions with identical structure:
func EngineSupportSQLReview(engine storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportQueryNewACL(engine storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportMasking(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportAutoComplete(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportStatementAdvise(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportStatementReport(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportPriorBackup(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportCreateDatabase(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportQuerySpanPlainField(e storepb.Engine) bool { /* 22-way switch */ }
func EngineSupportSyntaxCheck(e storepb.Engine) bool { /* 22-way switch */ }
func BackupDatabaseNameOfEngine(e storepb.Engine) string { /* 22-way switch */ }
```

## Recommended Remediation

1. **Data-Driven Capability Matrix**: Replace 11 switch statements with a single declarative map:
   ```go
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
       BackupDBName    string
   }
   
   var engineCapabilities = map[storepb.Engine]EngineCapabilities{
       storepb.Engine_POSTGRES: {SQLReview: true, QueryNewACL: true, Masking: true, ...},
       storepb.Engine_MYSQL:    {SQLReview: true, QueryNewACL: true, Masking: true, ...},
       // ... one line per engine
   }
   ```

2. **Benefits**:
   - Adding a new engine = 1 line change instead of 11
   - AI can read the entire capability matrix in ~30 tokens instead of ~3000
   - Compile-time exhaustiveness via `init()` validation

## Files to Modify

```
backend/common/engine.go → refactor to data-driven map
```

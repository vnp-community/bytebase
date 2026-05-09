# TASK-AI-004-2: Parse + Format Functions

| Field | Value |
|-------|-------|
| Solution | SOL-AI-004 |
| Priority | P1 |
| Depends On | TASK-AI-004-1 |
| Est. | M (~250 LoC code + ~200 LoC tests) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Implement `Parse*Ref()` functions and `String()` methods for all 12 resource ref types. Include comprehensive tests.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/common/resource_parser.go` — parse + format functions |
| CREATE | `backend/common/resource_ref_test.go` — table-driven tests |

## Specification

One parse function per ref type:

```go
func ParseProjectRef(name string) (*ProjectRef, error)
func ParsePlanRef(name string) (*PlanRef, error)
func ParseTaskRunRef(name string) (*TaskRunRef, error)
func ParseDatabaseRef(name string) (*DatabaseRef, error)
// ... etc
```

Each parse function includes the expected format in error messages. Each ref type implements `String()` for round-trip support.

### Tests (table-driven)

```go
func TestParsePlanRef(t *testing.T) {
    tests := []struct{ input string; wantProject string; wantUID int64; wantErr bool }{
        {"projects/p1/plans/123", "p1", 123, false},
        {"projects/p1/plans/abc", "", 0, true},  // non-numeric UID
        {"invalid", "", 0, true},
    }
    // ...
}
```

### Verification

```bash
go test ./backend/common/... -run TestParse -v -count=1
```

## Acceptance Criteria

- [ ] 12 parse functions implemented
- [ ] 12 String() methods implemented
- [ ] Tests cover valid, invalid, and edge cases
- [ ] Round-trip: `Parse(ref.String()) == ref` for all types

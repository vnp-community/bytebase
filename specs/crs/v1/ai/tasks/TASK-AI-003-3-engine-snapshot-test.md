# TASK-AI-003-3: Engine Capability Snapshot Test

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~80 LoC test file) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Write `engine_test.go` that captures the current behavior of all 11 engine capability functions BEFORE refactoring. This is the safety net for TASK-AI-003-4.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/common/engine_test.go` |

## Specification

```go
func TestEngineCapabilityMatrix_Exhaustive(t *testing.T) {
    for name, val := range storepb.Engine_value {
        eng := storepb.Engine(val)
        if eng == storepb.Engine_ENGINE_UNSPECIFIED { continue }
        t.Run(name, func(t *testing.T) {
            // Call all 11 functions and record results
            _ = EngineSupportSQLReview(eng)
            _ = EngineSupportQueryNewACL(eng)
            _ = EngineSupportMasking(eng)
            // ... all 11 functions
        })
    }
}
```

**Critical**: Run this test and save output BEFORE applying 003-4. After 003-4, the same test must produce identical results.

### Verification

```bash
go test ./backend/common/... -run TestEngine -v -count=1
```

## Acceptance Criteria

- [ ] Test covers all engine enum values
- [ ] Test calls all 11 capability functions
- [ ] All tests pass on current code (baseline)

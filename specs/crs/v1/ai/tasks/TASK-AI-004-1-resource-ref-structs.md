# TASK-AI-004-1: Typed Resource Ref Structs

| Field | Value |
|-------|-------|
| Solution | SOL-AI-004 |
| Priority | P1 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-10 |
| Est. | S (~120 LoC) |

## Objective

Define typed struct definitions for all AIP resource name patterns used by Bytebase.

## Delivered

**File**: `backend/common/resource_ref.go` (241 lines)

12 typed structs with godoc and AIP patterns:

| Struct | Fields |
|--------|--------|
| `ProjectRef` | ProjectID |
| `PlanRef` | ProjectID, PlanUID |
| `IssueRef` | ProjectID, IssueUID |
| `ReleaseRef` | ProjectID, ReleaseUID |
| `RolloutRef` | ProjectID, RolloutUID |
| `StageRef` | ProjectID, RolloutUID, StageID |
| `TaskRef` | ProjectID, RolloutUID, StageID, TaskUID |
| `TaskRunRef` | ProjectID, RolloutUID, StageID, TaskUID, TaskRunUID |
| `InstanceRef` | InstanceID |
| `DatabaseRef` | InstanceID, DatabaseName |
| `SettingRef` | SettingName |
| `UserRef` | UserUID |

Also includes 8 `Parse*Ref()` functions and 12 `String()` methods (co-located in same file).

### Verification (2026-05-10 re-verified)

```bash
go build ./backend/common/...   # ✅ PASS
go vet ./backend/common/...     # ✅ PASS
go test ./backend/common/... -run 'TestParse|TestResource' -count=1  # ✅ PASS
```

## Acceptance Criteria

- [x] 12 ref structs defined
- [x] Each has godoc with AIP pattern
- [x] 8 parse functions + 12 String() methods
- [x] `go build` passes
- [x] No callers — additive only

# TASK-AI-004-1: Typed Resource Ref Structs

| Field | Value |
|-------|-------|
| Solution | SOL-AI-004 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~120 LoC) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Define typed struct definitions for all AIP resource name patterns used by Bytebase.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/common/resource_ref.go` |

## Specification

Define 12 structs modeling the resource hierarchy:

```go
type ProjectRef struct { ProjectID string }
type PlanRef struct { ProjectID string; PlanUID int64 }
type IssueRef struct { ProjectID string; IssueUID int64 }
type ReleaseRef struct { ProjectID string; ReleaseUID int64 }
type RolloutRef struct { ProjectID string; RolloutUID int64 }
type StageRef struct { ProjectID string; RolloutUID int64; StageID string }
type TaskRef struct { ProjectID string; RolloutUID int64; StageID string; TaskUID int64 }
type TaskRunRef struct { ProjectID string; RolloutUID int64; StageID string; TaskUID int64; TaskRunUID int64 }
type InstanceRef struct { InstanceID string }
type DatabaseRef struct { InstanceID string; DatabaseName string }
type SettingRef struct { SettingName string }
type UserRef struct { UserUID string }
```

Each struct with godoc including the AIP pattern (e.g., `"projects/{project}/plans/{planUID}"`).

### Verification

```bash
go build ./backend/common/...
```

## Acceptance Criteria

- [ ] 12 ref structs defined
- [ ] Each has godoc with AIP pattern
- [ ] `go build` passes
- [ ] No callers — additive only

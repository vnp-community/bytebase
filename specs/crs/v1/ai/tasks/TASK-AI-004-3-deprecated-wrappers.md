# TASK-AI-004-3: Backward-Compatible Deprecated Wrappers

| Field | Value |
|-------|-------|
| Solution | SOL-AI-004 |
| Priority | P1 |
| Depends On | TASK-AI-004-2 |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | S |

## Objective

Add `// Deprecated:` annotations to existing resource name functions and rewire them to delegate to the new typed Parse*Ref parsers.

## Functions Migrated

| Old Function | New Delegate | Delegation |
|---|---|---|
| `GetProjectID` | `ParseProjectRef` | `ref.ProjectID` |
| `GetInstanceID` | `ParseInstanceRef` | `ref.InstanceID` |
| `GetInstanceDatabaseID` | `ParseDatabaseResourceRef` | `ref.InstanceID, ref.DatabaseName` |
| `GetUserEmail` | `ParseUserResourceRef` | `ref.Email` |
| `GetSettingName` | `ParseSettingRef` | `ref.SettingName` |
| `GetProjectIDIssueUID` | `ParseIssueResourceRef` | `ref.ProjectID, ref.IssueUID` |
| `GetProjectIDPlanID` | `ParseResourcePlanRef` | `ref.ProjectID, ref.PlanUID` |

**7 functions** deprecated and rewired. Remaining functions without typed equivalents left unchanged.

## Verification

```bash
go build ./backend/common/...  # ✅ PASS
go build ./backend/api/v1/...  # ✅ PASS
go test ./backend/common/...   # ✅ PASS (all tests pass)
```

## Acceptance Criteria

- [x] All functions with typed equivalents have `// Deprecated:` annotation
- [x] All functions delegate to new Parse*Ref() internally
- [x] Zero behavioral change — all existing tests pass
- [x] `go build` passes across consumer packages

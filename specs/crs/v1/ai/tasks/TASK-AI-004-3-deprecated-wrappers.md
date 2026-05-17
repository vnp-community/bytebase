# TASK-AI-004-3: Backward-Compatible Deprecated Wrappers

| Field | Value |
|-------|-------|
| Solution | SOL-AI-004 |
| Priority | P1 |
| Depends On | TASK-AI-004-2 |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-10 |
| Est. | S |

## Objective

Add `// Deprecated:` annotations to existing resource name functions and rewire them to delegate to the new typed Parse*Ref parsers.

## Delivered

**7 functions** deprecated and rewired in `backend/common/resource_name.go`:

| Old Function | New Delegate | Delegation |
|---|---|---|
| `GetProjectID` | `ParseProjectRef` | `ref.ProjectID` |
| `GetInstanceID` | `ParseInstanceRef` | `ref.InstanceID` |
| `GetInstanceDatabaseID` | `ParseDatabaseResourceRef` | `ref.InstanceID, ref.DatabaseName` |
| `GetUserEmail` | `ParseUserResourceRef` | `ref.Email` |
| `GetSettingName` | `ParseSettingRef` | `ref.SettingName` |
| `GetProjectIDIssueUID` | `ParseIssueResourceRef` | `ref.ProjectID, ref.IssueUID` |
| `GetProjectIDPlanID` | `ParseResourcePlanRef` | `ref.ProjectID, ref.PlanUID` |

Remaining functions without typed equivalents left unchanged.

### Verification (2026-05-10 re-verified)

```bash
go build ./backend/common/...  # ✅ PASS
go build ./backend/api/v1/...  # ✅ PASS
go test ./backend/common/...   # ✅ PASS (all tests pass, 0.767s)
```

## Acceptance Criteria

- [x] All functions with typed equivalents have `// Deprecated:` annotation (7 total)
- [x] All functions delegate to new Parse*Ref() internally
- [x] Zero behavioral change — all existing tests pass
- [x] `go build` passes across consumer packages

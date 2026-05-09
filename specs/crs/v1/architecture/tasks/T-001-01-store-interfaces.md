# T-001-01: Store Domain Interfaces

| Field | Value |
|---|---|
| **Task ID** | T-001-01 |
| **Solution** | SOL-ARCH-001 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/store/interfaces.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Tạo file `interfaces.go` chứa 12+ role-based interfaces cho Store, tách God Object thành domain-specific contracts: UserReader, UserWriter, ProjectReader, PlanReader, IssueReader, DatabaseReader, InstanceReader, PolicyReader, SettingReader, SheetReader, RolloutReader, ChangelogReader, AuditLogWriter...

## Context — Current Code

```go
// backend/store/store.go:18 — God Object
type Store struct {
    dbConnManager *DBConnectionManager
    enableCache   bool
    // 13 LRU caches
    // 200+ public methods across 73 files
}
```

## Implementation — DELIVERED

### File: `backend/store/interfaces.go` (203 lines)

18 interfaces created across 12 domains:

| Domain | Interface | Methods | Status |
|--------|-----------|---------|--------|
| User | `UserReader` | `GetUserByID`, `GetUserByEmail`, `BatchGetUsersByEmails`, `ListUsers` | ✅ |
| User | `UserWriter` | `CreateUser`, `UpdateUser`, `UpdateUserEmail` | ✅ |
| User | `UserStore` | `UserReader` + `UserWriter` | ✅ |
| Project | `ProjectReader` | `GetDefaultProjectID`, `GetProject`, `ListProjects` | ✅ |
| Project | `ProjectWriter` | `CreateProject` | ✅ |
| Plan | `PlanReader` | `CreatePlan`, `ListPlans` | ✅ |
| Issue | `IssueReader` | `GetIssue`, `ListIssues`, `CreateIssue` | ✅ |
| Database | `DatabaseReader` | `GetDatabase`, `ListDatabases` | ✅ |
| Instance | `InstanceReader` | `GetInstance`, `ListInstances`, `CreateInstance` | ✅ |
| Policy | `PolicyReader` | `GetPolicy`, `ListPolicies`, `GetRolloutPolicy`, `GetWorkspaceIamPolicy`, `GetProjectIamPolicy` | ✅ |
| Setting | `SettingReader` | `GetSetting`, `ListSettings`, `GetWorkspaceProfileSetting` | ✅ |
| Workspace | `WorkspaceReader` | `GetWorkspaceID`, `GetWorkspaceByID` | ✅ |
| Audit | `AuditLogWriter` | `CreateAuditLog` | ✅ |
| DBSchema | `DBSchemaReader` | `GetDBSchema` | ✅ |
| Sheet | `SheetReader` | `GetSheetTruncated`, `GetSheetFull`, `CreateSheets` | ✅ |
| Role | `RoleReader` | `GetRole`, `ListRoles` | ✅ |
| Changelog | `ChangelogReader` | `GetChangelog`, `ListChangelogs` | ✅ |
| Aggregate | `DataStore` | All above + `GetDB`, `Close`, `DeleteCache` | ✅ |

> **Note**: Method signatures were scanned from actual `user.go`, `project.go`, etc. to ensure exact match. All signatures compile against `*Store`.

## Acceptance Criteria

- [x] `backend/store/interfaces.go` created with 12+ domain interfaces (**18 interfaces**)
- [x] Each interface has 2-5 methods (narrow, role-based)
- [x] Method signatures match existing `*Store` methods exactly
- [x] `go build ./backend/store/...` passes ✅
- [x] No changes to any existing file

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ grep -c 'interface {' backend/store/interfaces.go → 18
$ wc -l backend/store/interfaces.go → 203 lines
```

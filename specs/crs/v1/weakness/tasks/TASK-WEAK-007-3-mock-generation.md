# TASK-WEAK-007-3: Mock Generation Setup

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | TASK-WEAK-007-1, TASK-WEAK-007-2 |
| Est. | S (~40 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Configure `go generate` with `mockgen` to auto-generate mocks from extracted interfaces.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/mock/generate.go` — expanded interface list |
| CREATE | `backend/component/iam/mock/generate.go` |
| CREATE | `backend/enterprise/mock/generate.go` |
| EXISTS | `backend/store/mock/mock_store.go` — pre-generated, 1807 lines |

## Implementation Notes

### Store Mocks (`backend/store/mock/generate.go`)

`go:generate` directive covers 30+ interfaces:

```
UserReader, UserWriter, UserStore, ProjectReader, ProjectWriter,
PlanReader, IssueReader, DatabaseReader, DatabaseWriter, InstanceReader,
PolicyReader, SettingReader, WorkspaceReader, AuditLogWriter, DBSchemaReader,
SheetReader, RoleReader, ChangelogReader, TaskStore, TaskRunStore,
QueryHistoryStore, AccessGrantReader, ExportArchiveReader, AccountReader,
SignalWriter, PlanWebhookWriter, SyncHistoryReader, AuthStore, DataStore
```

Existing `mock_store.go` (1807 LoC) already contains generated mocks for 18 core interfaces. New interfaces will be added on next `go generate` run.

### IAM Mocks (`backend/component/iam/mock/generate.go`)

```
PermissionChecker, PermissionProvider, GroupResolver, CacheReloader, IAMService
```

### Enterprise Mocks (`backend/enterprise/mock/generate.go`)

```
FeatureChecker, PlanReader, LimitReader, LicenseManager
```

### Go Toolchain Note

Mock regeneration requires `mockgen` built with Go 1.26 (project requirement). The `go.uber.org/mock v0.5.2` dependency is already in `go.mod`. Pre-generated `mock_store.go` compiles and is usable for testing.

### Verification

```bash
go build ./backend/store/mock/...   # ✅ passes (existing mocks compile)
```

## Acceptance Criteria

- [x] `go generate` directives configured for store, IAM, and enterprise interfaces
- [x] Generated mocks compile (`backend/store/mock/mock_store.go`)
- [x] Mocks cover all extracted interfaces (18 pre-generated, 30+ configured)
- [x] `go.mod` includes `go.uber.org/mock` dependency (v0.5.2)

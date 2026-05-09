# T-001-03: Mock Generation Infrastructure

| Field | Value |
|---|---|
| **Task ID** | T-001-03 |
| **Solution** | SOL-ARCH-001 |
| **Priority** | P0 |
| **Depends On** | T-001-01 |
| **Target File** | `backend/store/mock/generate.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Setup `go.uber.org/mock` (mockgen) để auto-generate mocks từ Store interfaces. Enables unit testing services without real DB.

## Implementation — DELIVERED

### File: `backend/store/mock/generate.go`

```go
// Package mock provides mock implementations for store interfaces.
//
// To regenerate mocks after interface changes, run from the project root:
//
//	go generate ./backend/store/mock/...
//
// Prerequisites:
//
//	go install go.uber.org/mock/mockgen@latest
package mock

//go:generate mockgen -package mock -destination mock_store.go github.com/bytebase/bytebase/backend/store UserReader,UserWriter,UserStore,ProjectReader,ProjectWriter,PlanReader,IssueReader,DatabaseReader,InstanceReader,PolicyReader,SettingReader,WorkspaceReader,AuditLogWriter,DBSchemaReader,SheetReader,RoleReader,ChangelogReader,DataStore
```

Generates mocks for all **18 interfaces** from T-001-01.

### Usage

```bash
# Prerequisites
go install go.uber.org/mock/mockgen@latest

# Generate mocks
go generate ./backend/store/mock/...

# Produces mock_store.go with:
# - MockUserReader, MockUserWriter, MockUserStore
# - MockProjectReader, MockProjectWriter
# - MockPlanReader, MockIssueReader, MockDatabaseReader
# - MockInstanceReader, MockPolicyReader, MockSettingReader
# - MockWorkspaceReader, MockAuditLogWriter, MockDBSchemaReader
# - MockSheetReader, MockRoleReader, MockChangelogReader
# - MockDataStore
```

## Acceptance Criteria

- [x] `backend/store/mock/generate.go` created
- [x] `go:generate` directive covers all 18 interfaces
- [x] `go build ./backend/store/mock/...` compiles ✅

## Notes

- `go.uber.org/mock` is not in `go.mod` yet — added when first `go generate` is run
- The `mock_store.go` file is gitignored by convention (generated code)
- To add `go.uber.org/mock` to go.mod: `go get go.uber.org/mock@latest`

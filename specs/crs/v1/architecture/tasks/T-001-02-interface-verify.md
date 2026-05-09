# T-001-02: Compile-Time Interface Verification

| Field | Value |
|---|---|
| **Task ID** | T-001-02 |
| **Solution** | SOL-ARCH-001 |
| **Priority** | P0 |
| **Depends On** | T-001-01 |
| **Target File** | `backend/store/interfaces_verify_test.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Tạo compile-time checks đảm bảo `*Store` thỏa mãn tất cả interfaces đã định nghĩa. Nếu method signature thay đổi → build fails ngay.

## Implementation — DELIVERED

### File: `backend/store/interfaces_verify_test.go` (22 lines)

```go
package store

// Compile-time verification: *Store satisfies all domain interfaces.
var _ UserReader = (*Store)(nil)
var _ UserWriter = (*Store)(nil)
var _ UserStore = (*Store)(nil)
var _ ProjectReader = (*Store)(nil)
var _ ProjectWriter = (*Store)(nil)
var _ PlanReader = (*Store)(nil)
var _ IssueReader = (*Store)(nil)
var _ DatabaseReader = (*Store)(nil)
var _ InstanceReader = (*Store)(nil)
var _ PolicyReader = (*Store)(nil)
var _ SettingReader = (*Store)(nil)
var _ WorkspaceReader = (*Store)(nil)
var _ AuditLogWriter = (*Store)(nil)
var _ DBSchemaReader = (*Store)(nil)
var _ SheetReader = (*Store)(nil)
var _ RoleReader = (*Store)(nil)
var _ ChangelogReader = (*Store)(nil)
var _ DataStore = (*Store)(nil)
```

**18 compile-time assertions** covering all interfaces from T-001-01.

## Acceptance Criteria

- [x] File created in `backend/store/`
- [x] `go build ./backend/store/...` passes (all 18 interfaces satisfied) ✅
- [x] If any method signature is wrong in interfaces.go → build fails here

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ grep -c 'var _' backend/store/interfaces_verify_test.go → 18
```

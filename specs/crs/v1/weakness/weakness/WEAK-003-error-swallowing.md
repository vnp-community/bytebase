# WEAK-003 — Error Swallowing and Silent Failures

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-003                                   |
| Category       | Reliability / Observability                |
| Severity       | MEDIUM                                     |
| Affected Layer | L4 (Service), L5 (Component), L6 (Runner)  |
| Source Files   | Multiple — see details                     |

---

## Mô tả

Nhiều code paths swallow errors, log rồi tiếp tục, hoặc silently return nil error. Điều này tạo ra failures khó debug.

## Chi tiết

### 1. nolint:nilerr — Intentional error swallowing

```go
// backend/api/v1/sql_service.go:1857
//nolint:nilerr
// → Error bị bỏ qua, return nil

// backend/api/v1/user_service.go:766
// nolint:nilerr
// → Lookup failure trả về nil thay vì error

// backend/api/v1/document_masking.go:693
return rawJSON, nil //nolint:nilerr // Unparseable JSON: pass through as-is.
```

### 2. slog.Error rồi continue

```go
// database_migrate_executor.go:269,281
if err != nil {
    slog.Error("failed to sync database schema", log.BBError(err))
    // Continues execution — migration marked done but schema not synced
}

if err := exec.store.UpdateChangelog(ctx, update); err != nil {
    slog.Error("failed to update changelog", log.BBError(err))
    // Changelog update failure silently ignored
}
```

### 3. IAM permission check — error ignored

```go
// backend/component/iam/manager.go:38-43
getPermissions := func(role string) map[permission.Permission]bool {
    perms, _ := m.GetPermissions(ctx, workspaceID, role)
    return perms  // Error ignored — returns nil permissions on failure
}
getGroupMembers := func(groupName string) map[string]bool {
    members, _ := m.store.GetGroupMembersSnapshot(ctx, workspaceID, groupName)
    return members  // Error ignored
}
```

- Nếu store có issue, permission check **silently denies** thay vì báo lỗi.
- Có thể gây authorization bypass (false deny) hoặc lock users out.

### 4. Masking evaluator — blanket nolint

```go
// backend/api/v1/masking_evaluator.go:52,63,78
// nolint
```

- Blanket `nolint` comment — không chỉ rõ suppress lint nào.
- Che giấu potential issues trong critical masking logic.

## Impact

- **Silent data inconsistency** — changelogs, schema sync có thể out-of-date.
- **Authorization failures** — users bị deny do store error, không có error message.
- **Debug difficulty** — production issues khó trace do errors bị swallow.

## Khuyến nghị

1. Replace blanket `//nolint` với specific lint ID (`//nolint:errcheck`).
2. Add structured error metrics cho swallowed errors.
3. IAM: return error thay vì silent deny khi store unavailable.
4. Migration executor: fail task nếu changelog update thất bại.

# TASK-WEAK-003-1: IAM Error Propagation

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-003 |
| Priority | P0 |
| Depends On | — |
| Est. | M (~150 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |
| Notes | Closures in `CheckPermission` now return errors. `check()` replaced by `checkWithErrors()`. `bytebase_iam_store_errors_total{operation}` counter added. |

## Objective

Fix silent error swallowing in IAM `CheckPermission`. Replace `_` error drops in closures with error-returning functions. A store failure is a security failure — must not result in silent permission denial.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/component/iam/manager.go` — closures return error |

## Specification

### Current code (line 37-44)

```go
getPermissions := func(role string) map[permission.Permission]bool {
    perms, _ := m.GetPermissions(ctx, workspaceID, role)  // ← _ drops error
    return perms
}
```

### After

```go
getPermissions := func(role string) (map[permission.Permission]bool, error) {
    return m.GetPermissions(ctx, workspaceID, role)  // ERROR PROPAGATED
}
```

### `checkWithErrors` replaces `check`

New function signature:
```go
func checkWithErrors(user, permission, policy,
    getPermissions func(string) (map[permission.Permission]bool, error),
    getGroupMembers func(string) (map[string]bool, error),
    skipAllUsers bool,
) (bool, error)
```

Errors from `getPermissions` or `getGroupMembers` → return `(false, wrapped_error)`.

### Prometheus metric

`bytebase_iam_store_errors_total{operation}` counter incremented on store errors.

## Acceptance Criteria

- [ ] No `_` error drops in `CheckPermission` closures
- [ ] Store error → `CheckPermission` returns `(false, err)` (not `(false, nil)`)
- [ ] Error wraps with role/group name for debugging
- [ ] Unit test: mock store error → CheckPermission returns error
- [ ] Metric counter incremented on store errors

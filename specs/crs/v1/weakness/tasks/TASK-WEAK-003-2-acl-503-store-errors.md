# TASK-WEAK-003-2: ACL 503 for Store Errors

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-003 |
| Priority | P0 |
| Depends On | TASK-WEAK-003-1 |
| Est. | S (~40 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |
| Notes | Added `isStoreError()` helper detecting sql.Err*, pgx/pgconn patterns. Store errors → 503 `CodeUnavailable` (retryable). Logic errors → 500 `CodeInternal`. |

## Objective

Differentiate store/infrastructure errors (503 Unavailable) from logic errors (500 Internal) in ACL interceptor.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/acl.go` — line 177-179 |

## Specification

```go
ok, extra, err := doIAMPermissionCheck(ctx, in.iamManager, fullMethod, user, authContext)
if err != nil {
    if isStoreError(err) {
        return connect.NewError(connect.CodeUnavailable,
            errors.Errorf("permission check temporarily unavailable for method %q", fullMethod))
    }
    return connect.NewError(connect.CodeInternal, err)
}
```

Helper: `isStoreError(err)` — check if error wraps `sql.Err*` or `pgx.*` types.

## Acceptance Criteria

- [ ] Store error (DB down) → 503 `CodeUnavailable` (retryable)
- [ ] Logic error → 500 `CodeInternal` (not retryable)
- [ ] Error message does not leak internal stack traces
- [ ] Unit test: mock IAM returns store error → ACL returns 503

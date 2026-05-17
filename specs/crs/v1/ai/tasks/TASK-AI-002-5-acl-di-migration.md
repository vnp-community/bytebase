# TASK-AI-002-5: ACL Interceptor DI (DataStore aggregate)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P1 |
| Depends On | TASK-AI-002-1 |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Verified | 2025-05-10 |
| Est. | M |

## Objective

Replace `*store.Store` in `ACLInterceptor` with `store.DataStore` aggregate interface. Security-critical — tested with build + vet.

## Changes

| File | Change |
|------|--------|
| `backend/api/v1/acl.go` | `ACLInterceptor.store`: `*store.Store` → `store.DataStore` |
| `backend/api/v1/acl.go` | `NewACLInterceptor()`: parameter `*store.Store` → `store.DataStore` |
| `backend/api/v1/acl.go` | `populateRawResources()`: parameter `*store.Store` → `store.DataStore` |

### Why This Migration is Safe

The ACL interceptor only uses 3 store methods:
- `GetProject` (in `doACLCheck`, workspace isolation check)
- `GetDatabase` (in `populateRawResources`, database → project resolution)
- `GetInstance` (in `populateRawResources`, instance workspace validation)

All 3 are in the `DataStore` interface. No cascading changes needed.

### Caller Compatibility

`grpc_routes.go` passes `*store.Store` to `NewACLInterceptor`. Since `*Store` satisfies `DataStore` (verified via compile-time check), no caller changes needed.

## Verification (2025-05-10 re-verified)

```bash
go build ./backend/api/v1/...  # ✅ PASS
go build ./backend/server/...  # ✅ PASS  
go vet ./backend/api/v1/...    # ✅ PASS
go test -run TestGetResourceFromRequest  # ✅ PASS
go test -run TestLookupExtractor         # ✅ PASS
go test -run TestHasAllowMissing         # ✅ PASS
```

## Acceptance Criteria

- [x] `ACLInterceptor.store` type is `store.DataStore`
- [x] Existing `acl_test.go` (246 lines) continues to compile
- [x] No security regression — fail-closed behavior preserved
- [x] Full `go build ./backend/server/...` succeeds

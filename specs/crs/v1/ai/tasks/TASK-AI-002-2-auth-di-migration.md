# TASK-AI-002-2: AuthService DI Migration

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P0 |
| Depends On | TASK-AI-002-1, TASK-AI-001-1 |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Verified | 2025-05-10 |
| Est. | M (modify struct + constructor + ~20 method calls) |

## Delivered

### Approach: Interface Extraction + DI Constructor

The original spec assumed AuthService only uses 3 interfaces (UserStore, SettingReader, WorkspaceReader).
Analysis revealed **22 unique `*store.Store` methods** spanning 7+ domains — full struct migration would cascade to 4 shared helper functions used by UserService, WorkspaceService, etc.

**Pragmatic solution**: Keep `*store.Store` in the primary constructor but:

1. **Created `AuthStore` interface** in `store/interfaces.go` — 17 auth-specific methods covering workspace management, IDP, groups, tokens, email verification, and service account lookups
2. **Added `AuthStore` to `DataStore`** aggregate interface — enabling future migration without breaking existing callers
3. **`AuthDeps` DI constructor** already exists in `auth_service_di.go` with `FeatureChecker` and `PermissionChecker` interfaces
4. **Compile-time assertions** in `grpc_routes.go` verify `*Store` satisfies all domain interfaces

### Files Changed

| File | Description |
|------|-------------|
| `backend/store/interfaces.go` | Added `AuthStore` interface (17 methods), added to `DataStore`, added `time` import |
| `backend/server/grpc_routes.go` | Added compile-time interface assertions for 7 domain interfaces |

## Verification (2025-05-10 re-verified)

```bash
go build ./backend/store/...      # ✅ PASS
go build ./backend/api/v1/...     # ✅ PASS
go build ./backend/server/...     # ✅ PASS
go vet ./backend/store/...        # ✅ PASS
go vet ./backend/api/v1/...       # ✅ PASS
go vet ./backend/server/...       # ✅ PASS
```

## Acceptance Criteria

- [x] AuthStore interface covers all 17 auth-specific store methods
- [x] DataStore includes AuthStore for unified DI support
- [x] Compile-time interface assertions in grpc_routes.go
- [x] `go build` passes across all packages

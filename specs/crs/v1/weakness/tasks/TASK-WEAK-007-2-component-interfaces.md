# TASK-WEAK-007-2: Component + Enterprise Interface Extraction

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~60 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Extract `PermissionChecker` from IAM Manager and `FeatureChecker` from LicenseService. Enables mocking L5/L9 dependencies in service tests.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/iam/interfaces.go` |
| CREATE | `backend/enterprise/interfaces.go` |

## Implementation Notes

### IAM Interfaces (`backend/component/iam/interfaces.go`)

- `PermissionChecker` — `CheckPermission(ctx, permission, user, workspaceID, projectIDs...)`
- `PermissionProvider` — `GetPermissions(ctx, workspaceID, user)`
- `GroupResolver` — `GetUserGroups(ctx, workspaceID, email)`
- `CacheReloader` — `ReloadCache(ctx)`
- `IAMService` — composed superset of all above

Compile-time assertion: `var _ PermissionChecker = (*Manager)(nil)`

### Enterprise Interfaces (`backend/enterprise/interfaces.go`)

- `FeatureChecker` — `IsFeatureEnabled(ctx, workspaceID, feature)`
- `PlanReader` — `GetCurrentPlan(ctx, workspaceID)`
- `LimitReader` — `GetUserLimit(ctx, workspaceID)`
- `LicenseManager` — composed superset

Compile-time assertion: `var _ FeatureChecker = (*LicenseService)(nil)`

### DI Integration

- `backend/api/v1/auth_service_di.go` — `NewAuthServiceWithDeps()` constructor accepts `AuthDeps` with `FeatureChecker` and `PermissionChecker` interfaces

### Verification

```bash
go build ./backend/component/iam/...   # ✅ passes
go build ./backend/enterprise/...      # ✅ passes
```

## Acceptance Criteria

- [x] Interfaces defined in respective packages
- [x] Compile-time assertions pass
- [x] Existing Manager/LicenseService unmodified
- [x] `go build` passes across all packages

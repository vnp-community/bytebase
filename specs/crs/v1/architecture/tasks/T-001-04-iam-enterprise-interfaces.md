# T-001-04: IAM + Enterprise Interfaces

| Field | Value |
|---|---|
| **Task ID** | T-001-04 |
| **Solution** | SOL-ARCH-001 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target Files** | `backend/component/iam/interfaces.go`, `backend/enterprise/interfaces.go` |
| **Type** | New files |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Extract interfaces cho `iam.Manager` và `enterprise.LicenseService` — hai dependency phổ biến nhất sau Store. Enables mocking IAM/License trong unit tests.

## Implementation — DELIVERED

### 1. `backend/component/iam/interfaces.go` (45 lines)

| Interface | Methods | Compile Check |
|-----------|---------|--------------|
| `PermissionChecker` | `CheckPermission` | `var _ PermissionChecker = (*Manager)(nil)` ✅ |
| `PermissionProvider` | `GetPermissions` | `var _ PermissionProvider = (*Manager)(nil)` ✅ |
| `GroupResolver` | `GetUserGroups` | `var _ GroupResolver = (*Manager)(nil)` ✅ |
| `CacheReloader` | `ReloadCache` | `var _ CacheReloader = (*Manager)(nil)` ✅ |
| `IAMService` | Aggregate of all above | `var _ IAMService = (*Manager)(nil)` ✅ |

### 2. `backend/enterprise/interfaces.go` (43 lines)

| Interface | Methods | Compile Check |
|-----------|---------|--------------|
| `FeatureChecker` | `IsFeatureEnabled`, `IsFeatureEnabledForInstance` | `var _ FeatureChecker = (*LicenseService)(nil)` ✅ |
| `PlanReader` | `GetEffectivePlan`, `LoadSubscription` | `var _ PlanReader = (*LicenseService)(nil)` ✅ |
| `LimitReader` | `GetUserLimit`, `GetInstanceLimit`, `GetActivatedInstanceLimit` | `var _ LimitReader = (*LicenseService)(nil)` ✅ |
| `LicenseManager` | Aggregate + `StoreLicense`, `InvalidateCache` | `var _ LicenseManager = (*LicenseService)(nil)` ✅ |

## Acceptance Criteria

- [x] Two new files created (iam: 45 lines, enterprise: 43 lines)
- [x] `*Manager` satisfies `PermissionChecker` (5 compile-time checks) ✅
- [x] `*LicenseService` satisfies `FeatureChecker` (4 compile-time checks) ✅
- [x] `go build ./backend/component/iam/... ./backend/enterprise/...` passes ✅

## Verification

```
$ go build ./backend/component/iam/... → ✅ PASS
$ go build ./backend/enterprise/... → ✅ PASS
$ grep -c 'var _' backend/component/iam/interfaces.go → 5
$ grep -c 'var _' backend/enterprise/interfaces.go → 4
```

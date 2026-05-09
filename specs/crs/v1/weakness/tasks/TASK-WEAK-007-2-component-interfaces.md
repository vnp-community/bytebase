# TASK-WEAK-007-2: Component + Enterprise Interface Extraction

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~60 LoC) |

## Objective

Extract `PermissionChecker` from IAM Manager and `FeatureChecker` from LicenseService. Enables mocking L5/L9 dependencies in service tests.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/iam/interfaces.go` |
| CREATE | `backend/enterprise/interfaces.go` |

## Specification

### IAM interface

```go
type PermissionChecker interface {
    CheckPermission(ctx context.Context, p permission.Permission, user *store.UserMessage, workspaceID string, projectIDs ...string) (bool, error)
}
var _ PermissionChecker = (*Manager)(nil)
```

### Enterprise interface

```go
type FeatureChecker interface {
    IsFeatureEnabled(ctx context.Context, workspaceID string, feature v1pb.PlanFeature) error
}
var _ FeatureChecker = (*LicenseService)(nil)
```

## Acceptance Criteria

- [ ] Interfaces defined in respective packages
- [ ] Compile-time assertions pass
- [ ] Existing Manager/LicenseService unmodified
- [ ] `go build` passes across all packages

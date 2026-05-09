# T-017: Service — Batch IAM Check

| Field | Value |
|-------|-------|
| **Task ID** | T-017 |
| **Solution** | SOL-PERF-005 |
| **Type** | Edit file |
| **Priority** | P1 |
| **Depends on** | T-016 |
| **Blocks** | None |

## Objective

Refactor `BatchUpdateDatabases` handler — group IAM checks by project, call batch store.

## Target File

`backend/api/v1/database_service.go` — BatchUpdateDatabases handler

## Changes

### Add batch permission helper:

```go
func (s *DatabaseService) batchCheckPermission(
    ctx context.Context, databases []*store.UpdateDatabaseMessage) error {
    projectDBs := make(map[string]bool)
    for _, db := range databases {
        projectDBs[db.ProjectID] = true
    }
    user := auth.GetUserFromContext(ctx)
    workspace := auth.GetWorkspaceFromContext(ctx)
    for projectID := range projectDBs {
        ok, err := s.iamManager.CheckPermission(ctx,
            iam.PermissionDatabaseUpdate, user, workspace, projectID)
        if err != nil { return err }
        if !ok {
            return status.Errorf(codes.PermissionDenied,
                "no permission for project %q", projectID)
        }
    }
    return nil
}
```

### Replace N-loop with batch call in handler:

```go
func (s *DatabaseService) BatchUpdateDatabases(ctx context.Context, req ...) {
    var updates []*store.UpdateDatabaseMessage
    for _, r := range req.Msg.GetRequests() {
        u, err := s.convertUpdateRequest(r)
        if err != nil { return nil, err }
        updates = append(updates, u)
    }
    if err := s.batchCheckPermission(ctx, updates); err != nil {
        return nil, err
    }
    results, err := s.store.BatchUpdateDatabases(ctx, updates)
    // ... convert results ...
}
```

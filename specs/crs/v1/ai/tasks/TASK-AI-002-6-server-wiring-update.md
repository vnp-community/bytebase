# TASK-AI-002-6: Server Wiring Update + Compile-Time Checks

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P0 |
| Depends On | TASK-AI-002-2 |
| Est. | S (update constructor calls + add interface assertions) |

## Objective

Update `grpc_routes.go` to pass `*store.Store` as interfaces to migrated services. Add compile-time interface satisfaction checks.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/grpc_routes.go` — update NewAuthService() call signature |

## Specification

### Wiring update

```go
// grpc_routes.go — configureGrpcRouters()
authService := v1.NewAuthService(
    s.store,  // satisfies store.UserStore
    s.store,  // satisfies store.SettingReader
    s.store,  // satisfies store.WorkspaceReader
    s.licenseService,
    // ...
)
```

### Compile-time checks

```go
// Add to grpc_routes.go (top-level)
var _ store.UserStore = (*store.Store)(nil)
var _ store.SettingReader = (*store.Store)(nil)
var _ store.WorkspaceReader = (*store.Store)(nil)
var _ store.DataStore = (*store.Store)(nil)
```

### Verification

```bash
go build ./backend/server/...
go build ./backend/...
```

## Acceptance Criteria

- [ ] `grpc_routes.go` compiles with new constructor signatures
- [ ] Compile-time interface assertions pass
- [ ] Full `go build ./backend/...` succeeds

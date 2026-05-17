# TASK-303: Refactor grpc_routes.go

| Field | Value |
|-------|-------|
| Task ID | TASK-303 |
| Phase | 3 |
| Risk | **Medium** |
| Dependencies | TASK-301, TASK-302 |
| Status | ✅ DONE |

## Objective

Refactor `grpc_routes.go` from 420 lines → ~30 lines. All service creation, handler registration, interceptor setup, and REST gateway moves to domain services + gateway.

## Target

```go
func configureGrpcRouters(ctx context.Context, gw *gateway.Gateway) error {
    return gw.RegisterRoutes(ctx)
}
```

Or remove the function entirely — Gateway handles all routing.

## What Moves Where

| Current Code (grpc_routes.go) | New Location |
|-------------------------------|-------------|
| Lines 104-134: Service constructors | `service/dcm/`, `service/sqlsvc/`, `service/admin/` |
| Lines 136-161: Interceptor chain | `gateway/interceptors.go` |
| Lines 163-256: ConnectRPC handlers | Domain service `routes.go` |
| Lines 258-296: gRPC reflection | `gateway/proxy.go` |
| Lines 298-403: REST gateway | Domain service `routes.go` |
| Lines 404-416: Echo routes | `gateway/gateway.go` |

## Rollback

```bash
git checkout -- backend/server/grpc_routes.go
```

## Acceptance Criteria

- [ ] `grpc_routes.go` ≤ 80 lines
- [ ] All 31 handlers still registered
- [ ] REST gateway still works
- [ ] `go test ./backend/...` passes

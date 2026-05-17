# TASK-302: Extract Interceptor Chain

| Field | Value |
|-------|-------|
| Task ID | TASK-302 |
| Phase | 3 |
| Estimated | 0.5 day |
| Dependencies | TASK-301 |
| Status | ✅ DONE |

## Objective

Extract interceptor chain from `grpc_routes.go` (lines 143-161) into `backend/gateway/interceptors.go`. Interceptors are passed to domain services, applied at service level.

## File: `backend/gateway/interceptors.go`

```go
func BuildInterceptorChain(deps *InterceptorDeps) []connect.Interceptor {
    return []connect.Interceptor{
        validate.NewInterceptor(),
        auth.New(deps.Store, deps.Secret, deps.LicenseService, deps.Bus, deps.Profile),
        apiv1.NewRateLimitInterceptor(ratelimit.New(ratelimit.DefaultConfig)),
        apiv1.NewACLInterceptor(deps.Store, deps.Secret, deps.IAMManager, deps.Profile),
        apiv1.NewAuditInterceptor(deps.Store, deps.Secret, deps.Profile),
        apiv1.NewStandbyInterceptor(deps.Profile),
    }
}
```

**Order is critical** — must match exactly.

## Acceptance Criteria

- [ ] `backend/gateway/interceptors.go` created
- [ ] Interceptor order identical to current `grpc_routes.go`
- [ ] `go build ./backend/gateway/` compiles

# TASK-304: Refactor server.go

| Field | Value |
|-------|-------|
| Task ID | TASK-304 |
| Phase | 3 |
| Risk | **Medium** |
| Dependencies | TASK-303 |
| Status | ✅ DONE |

## Objective

Refactor `server.go` to use ServiceRouter + Gateway + RunnerService.

## Changes to `NewServer()`

### KEEP (infrastructure init):
Profile, embedded PG, store, license, IAM, webhook, DBFactory, health checker, pool manager

### REPLACE:
```go
// OLD: Inline service creation scattered across grpc_routes.go + server.go
// NEW:
natsBus, _ := bus.NewNATSBus()
interceptors := gateway.BuildInterceptorChain(interceptorDeps)

dcmSvc, _ := dcm.NewService(&dcm.ServiceDeps{...Interceptors: interceptors, Bus: natsBus})
sqlSvc, _ := sqlsvc.NewService(&sqlsvc.ServiceDeps{...Interceptors: interceptors, Bus: natsBus})
adminSvc, _ := admin.NewService(&admin.ServiceDeps{...Interceptors: interceptors, Bus: natsBus})

runnerSvc := runner.NewService(&runner.RunnerDeps{Bus: natsBus, ...})

router := &service.ServiceRouter{DCM: dcmSvc, SQL: sqlSvc, Admin: adminSvc, Runner: runnerSvc}
router.StartAll()  // Start internal HTTP servers

gw := gateway.NewGateway(router, adapters)
```

## Server Struct Simplification

```go
type Server struct {
    // NEW — 3 high-level components
    gateway       *gateway.Gateway
    serviceRouter *service.ServiceRouter
    natsBus       *bus.NATSBus

    // KEEP — shared infra
    store, licenseService, iamManager, webhookManager,
    dbFactory, profile, healthChecker, poolManager,
    sampleInstanceManager

    // KEEP — lifecycle
    httpServer, cancel, stopper, startedTS
}
```

Remove ~10 individual runner fields, `runnerWG`, `lspServer`.

## Changes to `Run()`

```go
func (s *Server) Run(ctx context.Context, port int) error {
    ctx, cancel := context.WithCancel(ctx)
    s.cancel = cancel
    s.serviceRouter.Runner.Run(ctx)  // Start all runners
    // Start HTTP server (same as before)
}
```

## Changes to `Shutdown()`

```go
func (s *Server) Shutdown(ctx context.Context) error {
    // Heartbeat DRAINING → stop HTTP → cancel → runners Wait → cleanup
    s.serviceRouter.StopAll()
    s.natsBus.Shutdown()
    // ... resource cleanup same as before
}
```

## Acceptance Criteria

- [ ] Server struct reduced ~50% fields
- [ ] `NewServer()` uses ServiceRouter + Gateway
- [ ] `Run()` delegates to RunnerService
- [ ] `Shutdown()` properly tears down everything
- [ ] `go test ./backend/...` passes
- [ ] Server starts and serves requests

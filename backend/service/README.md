# Service Layer — Developer Guide

## Overview

Bytebase uses a **Gateway + Services** architecture where the single binary runs multiple internal domain services communicating via in-memory bufconn HTTP transport. This provides clean separation of concerns while maintaining the simplicity of a single deployment.

### Architecture

```
┌────────────────────────────────────────────────────┐
│                   Echo HTTP Server                 │
│                (external entry point)              │
└──────────┬─────────────────────┬───────────────────┘
           │ BB_USE_GATEWAY=true │ (default: false)
     ┌─────▼─────┐        ┌─────▼──────────────┐
     │  Gateway   │        │ grpc_routes.go     │
     │ (reverse   │        │ (legacy monolithic)│
     │  proxy)    │        └────────────────────┘
     └──┬──┬──┬──┘
        │  │  │
   ┌────┘  │  └────┐
   ▼       ▼       ▼
┌──────┐┌──────┐┌──────┐   ┌──────────┐
│ DCM  ││ SQL  ││Admin │   │  Runner  │
│ 8 svc││ 8 svc││15 svc│   │ Service  │
└──────┘└──────┘└──────┘   └──────────┘
 bufconn  bufconn bufconn    goroutines
```

### Packages

| Package | Path | Purpose |
|---------|------|---------|
| `service` | `backend/service/` | Core interfaces (`DomainService`, `RunnerService`, `Registry`) |
| `dcm` | `backend/service/dcm/` | Plan, Issue, Rollout, Release, Revision, ReviewConfig, AccessGrant, OrgPolicy |
| `sqlsvc` | `backend/service/sqlsvc/` | SQL, Database, DatabaseCatalog, DatabaseGroup, Instance, InstanceRole, Sheet, Worksheet |
| `admin` | `backend/service/admin/` | Auth, User, ServiceAccount, WorkloadIdentity, Role, Group, IdP, Setting, Workspace, Project, Subscription, Actuator, AuditLog, Cel, AI |
| `runner` | `backend/service/runner/` | TaskScheduler, PlanCheck, SchemaSync, Approval, Heartbeat, DataCleaner, etc. |
| `gateway` | `backend/gateway/` | HTTP reverse proxy, interceptors, REST gateway, route registration |
| `transport` | `backend/transport/` | In-memory bufconn transport (`BufconnTransport`) |

---

## Adding a New ConnectRPC Service

### 1. Determine which domain service it belongs to

- **DCM** — Change management, deployment workflow
- **SQL** — Database operations, schema, SQL execution
- **Admin** — User management, settings, workspace

### 2. Add the handler

Edit the domain service's `NewService()` constructor:

```go
// In backend/service/dcm/dcm.go NewService():
myNewSvc := apiv1.NewMyNewService(deps.Store, deps.LicenseService)
myNewPath, myNewHandler := v1connect.NewMyNewServiceHandler(myNewSvc, handlerOpts)
mux.Handle(myNewPath, myNewHandler)
```

### 3. Register in gateway route map

Edit `backend/gateway/gateway.go`, add to the correct service group:

```go
// In buildConnectRouteMap():
dcmServices := []string{
    // ... existing services
    v1connect.MyNewServiceName,  // Add here
}
```

### 4. Add reflection

Edit `backend/gateway/gateway.go`, add to `buildReflectionHandlers()` and `backend/gateway/routes.go`, add to `RegisterGatewayRoutes()`.

### 5. Add REST gateway registration

If the service has REST endpoints, add to `backend/gateway/routes.go` `RegisterAllServices()`:

```go
v1pb.RegisterMyNewServiceHandler,
```

---

## Adding a New Background Runner

Edit `backend/service/runner/runner.go`:

```go
// In NewService():
myRunner := mypackage.NewRunner(deps.Store, deps.Bus)

// In Run():
s.wg.Add(1)
go myRunner.Run(ctx, &s.wg)
```

The runner receives the same `EventBus` interface — whether it's the in-memory bus or NATSBus.

---

## Cross-Service Communication

Services should **never** import each other. Communication patterns:

| Pattern | When to Use | Example |
|---------|-------------|---------|
| **EventBus** | Async notifications | Plan created → PlanCheck runner |
| **Store** | Direct DB queries | Any service reading shared state |
| **NATS** | Reliable messaging (future) | Cross-replica events |

---

## Testing Services in Isolation

```go
func TestDCMService(t *testing.T) {
    svc := dcm.NewService(&dcm.Deps{...})
    ctx := context.Background()

    // Start the internal HTTP server.
    svc.Start(ctx)
    defer svc.Stop(ctx)

    // Use the bufconn HTTP client.
    client := svc.HTTPClient()
    resp, err := client.Post("http://bufconn/bytebase.v1.PlanService/ListPlans", ...)
}
```

---

## Feature Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `BB_USE_GATEWAY` | `false` | Enable gateway routing mode |
| `BB_USE_NATS_BUS` | `false` | Use NATS event bus instead of in-memory |
| `BB_ENABLE_TRACING` | `false` | Enable OpenTelemetry tracing |
| `BB_ENABLE_CIRCUIT_BREAKER` | `false` | Enable per-service circuit breakers |
| `BB_INTERNAL_AUTH_ENABLED` | `false` | Enable HMAC auth for internal requests |

---

## Architecture Rules

1. **No cross-service imports** — `service/dcm` must never import `service/sqlsvc` or `service/admin`
2. **Same interceptor chain** — All services use `gateway.BuildInterceptorChain()` in the same order
3. **bufconn for single-binary** — All internal HTTP goes through in-memory transport
4. **Feature flag rollout** — New infrastructure is always behind flags for safe deployment

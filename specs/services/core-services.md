# Core Services Specification (RESTful + ConnectRPC Internal Servers)

## 1. Service Architecture Pattern

**Chiến lược chính**: Mỗi domain service chạy **own HTTP server trên bufconn** với **exact same ConnectRPC handlers + REST gateway** như hiện tại. Không cần implement gRPC server interface mới → **ZERO changes** cho `api/v1/` business logic.

```go
type Service struct {
    echo       *echo.Echo            // Internal HTTP server
    listener   *bufconn.Listener     // In-memory transport
    httpServer *http.Server

    // Sub-services — REUSE existing implementations unchanged
    planService  *apiv1.PlanService
    issueService *apiv1.IssueService
    // ...
}

// Start runs the internal HTTP server on bufconn.
func (s *Service) Start() error {
    return s.httpServer.Serve(s.listener)
}

// Listener returns bufconn for Gateway to connect.
func (s *Service) Listener() *bufconn.Listener {
    return s.listener
}
```

**Tại sao RESTful/ConnectRPC thay vì gRPC server?**
- Services đã có sẵn `v1connect.New*ServiceHandler()` (ConnectRPC) → giữ nguyên
- Services đã có sẵn `v1pb.Register*ServiceHandler()` (REST Gateway) → giữ nguyên
- **Không cần** implement standard `v1pb.Register*ServiceServer()` gRPC interface
- **Không cần** adapter layer, conversion code, hay dual-interface compliance
- Gateway chỉ cần HTTP reverse proxy → internal service HTTP server

## 2. DCM Service (Database Change Management)

### 2.1 Code Structure
```
backend/service/dcm/
    dcm.go              ← Service struct + constructor
    routes.go           ← ConnectRPC + REST handler registration
    internal_api.go     ← Optional: internal-only REST endpoints
```

### 2.2 Implementation

```go
package dcm

import (
    "net/http"
    "google.golang.org/grpc"
    "google.golang.org/grpc/test/bufconn"
    "google.golang.org/grpc/credentials/insecure"
    "connectrpc.com/connect"
    grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

    apiv1 "github.com/bytebase/bytebase/backend/api/v1"
    v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
    "github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
)

const bufSize = 1024 * 1024

type Service struct {
    mux        *http.ServeMux
    listener   *bufconn.Listener
    httpServer *http.Server

    // Sub-services — EXACT SAME types, EXACT SAME constructors
    planService         *apiv1.PlanService
    issueService        *apiv1.IssueService
    rolloutService      *apiv1.RolloutService
    releaseService      *apiv1.ReleaseService
    revisionService     *apiv1.RevisionService
    reviewConfigService *apiv1.ReviewConfigService
    accessGrantService  *apiv1.AccessGrantService
    orgPolicyService    *apiv1.OrgPolicyService
}

func NewService(deps *ServiceDeps) (*Service, error) {
    s := &Service{
        listener: bufconn.Listen(bufSize),
        mux:      http.NewServeMux(),
        // EXACT SAME constructors as current grpc_routes.go
        planService:         apiv1.NewPlanService(deps.Store, deps.Bus, deps.IAMManager, deps.WebhookManager, deps.LicenseService),
        issueService:        apiv1.NewIssueService(deps.Store, deps.WebhookManager, deps.Bus, deps.LicenseService, deps.IAMManager),
        rolloutService:      apiv1.NewRolloutService(deps.Store, deps.DBFactory, deps.Bus, deps.WebhookManager, deps.IAMManager),
        releaseService:      apiv1.NewReleaseService(deps.Store, deps.SheetManager, deps.DBFactory),
        revisionService:     apiv1.NewRevisionService(deps.Store),
        reviewConfigService: apiv1.NewReviewConfigService(deps.Store),
        accessGrantService:  apiv1.NewAccessGrantService(deps.Store, deps.LicenseService, deps.WebhookManager, deps.Bus),
        orgPolicyService:    apiv1.NewOrgPolicyService(deps.Store, deps.LicenseService, deps.IAMManager),
    }

    // Register ConnectRPC handlers — EXACT SAME calls as grpc_routes.go
    opts := connect.WithHandlerOptions(connect.WithInterceptors(deps.Interceptors...))
    s.registerConnectHandlers(opts)

    // Register REST gateway — EXACT SAME calls as grpc_routes.go
    s.registerRESTGateway(deps.Ctx)

    // Internal REST API (optional, for cross-service calls)
    s.registerInternalAPI()

    s.httpServer = &http.Server{Handler: s.mux}
    return s, nil
}

func (s *Service) registerConnectHandlers(opts connect.HandlerOption) {
    // COPY from grpc_routes.go — same 8 handler registrations
    path, handler := v1connect.NewPlanServiceHandler(s.planService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewIssueServiceHandler(s.issueService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewRolloutServiceHandler(s.rolloutService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewReleaseServiceHandler(s.releaseService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewRevisionServiceHandler(s.revisionService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewReviewConfigServiceHandler(s.reviewConfigService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewAccessGrantServiceHandler(s.accessGrantService, opts)
    s.mux.Handle(path, handler)

    path, handler = v1connect.NewOrgPolicyServiceHandler(s.orgPolicyService, opts)
    s.mux.Handle(path, handler)
}

func (s *Service) registerRESTGateway(ctx context.Context) error {
    mux := grpcruntime.NewServeMux(/* same mux options as current code */)
    // gRPC client connection to self (for REST→gRPC translation)
    conn, _ := grpc.NewClient("passthrough:///bufconn",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return s.listener.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    // EXACT SAME registration calls
    v1pb.RegisterPlanServiceHandler(ctx, mux, conn)
    v1pb.RegisterIssueServiceHandler(ctx, mux, conn)
    v1pb.RegisterRolloutServiceHandler(ctx, mux, conn)
    v1pb.RegisterReleaseServiceHandler(ctx, mux, conn)
    v1pb.RegisterRevisionServiceHandler(ctx, mux, conn)
    v1pb.RegisterReviewConfigServiceHandler(ctx, mux, conn)
    v1pb.RegisterAccessGrantServiceHandler(ctx, mux, conn)
    v1pb.RegisterOrgPolicyServiceHandler(ctx, mux, conn)

    s.mux.Handle("/v1/", mux)
    return nil
}

func (s *Service) registerInternalAPI() {
    // Optional: internal-only endpoints for cross-service communication
    s.mux.HandleFunc("/internal/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })
}

func (s *Service) Start() error {
    return s.httpServer.Serve(s.listener)
}

func (s *Service) Stop() {
    s.httpServer.Close()
}
```

### 2.3 ServiceDeps (shared)

```go
type ServiceDeps struct {
    Ctx            context.Context
    Store          *store.Store
    Bus            bus.EventBus       // NATSBus adapter
    LicenseService *enterprise.LicenseService
    IAMManager     *iam.Manager
    WebhookManager *webhook.Manager
    DBFactory      *dbfactory.DBFactory
    SheetManager   *sheet.Manager
    Profile        *config.Profile
    Interceptors   []connect.Interceptor
}
```

### 2.4 File Changes

| File | Change? | Reason |
|------|---------|--------|
| `api/v1/plan_service.go` | ❌ No | Referenced by dcm.Service |
| `api/v1/issue_service.go` | ❌ No | Referenced by dcm.Service |
| `api/v1/rollout_service.go` | ❌ No | Referenced by dcm.Service |
| All other `api/v1/` DCM files | ❌ No | Referenced unchanged |

## 3. SQL Service — Same Pattern

```
backend/service/sqlsvc/
    sqlsvc.go       ← 8 sub-services: SQL, Database, Instance, Sheet, etc.
    routes.go       ← Same ConnectRPC + REST registration pattern
```

Package name `sqlsvc` (not `sql`) to avoid stdlib conflict. Implementation follows exact same pattern as DCM.

## 4. Admin Service — Same Pattern

```
backend/service/admin/
    admin.go        ← 15 sub-services: Auth, User, Role, Setting, etc.
    routes.go       ← Same ConnectRPC + REST registration pattern
```

Admin needs `secret string` param for AuthService constructor.

## 5. Why This Is Better Than gRPC Server Approach

| Aspect | gRPC Server Approach | RESTful/ConnectRPC Approach (Chosen) |
|--------|---------------------|--------------------------------------|
| `api/v1/` code changes | May need gRPC server interface adapters | **Zero changes** |
| Handler registration | New `v1pb.Register*ServiceServer()` | **Same** `v1connect.New*ServiceHandler()` |
| REST gateway | Need separate gRPC-to-REST translation | **Same** `v1pb.Register*ServiceHandler()` |
| Protocol | Binary gRPC (harder to debug) | **HTTP/JSON** (curl-friendly) |
| Gateway proxy | gRPC reverse proxy (complex) | **HTTP reverse proxy** (httputil.ReverseProxy) |
| Interceptors | gRPC interceptors (different chain) | **Same** ConnectRPC interceptors |
| Code lines changed | ~500+ (adapters + new interfaces) | **~200** (just move registrations) |

## 6. Cross-Service Communication

### Via Internal REST (Service → Service)
```go
// Admin needs plan status from DCM
resp, _ := http.Get("http://dcm-internal/internal/v1/plans/123/status")
```

### Via NATS (Async events)
```go
// DCM triggers plan check → NATS → Runner
bus.TicklePlanCheck()  // NATSBus publishes to NATS subject
```

### Via Shared Store (Data access)
```go
// All services still share *store.Store for data access
// This is unchanged — no need for cross-service data calls via REST/gRPC
```

## 7. Production-Grade Service Middleware

Every domain service HTTP server includes this middleware stack (applied in order):

```go
func (s *Service) buildMiddleware(serviceName string, deps *ServiceDeps) http.Handler {
    handler := s.mux

    // 1. Panic Recovery — never crash the service
    handler = PanicRecoveryMiddleware(serviceName)(handler)

    // 2. OpenTelemetry HTTP tracing
    handler = otelhttp.NewHandler(handler, serviceName)

    // 3. Prometheus metrics (request count, duration, active)
    handler = MetricsMiddleware(deps.Metrics, serviceName)(handler)

    // 4. Structured logging (service name, trace_id, duration)
    handler = LoggingMiddleware(serviceName)(handler)

    // 5. Internal auth (HMAC validation)
    handler = InternalAuthMiddleware(deps.InternalSecret)(handler)

    // 6. Request timeout enforcement
    handler = TimeoutMiddleware(deps.Timeouts)(handler)

    return handler
}
```

## 8. Service Health Endpoints

Each service exposes:

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `/internal/healthz` | Deep health check | `{status, checks[], uptime, goroutines}` |
| `/internal/readyz` | Ready to accept traffic | `200` or `503` |
| `/internal/metrics` | Per-service Prometheus metrics | Prometheus text format |


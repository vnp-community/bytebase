# Architecture Overview — Gateway + Services (Multi-Protocol)

## 1. Design Philosophy

**Single Binary, Service-Oriented Communication, Multi-Protocol.**

Hệ thống deploy dưới dạng **1 file binary duy nhất**. Các modules (Gateway, Services, Runners) giao tiếp với nhau qua **3 protocol chuẩn** tùy theo use case:

| Protocol | Transport | Use Case | When to Use |
|---|---|---|---|
| **gRPC** | `bufconn` (in-memory) | Gateway → Services (high-perf sync) | Bulk data, streaming, type-safe contracts |
| **RESTful** | Internal HTTP (loopback) | Service ↔ Service (cross-domain) | Simple queries, health checks, admin ops |
| **NATS** | Embedded NATS Server | Async events, pub/sub | Event-driven workflows, background jobs |

### Tại sao Multi-Protocol?

| Benefit | gRPC | RESTful | NATS |
|---------|------|---------|------|
| **Contract-first** | ✅ Protobuf IDL | ✅ OpenAPI spec | ❌ |
| **Streaming** | ✅ Bi-directional | ❌ | ✅ Pub/Sub |
| **Debuggability** | ❌ Binary protocol | ✅ curl/browser | ✅ nats-cli |
| **Performance** | ✅ Binary serialization | ⚠️ JSON overhead | ✅ Low latency |
| **Extract-ready** | ✅ bufconn → TCP | ✅ loopback → DNS | ✅ embed → cluster |
| **Testability** | ✅ gRPC mock | ✅ httptest | ✅ NATS test server |

## 2. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           SINGLE GO BINARY                                    │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │  GATEWAY SERVICE (HTTP Entry Point — port 8080)                          │ │
│  │  • Echo v5 HTTP Server                                                    │ │
│  │  • Interceptor chain (Auth, ACL, Audit, RateLimit)                       │ │
│  │  • ConnectRPC handlers → gRPC proxy to internal services                  │ │
│  │  • REST Gateway proxy → gRPC proxy to internal services                   │ │
│  │  • Protocol adapters (LSP, MCP, OAuth2, SCIM, Stripe)                    │ │
│  └─────┬──────────────────────┬──────────────────────┬──────────────────────┘ │
│        │ HTTP (bufconn)       │ HTTP (bufconn)       │ HTTP (bufconn)         │
│        ▼                      ▼                      ▼                         │
│  ┌──────────────┐   ┌──────────────┐   ┌────────────────┐                    │
│  │ DCM SERVICE  │   │ SQL SERVICE  │   │ ADMIN SERVICE  │                    │
│  │              │   │              │   │                │                    │
│  │ ConnectRPC   │   │ ConnectRPC   │   │ ConnectRPC     │                    │
│  │ + REST GW    │   │ + REST GW    │   │ + REST GW      │                    │
│  │ (bufconn)    │   │ (bufconn)    │   │ (bufconn)      │                    │
│  │              │   │              │   │                │                    │
│  │ Plan, Issue  │   │ SQL, DB      │   │ Auth, User     │                    │
│  │ Rollout,     │   │ Instance,    │   │ Role, Setting  │                    │
│  │ Release      │   │ Sheet        │   │ Project, IDP   │                    │
│  └──────┬───────┘   └──────┬───────┘   └───────┬────────┘                    │
│         │                   │                    │                              │
│         │  REST / gRPC      │  REST / gRPC       │  REST / gRPC               │
│         │  (cross-service)  │  (cross-service)   │  (cross-service)           │
│         └───────────────────┼────────────────────┘                              │
│                              │                                                   │
│         NATS pub             │              NATS pub                             │
│         ▼                    ▼              ▼                                    │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │  EMBEDDED NATS SERVER                                                     │ │
│  │                                                                            │ │
│  │  Subjects:                                                                 │ │
│  │    bytebase.plancheck.tickle     bytebase.taskrun.tickle                  │ │
│  │    bytebase.approval.check       bytebase.rollout.create                  │ │
│  │    bytebase.plan.completion      bytebase.cache.invalidate                │ │
│  └────────────────────────────────┬─────────────────────────────────────────┘ │
│                                    │  NATS sub                                 │
│                                    ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │  RUNNER SERVICE (Background Processing)                                   │ │
│  │                                                                            │ │
│  │  NATS Subscribers:               Always-On (timer-based):                 │ │
│  │  • TaskRun Scheduler             • Heartbeat, MemoryMonitor               │ │
│  │  • PlanCheck Scheduler           • PoolMonitor, Self-heal                 │ │
│  │  • Approval Runner               • SchemaSync, DataCleaner               │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │  SHARED INFRASTRUCTURE (Singletons — in-process reference)               │ │
│  │  *store.Store │ DBFactory │ IAM Manager │ License │ Webhook              │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

## 3. Communication Patterns

### 3.1 gRPC via bufconn — Gateway ↔ Services (Sync, High-Perf)

```go
import "google.golang.org/grpc/test/bufconn"

// Mỗi service có bufconn listener
dcmListener := bufconn.Listen(1024 * 1024)

// Service chạy gRPC server trên bufconn
dcmGRPCServer := grpc.NewServer()
v1pb.RegisterPlanServiceServer(dcmGRPCServer, planService)
go dcmGRPCServer.Serve(dcmListener)

// Gateway tạo gRPC client qua bufconn dialer
dcmConn, _ := grpc.NewClient("passthrough:///bufconn",
    grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
        return dcmListener.DialContext(ctx)
    }),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

**Latency**: ~10-50μs per call (protobuf serialize + in-memory copy, no TCP).

### 3.2 RESTful via Internal HTTP — Service ↔ Service (Cross-Domain)

```go
// Mỗi service expose internal REST endpoints trên riêng 1 http.Server (loopback)
// Sử dụng bufconn hoặc random localhost port

// DCM Service internal REST server
dcmHTTPListener := bufconn.Listen(1024 * 1024) // hoặc net.Listen("tcp", "127.0.0.1:0")
dcmRouter := chi.NewRouter() // hoặc echo sub-router
dcmRouter.Get("/internal/v1/plans/{planID}/status", dcmService.GetPlanStatus)
dcmRouter.Post("/internal/v1/plans/{planID}/check", dcmService.TriggerPlanCheck)
go http.Serve(dcmHTTPListener, dcmRouter)

// Admin service calls DCM via REST
resp, _ := dcmHTTPClient.Get("http://dcm/internal/v1/plans/123/status")

// hoặc qua service registry
client := serviceRegistry.RESTClient("dcm")
status, _ := client.Get(ctx, "/internal/v1/plans/123/status")
```

**Use cases cho RESTful cross-service**:
- Admin Service cần check DCM plan status → `GET /internal/v1/plans/{id}/status`
- SQL Service cần resolve project settings → `GET /internal/v1/projects/{id}/settings`
- Gateway health aggregation → `GET /internal/healthz` trên mỗi service
- Debug/inspection → curl-friendly JSON endpoints

### 3.3 NATS — Async Event-Driven (Services → Runners)

```go
import (
    natsserver "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
)

// Embedded NATS server
ns, _ := natsserver.NewServer(&natsserver.Options{
    Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
})
go ns.Start()
ns.ReadyForConnections(5 * time.Second)

// Publish (from services)
nc, _ := nats.Connect(ns.ClientURL())
nc.Publish("bytebase.plancheck.tickle", nil)

// Subscribe (in runners)
nc.Subscribe("bytebase.plancheck.tickle", func(msg *nats.Msg) {
    planCheckScheduler.WakeUp()
})
```

### 3.4 NATSBus — EventBus Interface Adapter

```go
// backend/component/bus/nats_bus.go
// Implements existing EventBus interface → ZERO changes to runners

type NATSBus struct {
    nc             *nats.Conn
    planCheckCh    chan int        // populated by NATS subscriber
    taskRunCh      chan int
    approvalCh     chan IssueRef
    rolloutCh      chan PlanRef
    planCompleteCh chan PlanRef
    cancelFuncs    sync.Map
}

func (b *NATSBus) TicklePlanCheck()                   { b.nc.Publish("bytebase.plancheck.tickle", nil) }
func (b *NATSBus) TickleTaskRun()                      { b.nc.Publish("bytebase.taskrun.tickle", nil) }
func (b *NATSBus) RequestApprovalCheck(ref IssueRef)   { b.nc.Publish("bytebase.approval.check", marshal(ref)) }
func (b *NATSBus) PlanCheckChan() <-chan int            { return b.planCheckCh }
func (b *NATSBus) TaskRunChan() <-chan int              { return b.taskRunCh }
func (b *NATSBus) ApprovalChan() <-chan IssueRef        { return b.approvalCh }
// ... all other EventBus methods
```

## 4. Protocol Decision Matrix

| Communication Path | Protocol | Rationale |
|---|---|---|
| External client → Gateway | ConnectRPC / REST | Existing public API, no change |
| Gateway → DCM/SQL/Admin | **RESTful/ConnectRPC (bufconn HTTP)** | **Giữ nguyên existing handlers** — zero code change cho `api/v1/` |
| DCM → SQL (cross-service query) | **RESTful (internal)** | Simple request, easy to debug, curl-friendly |
| Admin → DCM (cross-service) | **RESTful (internal)** | Lightweight, human-readable |
| DCM → Runner (trigger plancheck) | **NATS (pub/sub)** | Fire-and-forget async event |
| Service → Runner (trigger taskrun) | **NATS (pub/sub)** | Decoupled async processing |
| Runner → Runner (HA coordination) | **NATS (pub/sub)** | Replaces PG LISTEN/NOTIFY for HA |
| Health check (any → any) | **RESTful** | Standard `/healthz` pattern |
| Future: High-perf cross-service | **gRPC (bufconn)** | Available khi cần streaming/binary protocol |

> **Nguyên tắc chính**: Services dùng RESTful/ConnectRPC (existing HTTP handlers) để **không phải thay đổi code nhiều**. gRPC available cho tương lai khi cần performance cao hơn.

## 5. NATS Subject Map

| Subject | Publisher | Subscriber | Replaces |
|---------|-----------|------------|----------|
| `bytebase.plancheck.tickle` | PlanService, IssueService | PlanCheck Scheduler | `planCheckTickleCh` |
| `bytebase.taskrun.tickle` | RolloutService | TaskRun Scheduler | `taskRunTickleCh` |
| `bytebase.approval.check` | PlanCheck Scheduler | Approval Runner | `approvalCheckCh` |
| `bytebase.rollout.create` | Approval Runner | Rollout Creator | `rolloutCreationCh` |
| `bytebase.plan.completion` | TaskRun Scheduler | Webhook Manager | `planCompletionCheckCh` |
| `bytebase.cache.invalidate` | Any Service | Store Cache | PG LISTEN/NOTIFY (HA) |

## 6. Service Domain Mapping

### 6.1 DCM Service — 8 Proto Services
PlanService, IssueService, RolloutService, ReleaseService, RevisionService, ReviewConfigService, AccessGrantService, OrgPolicyService

### 6.2 SQL Service — 8 Proto Services
SQLService, DatabaseService, DatabaseCatalogService, DatabaseGroupService, InstanceService, InstanceRoleService, SheetService, WorksheetService

### 6.3 Admin Service — 15 Proto Services
AuthService, UserService, ServiceAccountService, WorkloadIdentityService, RoleService, GroupService, IdentityProviderService, SettingService, WorkspaceService, ProjectService, SubscriptionService, ActuatorService, AuditLogService, CelService, AIService

## 7. Internal REST API Convention

```
/internal/v1/{service}/{resource}[/{id}][/{action}]

Examples:
  GET    /internal/v1/dcm/plans/123/status
  POST   /internal/v1/dcm/plans/123/trigger-check
  GET    /internal/v1/admin/users/me
  GET    /internal/v1/sql/instances/456/schema
  GET    /internal/healthz
  GET    /internal/metrics
```

- Prefix `/internal/` — **not exposed externally** (gateway does NOT proxy these)
- JSON request/response (simpler than protobuf for cross-service)
- Endpoints defined per service, not centralized

## 8. Transport Abstraction Layer

```go
// backend/transport/transport.go

// ServiceTransport abstracts the communication protocol for service-to-service calls.
type ServiceTransport interface {
    // gRPC connection (for gateway → service high-perf path)
    GRPCConn() *grpc.ClientConn

    // REST client (for cross-service lightweight calls)
    RESTClient() *http.Client

    // Service endpoint (for REST calls)
    BaseURL() string
}

// BufconnTransport — in-process (single binary mode)
type BufconnTransport struct {
    grpcListener *bufconn.Listener
    httpListener *bufconn.Listener
    grpcConn     *grpc.ClientConn
    httpClient   *http.Client
}

// TCPTransport — network (multi-binary mode, future)
type TCPTransport struct {
    grpcAddr string
    httpAddr string
}
```

## 9. Impact Analysis

### New Dependencies
| Dependency | Purpose | Already in go.mod? |
|---|---|---|
| `google.golang.org/grpc/test/bufconn` | In-memory gRPC | ✅ Yes (grpc dep) |
| `github.com/nats-io/nats-server/v2` | Embedded NATS | ❌ New (~15MB) |
| `github.com/nats-io/nats.go` | NATS client | ❌ New (~2MB) |

### Code Change Estimate
| Area | Change | Description |
|---|---|---|
| `backend/api/v1/` | **Low** | Verify gRPC server interface compatibility |
| `backend/component/bus/` | **Medium** | New `NATSBus` + `nats_bus.go` |
| `backend/transport/` (NEW) | **New** | Transport abstraction layer |
| `backend/service/` (NEW) | **New** | Domain services with gRPC + REST servers |
| `backend/gateway/` (NEW) | **New** | Gateway with gRPC proxy |
| `backend/server/server.go` | **Medium** | Init NATS + services + gateway |
| `backend/server/grpc_routes.go` | **High** | Replaced by gateway |
| `backend/runner/` | **None** | Consumes EventBus interface unchanged |
| `frontend/` | **None** | Zero changes |

## 10. Scaling Path

```
Phase 1 (Now):                Phase 2 (Future):
Single Binary                  Multi-Binary
┌──────────────┐               ┌──────────┐    ┌──────────┐
│  Gateway     │               │ Gateway  │    │ DCM Svc  │
│  DCM         │               │ (HTTP)   │    │ (gRPC+   │
│  SQL         │   ──────→     └────┬─────┘    │  REST)   │
│  Admin       │                    │TCP        └────┬─────┘
│  Runner      │               ┌────┴────┐          │TCP
│  NATS(embed) │               │ NATS    │     ┌────┴─────┐
└──────────────┘               │(cluster)│     │ Runner   │
                               └─────────┘     └──────────┘
Transport:                     Transport:
  gRPC: bufconn                  gRPC: TCP
  REST: bufconn                  REST: HTTP
  NATS: embedded                 NATS: cluster
```

**Chỉ cần thay `BufconnTransport` → `TCPTransport` khi extract services ra binary riêng.**

---

# Production-Grade & Enterprise Requirements

## 11. Observability Stack

### 11.1 Distributed Tracing (OpenTelemetry)

OTel SDK đã có trong `go.mod`. Mỗi service tạo spans cho request lifecycle:

```go
// backend/component/otel/tracer.go
import "go.opentelemetry.io/otel"

var Tracer = otel.Tracer("bytebase")

// Trong mỗi service handler:
func (s *PlanService) CreatePlan(ctx context.Context, req *v1pb.CreatePlanRequest) (*v1pb.Plan, error) {
    ctx, span := otel.Tracer("dcm").Start(ctx, "PlanService.CreatePlan")
    defer span.End()
    span.SetAttributes(attribute.String("project.id", req.Parent))
    // ... business logic
}
```

**Trace propagation flow**:
```
Client → Gateway (span: gateway.proxy)
           → DCM Service (span: dcm.PlanService.CreatePlan)
              → Store (span: store.CreatePlan)
              → NATS publish (span: nats.publish.plancheck.tickle)
                 → Runner (span: runner.plancheck.process)
```

| Component | Instrumentation |
|-----------|----------------|
| Gateway HTTP | `otelhttp.NewHandler()` middleware |
| gRPC/ConnectRPC | `otelconnect.NewInterceptor()` |
| NATS pub/sub | Manual span injection via `msg.Header` |
| SQL queries | `otelsql.Open()` driver wrapper |
| HTTP client (cross-service) | `otelhttp.NewTransport()` |

### 11.2 Metrics (Prometheus)

Hệ thống hiện có Prometheus registry. Thêm per-service metrics:

```go
// backend/component/metrics/service_metrics.go

type ServiceMetrics struct {
    RequestTotal    *prometheus.CounterVec   // {service, method, status}
    RequestDuration *prometheus.HistogramVec // {service, method}
    ActiveRequests  *prometheus.GaugeVec     // {service}
    NATSPublished   *prometheus.CounterVec   // {subject}
    NATSSubscribed  *prometheus.CounterVec   // {subject}
    ErrorTotal      *prometheus.CounterVec   // {service, error_code}
    CircuitState    *prometheus.GaugeVec     // {service, state}
}
```

**Key metrics đảm bảo SLI/SLO**:

| Metric | Type | Labels | SLO Target |
|--------|------|--------|------------|
| `bytebase_request_duration_seconds` | Histogram | service, method | p99 < 500ms |
| `bytebase_request_total` | Counter | service, method, status | error_rate < 0.1% |
| `bytebase_active_requests` | Gauge | service | < 1000 concurrent |
| `bytebase_nats_messages_total` | Counter | subject, direction | delivery_rate > 99.9% |
| `bytebase_circuit_breaker_state` | Gauge | service | open_duration < 30s |
| `bytebase_health_status` | Gauge | check | healthy = 1.0 |

### 11.3 Structured Logging

```go
// backend/component/log/context.go
// Attach service context to every log entry

func WithService(ctx context.Context, service string) context.Context {
    return context.WithValue(ctx, serviceKey, service)
}

// Structured log fields:
// {
//   "level": "info",
//   "ts": "2026-05-15T18:52:00Z",
//   "service": "dcm",
//   "method": "CreatePlan",
//   "trace_id": "abc123",
//   "span_id": "def456",
//   "duration_ms": 42,
//   "status": "OK",
//   "user": "user@example.com"
// }
```

## 12. Resilience Patterns

### 12.1 Circuit Breaker (Gateway → Services)

```go
// backend/gateway/circuitbreaker.go
import "github.com/sony/gobreaker"

type ServiceCircuitBreakers struct {
    dcm   *gobreaker.CircuitBreaker
    sql   *gobreaker.CircuitBreaker
    admin *gobreaker.CircuitBreaker
}

func NewCircuitBreakers(metrics *ServiceMetrics) *ServiceCircuitBreakers {
    settings := gobreaker.Settings{
        Name:        "dcm",
        MaxRequests: 5,              // Half-open: allow 5 test requests
        Interval:    30 * time.Second, // Reset counts every 30s
        Timeout:     10 * time.Second, // Open → Half-open after 10s
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 10 && failureRatio >= 0.5
        },
        OnStateChange: func(name string, from, to gobreaker.State) {
            slog.Warn("circuit breaker state change",
                "service", name, "from", from, "to", to)
            metrics.CircuitState.WithLabelValues(name, to.String()).Set(1)
        },
    }
    return &ServiceCircuitBreakers{
        dcm:   gobreaker.NewCircuitBreaker(settings),
        sql:   gobreaker.NewCircuitBreaker(withName(settings, "sql")),
        admin: gobreaker.NewCircuitBreaker(withName(settings, "admin")),
    }
}
```

### 12.2 Timeout & Deadline Propagation

```go
// Gateway enforces per-route timeouts
var routeTimeouts = map[string]time.Duration{
    "/bytebase.v1.SQLService/Execute":     120 * time.Second, // Long SQL execution
    "/bytebase.v1.SQLService/ExportData":   300 * time.Second, // Data export
    "/bytebase.v1.RolloutService/":         60 * time.Second,  // Default
    "default":                              30 * time.Second,
}

// Context deadline propagated through all layers:
// Gateway (30s) → Service handler (inherits) → Store query (inherits)
```

### 12.3 Retry with Backoff (NATS)

```go
// NATSBus retry for failed publishes
func (b *NATSBus) publishWithRetry(subject string, data []byte) error {
    return retry.Do(
        func() error { return b.nc.Publish(subject, data) },
        retry.Attempts(3),
        retry.Delay(100*time.Millisecond),
        retry.DelayType(retry.BackOffDelay),
        retry.OnRetry(func(n uint, err error) {
            slog.Warn("NATS publish retry", "subject", subject, "attempt", n, "err", err)
        }),
    )
}
```

### 12.4 Graceful Degradation

```go
// If a non-critical service is down, gateway returns partial results
func (g *Gateway) proxyWithFallback(w http.ResponseWriter, r *http.Request, svc string) {
    result, err := g.circuitBreakers.Execute(svc, func() (any, error) {
        return g.proxy(r, svc)
    })
    if err != nil {
        if errors.Is(err, gobreaker.ErrOpenState) {
            // Service unavailable — return 503 with retry-after
            w.Header().Set("Retry-After", "10")
            http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
            return
        }
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    // Forward response
}
```

## 13. Security

### 13.1 Internal Service Authentication

```go
// Internal traffic uses shared HMAC token (not mTLS — overkill for single binary)
const internalAuthHeader = "X-Bytebase-Internal-Auth"

// Gateway sets token on all proxied requests
func (g *Gateway) addInternalAuth(req *http.Request) {
    token := hmac.Sign(g.internalSecret, time.Now().Unix())
    req.Header.Set(internalAuthHeader, token)
}

// Services validate internal auth
func InternalAuthMiddleware(secret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !hmac.Verify(secret, r.Header.Get(internalAuthHeader)) {
                http.Error(w, "unauthorized internal request", 403)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### 13.2 Audit Trail

```go
// Every cross-service call is logged with:
// - Caller service name
// - Target service/method
// - User context (propagated from gateway)
// - Trace ID
// - Timestamp
// - Result status

// Existing AuditInterceptor already captures external API calls.
// Internal calls add X-Bytebase-Caller header for traceability.
```

### 13.3 Secret Management

| Secret | Storage | Rotation |
|--------|---------|----------|
| Auth JWT secret | Profile config / env var | Manual restart |
| Internal HMAC secret | Derived from JWT secret | Automatic with JWT |
| NATS auth token | Generated at startup | Per-restart |
| DB connection string | Profile config | Manual |

## 14. Health Management

### 14.1 Deep Health Check (per-service)

```go
// Each domain service exposes /internal/healthz with deep checks

type ServiceHealth struct {
    Service    string            `json:"service"`
    Status     health.Status     `json:"status"`
    Uptime     time.Duration     `json:"uptime"`
    Checks     []health.CheckResult `json:"checks"`
    Version    string            `json:"version"`
    GoRoutines int               `json:"goroutines"`
}

// DCM health checks:
// - PostgreSQL connectivity (critical)
// - NATS connectivity (critical)
// - PlanCheck scheduler alive (non-critical)
// - TaskRun scheduler alive (non-critical)

// SQL health checks:
// - PostgreSQL connectivity (critical)
// - External DB connectivity (non-critical, per-instance)

// Admin health checks:
// - PostgreSQL connectivity (critical)
// - License service valid (non-critical)
```

### 14.2 Gateway Health Aggregation

```go
// GET /healthz — aggregated health from all services

func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
    results := make(map[string]ServiceHealth)
    var overallStatus = health.StatusHealthy

    for _, svc := range g.services.AllServices() {
        resp, err := g.internalClient(svc.Name()).Get("/internal/healthz")
        if err != nil {
            results[svc.Name()] = ServiceHealth{Status: health.StatusUnhealthy}
            overallStatus = health.StatusUnhealthy
            continue
        }
        // Parse and aggregate
    }

    // Kubernetes-compatible response
    if overallStatus == health.StatusUnhealthy {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    json.NewEncoder(w).Encode(map[string]any{
        "status":   overallStatus,
        "services": results,
    })
}

// GET /readyz — readiness probe (can accept traffic?)
// GET /livez  — liveness probe (should restart?)
```

### 14.3 NATS Health Monitoring

```go
// Check NATS embedded server health
func (b *NATSBus) HealthCheck() health.CheckResult {
    if !b.ns.Running() {
        return health.CheckResult{Status: health.StatusUnhealthy, Message: "NATS not running"}
    }
    info := b.ns.Connz(nil)
    return health.CheckResult{
        Status:  health.StatusHealthy,
        Message: fmt.Sprintf("connections=%d, in_msgs=%d", info.NumConns, info.InMsgs),
    }
}
```

## 15. Error Handling Standards

### 15.1 Standardized Error Codes

```go
// backend/component/errors/codes.go

type ServiceError struct {
    Code       string `json:"code"`        // "DCM_PLAN_NOT_FOUND"
    Message    string `json:"message"`     // Human-readable
    Service    string `json:"service"`     // "dcm"
    TraceID    string `json:"trace_id"`    // OTel trace ID
    RetryAfter int    `json:"retry_after"` // Seconds (0 = not retryable)
}

// Error code prefixes by service:
// DCM_*   — Database Change Management errors
// SQL_*   — Query/Instance errors
// ADMIN_* — Auth/Access errors
// GW_*    — Gateway errors (circuit open, timeout, etc.)
// NATS_*  — Messaging errors
```

### 15.2 Error Propagation

```
Service error → Gateway → Client
    │
    ├─ 4xx → forward as-is (client error)
    ├─ 5xx → wrap with GW_ prefix + trace_id
    ├─ timeout → 504 + retry_after
    └─ circuit open → 503 + retry_after
```

### 15.3 Panic Recovery (per-service)

```go
// Each service HTTP server has panic recovery middleware
func PanicRecoveryMiddleware(serviceName string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    slog.Error("panic recovered",
                        "service", serviceName,
                        "error", err,
                        "stack", string(debug.Stack()),
                    )
                    metrics.PanicTotal.WithLabelValues(serviceName).Inc()
                    http.Error(w, "internal server error", 500)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

## 16. Configuration Management

### 16.1 Service Configuration

```go
// backend/component/config/service_config.go

type ServiceConfig struct {
    // Per-service timeouts
    Timeouts map[string]time.Duration `json:"timeouts"`

    // Circuit breaker settings
    CircuitBreaker struct {
        MaxRequests uint32        `json:"max_requests"`
        Interval    time.Duration `json:"interval"`
        Timeout     time.Duration `json:"timeout"`
        FailureRate float64       `json:"failure_rate"`
    } `json:"circuit_breaker"`

    // NATS settings
    NATS struct {
        MaxReconnects  int           `json:"max_reconnects"`
        ReconnectWait  time.Duration `json:"reconnect_wait"`
        MaxPendingMsgs int          `json:"max_pending_msgs"`
    } `json:"nats"`

    // Observability
    Observability struct {
        TraceSampleRate float64 `json:"trace_sample_rate"` // 0.0-1.0
        MetricsEnabled  bool    `json:"metrics_enabled"`
        LogLevel        string  `json:"log_level"`
    } `json:"observability"`
}
```

### 16.2 Feature Flags (Enterprise)

```go
// Runtime feature flags for gradual rollout
type FeatureFlags struct {
    UseNATSBus        bool `json:"use_nats_bus"`         // false = fallback to Go channels
    EnableTracing     bool `json:"enable_tracing"`
    EnableCircuitBreaker bool `json:"enable_circuit_breaker"`
    InternalAuthEnabled  bool `json:"internal_auth_enabled"`
}

// Default: all false (safe rollout)
// Enable one-by-one via env vars or settings
```

## 17. Operational Runbook

### 17.1 Startup Sequence

```
1. Load config + profile
2. Start embedded PostgreSQL (if needed)
3. Connect to PostgreSQL, run migrations
4. Initialize shared infra (store, IAM, license, webhook, dbFactory)
5. Start embedded NATS server
6. Create NATSBus (EventBus adapter)
7. Build interceptor chain
8. Create domain services (DCM, SQL, Admin) → Start internal HTTP servers
9. Create runner service → Start background goroutines
10. Create gateway → Register routes, reverse proxy
11. Start external HTTP server (port 8080)
12. Register Prometheus metrics, health checks
13. Send heartbeat: status=RUNNING
```

### 17.2 Shutdown Sequence

```
1. Heartbeat: status=DRAINING
2. Stop accepting new HTTP connections
3. Drain in-flight requests (30s timeout)
4. Cancel context → signal all goroutines
5. RunnerService.Wait() — wait for runners to finish
6. Stop domain services (internal HTTP servers)
7. NATSBus.Shutdown() — close NATS connections, stop embedded server
8. Close store, pool manager, PG stoppers
9. Heartbeat: status=STOPPED
```

### 17.3 Failure Modes & Recovery

| Failure | Detection | Recovery |
|---------|-----------|----------|
| Service HTTP server crash | Circuit breaker opens | Auto-restart via panic recovery |
| NATS embedded crash | Health check fails | Fallback to Go channels bus |
| PostgreSQL connection loss | Health check critical | Auto-reconnect (pgx pool) |
| Memory pressure > 2.5GB | Memory health check | GC pressure + alert |
| Deadlocked runner | Heartbeat timeout | Self-heal runner restarts |
| Certificate expiry | Startup check | Alert 30 days before |

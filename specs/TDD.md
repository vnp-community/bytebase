# Technical Design Document (TDD)
# Bytebase — Database CI/CD Platform

| Metadata       | Value                              |
|----------------|------------------------------------|
| Product        | Bytebase                          |
| Document Date  | 2026-05-08                        |
| Source         | Source code deep analysis          |
| Go Version     | 1.26                              |
| Proto Tooling  | buf.build                         |

---

## 1. Design Philosophy

### 1.1 Core Principles
1. **Protobuf-first API** — Tất cả API contracts được định nghĩa trong `.proto` files. Code Go, TypeScript, REST gateway đều auto-generated từ proto.
2. **Plugin-based extensibility** — Database drivers, SQL advisors, identity providers đều là plugins có interface chuẩn hóa.
3. **Event-driven coordination** — Các background runners giao tiếp qua internal message bus (Go channels), PostgreSQL LISTEN/NOTIFY cho cross-instance.
4. **Embedded-first deployment** — Server đóng gói frontend SPA + embedded PostgreSQL, tạo single-binary deployment.
5. **Feature-gated licensing** — Enterprise features được kiểm soát qua license key, plan.yaml define feature matrix.

### 1.2 Monolith Architecture Choice
Bytebase chọn kiến trúc **modular monolith** thay vì microservices:
- Single Go binary chứa tất cả services
- Single PostgreSQL database cho metadata
- Separation of concerns qua Go packages (không qua network boundaries)
- Background runners chạy trong cùng process, giao tiếp qua channels

---

## 2. Server Bootstrap Sequence

```
main.go
  └─ NewServer(ctx, profile)
       ├─ 1. StartMetadataInstance() [if embedded PG]
       ├─ 2. store.New(pgURL) → PostgreSQL connection + LRU caches
       ├─ 3. migrator.MigrateSchema() → self-migration
       ├─ 4. enterprise.NewLicenseService() → license validation
       ├─ 5. iam.NewManager() → RBAC engine + cache
       ├─ 6. webhook.NewManager() → notification dispatcher
       ├─ 7. dbfactory.New() → database connection factory
       ├─ 8. echo.New() → HTTP server
       ├─ 9. Initialize Runners:
       │     ├─ schemasync.NewSyncer()
       │     ├─ approval.NewRunner()
       │     ├─ taskrun.NewScheduler() + Register executors
       │     ├─ plancheck.NewScheduler()
       │     ├─ notifylistener.NewListener()
       │     ├─ cleaner.NewDataCleaner()
       │     └─ heartbeat.NewRunner()
       ├─ 10. Initialize Protocol Servers:
       │     ├─ lsp.NewServer()
       │     ├─ directorysync.NewService()
       │     ├─ oauth2.NewService()
       │     ├─ mcp.NewServer()
       │     └─ stripeapi.NewWebhookHandler()
       ├─ 11. [DUAL MODE] Route Configuration:
       │     ├─ [BB_USE_GATEWAY=true] Gateway Mode:
       │     │   ├─ initGatewayServices() → DCM + SQL + Admin + Runner services
       │     │   ├─ registry.StartAll() → Start bufconn HTTP servers
       │     │   └─ configureGatewayRouters() → Reverse proxy routing
       │     └─ [default] Legacy Mode:
       │         └─ configureGrpcRouters() → Monolithic 31-handler registration
       └─ 12. configureEchoRouters() → HTTP middleware + protocol routes

  └─ Server.Run(ctx, port)
       ├─ [BB_USE_GATEWAY=true]: runnerService.Run(ctx)
       ├─ [HA mode]: Leader-elected runners + shared runners
       ├─ [default]: Start 8+ background goroutines (via runnerWG)
       ├─ net.Listen("tcp", :port)
       └─ httpServer.Serve() [H2C handler]

  └─ Server.Shutdown(ctx)
       ├─ heartbeat.SetStatus("DRAINING") + SendHeartbeat
       ├─ httpServer.Shutdown() → drain connections
       ├─ cancel() → signal runners
       ├─ [BB_USE_GATEWAY=true]: runnerService.Wait() + registry.StopAll()
       ├─ [default]: runnerWG.Wait()
       ├─ heartbeat.SetStatus("STOPPED") + SendHeartbeat
       ├─ poolManager.Close()
       ├─ store.Close()
       └─ stopper() → embedded PG cleanup
```

---

## 3. Request Processing Pipeline

### 3.1 Interceptor Chain

Mỗi gRPC/ConnectRPC request đi qua 4 interceptors theo thứ tự:

```
Client Request
  │
  ▼
┌─────────────────────────────────────┐
│ 1. Validate Interceptor              │  ← protovalidate: check field constraints
│    (connectrpc.com/validate)         │
├─────────────────────────────────────┤
│ 2. Auth Interceptor                  │  ← Extract JWT/Cookie/API-key
│    (backend/api/auth/)               │     Validate signature (HMAC-SHA256)
│                                      │     Load UserMessage from store
│                                      │     Set ctx: user, workspace, token
├─────────────────────────────────────┤
│ 3. ACL Interceptor                   │  ← Resolve resource from request
│    (backend/api/v1/acl.go)           │     Check IAM permission via Manager
│    19,310 bytes                      │     Workspace-level → Project-level
│                                      │     CEL condition evaluation
├─────────────────────────────────────┤
│ 4. Audit Interceptor                 │  ← Log request metadata
│    (backend/api/v1/audit.go)         │     Capture response status
│    25,157 bytes                      │     Write AuditLog entry async
└─────────────────────────────────────┘
  │
  ▼
Service Handler → Store → Response
```

### 3.2 Dual Transport: ConnectRPC + REST Gateway

```
                    ┌──── ConnectRPC Handler ───► Service Implementation
Client ──► Echo ──►│
                    └──── gRPC-Gateway (REST) ─► gRPC Client ─► ConnectRPC Handler
```

- **ConnectRPC** (`/bytebase.v1.*/`): Native protocol, HTTP/2 + HTTP/1.1 compatible
- **REST Gateway** (`/v1/*`): Auto-generated từ proto annotations, proxy sang ConnectRPC qua loopback gRPC connection
- **WebSocket** (`/v1:adminExecute`): Streaming SQL execution qua `wsproxy`

---

## 4. Data Access Layer (Store)

### 4.1 Store Architecture

```go
type Store struct {
    dbConnManager *DBConnectionManager  // PostgreSQL connection via pgx/v5
    enableCache   bool                  // HA mode disables cache

    // LRU Caches (no-expiry, explicit invalidation)
    userEmailCache  *lru.Cache[string, *UserMessage]     // 32,768 entries
    instanceCache   *lru.Cache[string, *InstanceMessage]  // 32,768 entries
    databaseCache   *lru.Cache[string, *DatabaseMessage]  // 32,768 entries
    projectCache    *lru.Cache[string, *ProjectMessage]   // 32,768 entries
    policyCache     *lru.Cache[string, *PolicyMessage]    // 4,096 entries
    settingCache    *lru.Cache[string, *SettingMessage]    // 1,024 entries
    sheetFullCache  *lru.Cache[string, *SheetMessage]     // 10 entries (large)

    // TTL Caches (auto-expiry)
    rolesCache        *expirable.LRU  // 128, TTL=1min
    groupCache        *expirable.LRU  // 1,024, TTL=1min
    dbSchemaCache     *expirable.LRU  // 128, TTL=5min
    iamPolicyCache    *expirable.LRU  // 1,024, TTL=1min
}
```

### 4.2 Cache Strategy
- **HA mode**: Cache disabled (`enableCache=false`) — mỗi request đọc trực tiếp từ DB
- **Single-node mode**: Cache enabled — write-through invalidation
- **Cache key format**: `workspaces/{workspace}/{resource_type}/{resource_id}`
- **Group-related caches**: 3 caches liên kết (`group`, `groupMembers`, `memberGroups`) — purge đồng thời

### 4.3 Composite Primary Keys

Nhiều bảng sử dụng composite PK `(project, id)`:

```sql
-- Ví dụ từ LATEST.sql
CREATE TABLE plan (
    project TEXT NOT NULL,
    id BIGSERIAL,
    PRIMARY KEY (project, id),
    ...
);
```

**Tables với composite PK**: `plan`, `issue`, `task`, `task_run`, `plan_check_run`, `release`, `db_group`, `plan_webhook_delivery`, `task_run_log`

**Design rationale**: Hỗ trợ project-level data isolation, tránh collision khi merge data across projects.

### 4.4 JSONB Storage Pattern

Các trường phức tạp được lưu dưới dạng Protocol Buffers JSON (camelCase):

```go
// Go code marshal
data, _ := protojson.Marshal(protoMessage)
// Stored as JSONB: {"taskRun": {...}, "planConfig": {...}}
// NOT snake_case: {"task_run": {...}}
```

**Affected columns**: `plan.config`, `task.payload`, `task_run.result`, `policy.payload`, `setting.value`, `issue.payload`

---

## 5. Background Runner System

### 5.1 Message Bus Design

```go
type Bus struct {
    ApprovalCheckChan       chan IssueRef      // buffer: 1000
    PlanCheckTickleChan     chan int            // buffer: 1000
    TaskRunTickleChan       chan int            // buffer: 1000
    RolloutCreationChan     chan PlanRef        // buffer: 100
    PlanCompletionCheckChan chan PlanRef        // buffer: 1000

    RunningTaskRunsCancelFunc      sync.Map    // TaskRunRef → CancelFunc
    RunningPlanCheckRunsCancelFunc sync.Map    // PlanCheckRunRef → CancelFunc
}
```

**Design decision**: Buffered Go channels thay vì external message queue — đơn giản, low-latency, phù hợp monolith. Trade-off: messages mất khi server crash.

### 5.2 Task Execution Pipeline

```
Plan Created/Updated
  │
  ├─► Bus.PlanCheckTickleChan ─► PlanCheck Scheduler
  │     └─ CombinedExecutor.Run()
  │         ├─ SQL Review (advisor rules)
  │         ├─ Statement type check
  │         └─ Schema compatibility check
  │
  ├─► Bus.ApprovalCheckChan ─► Approval Runner
  │     └─ Find matching approval template
  │         └─ Create/update approval flow
  │
  └─► [After approval] Bus.RolloutCreationChan ─► Rollout Creator
        └─ Create Rollout with Stages/Tasks
            │
            └─► Bus.TaskRunTickleChan ─► TaskRun Scheduler
                  ├─ PendingScheduler: pick pending tasks
                  │    └─ Check environment policy, dependencies
                  └─ RunningScheduler: execute task
                       └─ Executor.RunOnce(ctx, driverCtx, task, taskRunUID)
                            ├─ DATABASE_CREATE  → DatabaseCreateExecutor
                            ├─ DATABASE_MIGRATE → DatabaseMigrateExecutor
                            └─ DATABASE_EXPORT  → DataExportExecutor
```

### 5.3 Task Executor Interface

```go
type Executor interface {
    RunOnce(ctx context.Context, driverCtx context.Context,
            task *store.TaskMessage, taskRunUID int64) (*storepb.TaskRunResult, error)
}
```

- `ctx`: lifecycle context, cancelled khi server shutdown
- `driverCtx`: database driver context, có thể cancel riêng để abort query mà vẫn cleanup migration history
- Panic recovery qua `RunExecutorOnce()` wrapper

### 5.4 PostgreSQL LISTEN/NOTIFY

`NotifyListener` sử dụng PG native pub/sub cho cross-component notification:

```
PG NOTIFY 'bytebase:plan_check' ─► NotifyListener ─► Bus.PlanCheckTickleChan
PG NOTIFY 'bytebase:task_run'   ─► NotifyListener ─► Bus.TaskRunTickleChan
```

**Use case**: Khi store ghi dữ liệu, trigger PG NOTIFY để runners phản hồi ngay lập tức thay vì đợi polling interval.

---

## 6. Plugin System Design

### 6.1 Database Driver Plugin

```go
// Registration pattern (init-time)
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}

// Interface
type Driver interface {
    Open(ctx, dbType, config) (Driver, error)
    Close(ctx) error
    Ping(ctx) error
    GetDB() *sql.DB
    Execute(ctx, statement, opts) (int64, error)
    QueryConn(ctx, conn, statement, queryContext) ([]*QueryResult, error)
    SyncInstance(ctx) (*InstanceMetadata, error)
    SyncDBSchema(ctx) (*DatabaseSchemaMetadata, error)
    Dump(ctx, out, dbMetadata) error
}
```

**22 driver implementations**: Mỗi driver trong `backend/plugin/db/<engine>/` tự register qua `init()`. Factory pattern qua `db.Open()`.

### 6.2 SQL Advisor Plugin

```go
// Core interface
type Advisor interface {
    Check(ctx context.Context, ...) ([]*Advice, error)
}

// Engine-specific rules: backend/plugin/advisor/<engine>/
// 9 engine rule sets: pg, mysql, tidb, oracle, mssql, snowflake, oceanbase, redshift
```

**200+ lint rules** phân theo categories: naming, statement type, table design, column, index, performance, security.

### 6.3 Identity Provider Plugin

```
backend/plugin/idp/
  ├── OIDC  (OpenID Connect)
  ├── SAML  (Security Assertion Markup Language)
  └── LDAP  (Lightweight Directory Access Protocol)
```

---

## 7. IAM & Authorization Design

### 7.1 Two-level Permission Model

```
Workspace IAM Policy
  ├── Binding: role=roles/workspaceAdmin, members=[users/admin@...]
  ├── Binding: role=roles/workspaceDBA, members=[users/dba@...]
  └── Binding: role=roles/workspaceDeveloper, members=[allUsers]

Project IAM Policy (per project)
  ├── Binding: role=roles/projectOwner, members=[users/lead@...]
  ├── Binding: role=roles/projectDeveloper, members=[groups/dev-team]
  └── Binding: role=roles/projectQuerier, members=[users/analyst@...]
```

### 7.2 Permission Check Flow

```go
func (m *Manager) CheckPermission(ctx, permission, user, workspaceID, projectIDs...) (bool, error) {
    // 1. Check workspace-level IAM policy
    //    - SaaS mode: skip allUsers for workspace-level (explicit membership required)
    //    - Self-hosted: allUsers means all authenticated users

    // 2. If projectIDs provided, check project-level IAM policy
    //    - User must have permission on ALL specified projects
    //    - allUsers at project level = all workspace members (always safe)

    // 3. Role → Permissions mapping via store.GetRoleSnapshot()
    //    - Predefined roles: workspaceAdmin, workspaceDBA, projectOwner, etc.
    //    - Custom roles (Enterprise): user-defined permission sets

    // 4. Group membership expansion
    //    - groups/{email} → resolve member list from store
}
```

### 7.3 Principal Types

```go
switch user.Type {
case PrincipalType_SERVICE_ACCOUNT:   → "serviceAccounts/{email}"
case PrincipalType_WORKLOAD_IDENTITY: → "workloadIdentities/{email}"
default:                               → "users/{email}"
}
```

---

## 8. Schema Migration Pipeline (for managed databases)

### 8.1 Migration Execution Flow

```
DatabaseMigrateExecutor.RunOnce()
  │
  ├─ 1. Parse SQL statements (ANTLR parser per engine)
  ├─ 2. Schema dump BEFORE migration (Dump())
  ├─ 3. FOR each SQL command:
  │     ├─ Log: COMMAND_EXECUTE (statement or range)
  │     ├─ Execute via driver.Execute()
  │     ├─ Log: COMMAND_RESPONSE (affected rows, error)
  │     └─ Retry logic (if lock timeout, up to MaximumRetries)
  ├─ 4. Schema dump AFTER migration (Dump())
  ├─ 5. Compute diff (before vs after)
  ├─ 6. Database sync (SyncDBSchema)
  ├─ 7. Create changelog entry
  └─ 8. Return TaskRunResult
```

### 8.2 Online Schema Change (gh-ost)

Cho MySQL: sử dụng `github/gh-ost` (fork: `bytebase/gh-ost2`) để thực hiện schema change không downtime:
- Tạo ghost table → copy data → swap tables
- Component: `backend/component/ghost/`

---

## 9. Data Masking Engine

### 9.1 Masking Pipeline

```
SQL Query → Parse AST → Identify columns → Evaluate masking policy → Apply masking → Return results
```

### 9.2 Masking Evaluation

```
MaskingEvaluator
  ├─ Input: column metadata, user context, access grants
  ├─ Check: DatabaseCatalog masking rules
  ├─ Check: OrgPolicy masking rules
  ├─ Check: Data classification → auto-masking
  ├─ Check: Access grants (unmask=true overrides)
  └─ Output: masking level per column (NONE, PARTIAL, FULL)
```

### 9.3 Document Masking (NoSQL)

`document_masking.go` (44KB) — xử lý masking cho:
- MongoDB documents (nested JSON paths)
- CosmosDB documents
- Elasticsearch documents

---

## 10. Frontend Architecture

### 10.1 Hybrid Vue/React Stack

```
frontend/src/
  ├── Vue 3 Layer (legacy, being migrated)
  │     ├── components/     (Vue SFC components)
  │     ├── composables/    (Vue composition API hooks)
  │     ├── store/modules/  (Pinia stores)
  │     ├── views/          (Vue page components)
  │     └── router/         (Vue Router)
  │
  ├── React 19 Layer (new code)
  │     ├── components/ui/  (shadcn-style, Base UI primitives)
  │     ├── hooks/          (React hooks)
  │     └── pages/          (React page components)
  │
  └── Bridge Layer
        └── useVueState(getter)  — React hook subscribing to Vue reactive state
            via useSyncExternalStore
```

### 10.2 API Client

```
@connectrpc/connect-web → ConnectRPC transport
@bufbuild/protobuf      → Protobuf serialization
```

Frontend gọi API qua ConnectRPC client (generated từ proto), không dùng REST.

### 10.3 Code Editor

Monaco Editor (via `@codingame/monaco-vscode-api`):
- LSP integration qua WebSocket (`/lsp` endpoint)
- Auto-complete từ schema metadata
- SQL syntax highlighting per engine
- WASM modules cho TextMate grammar

---

## 11. Security Architecture

### 11.1 Authentication Token Flow

```
Login → JWT access token (short-lived) + Refresh token (DB-stored)
  │
  ├─ Access token: HMAC-SHA256, stored in Cookie (HttpOnly, SameSite=Lax)
  ├─ Refresh token: stored in web_refresh_token table
  └─ Service Account: API key in Authorization header
```

### 11.2 HTTP Security Headers

```
Content-Security-Policy: default-src 'self'; script-src 'self' {hashes} 'wasm-unsafe-eval'; ...
X-Frame-Options: SAMEORIGIN
X-Content-Type-Options: nosniff
Strict-Transport-Security: max-age=31536000; includeSubDomains
Cross-Origin-Opener-Policy: same-origin-allow-popups
```

CSP hashes được generate bởi `vite-plugin-export-csp-hashes.ts` at build time.

### 11.3 Panic Recovery

Hai lớp panic recovery:
1. **Echo middleware** (`recoverMiddleware`): catch HTTP handler panics
2. **ConnectRPC `WithRecover`**: catch gRPC handler panics với stack trace logging
3. **TaskExecutor wrapper** (`RunExecutorOnce`): catch executor panics

---

## 12. Self-Migration System

### 12.1 Bytebase Metadata Schema

Bytebase tự quản lý schema migration cho metadata database của chính nó:

```
backend/migrator/
  ├── migrator.go           → Migration engine
  ├── migration/
  │     ├── LATEST.sql      → Current complete schema
  │     └── <version>/      → Incremental migration files
  └── migrator_test.go      → TestLatestVersion validation
```

### 12.2 Migration Process

```
Server Start → migrator.MigrateSchema()
  ├─ Read current schema version from DB
  ├─ Find pending migration files
  ├─ Execute migrations in order
  ├─ Update schema version
  └─ Validate against LATEST.sql
```

---

## 13. Observability

| Component           | Tool                              | Endpoint/Location           |
|--------------------|-----------------------------------|-----------------------------|
| Metrics            | Prometheus client_golang          | `GET /metrics`              |
| Health check       | Echo handler                      | `GET /healthz`              |
| Request logging    | Echo RequestLogger middleware     | Error-only logging          |
| Audit logging      | Custom AuditInterceptor           | `audit_log` table           |
| Memory monitoring  | monitor.MemoryMonitor runner      | Background goroutine        |
| Debug profiling    | net/http/pprof                    | Conditional on RuntimeDebug |
| Telemetry          | Custom reporter                   | Anonymous usage metrics     |
| DB query tracing   | Custom db_tracer.go               | SQL query timing            |

---

## 14. Key Design Decisions & Trade-offs

| Decision                                    | Rationale                                              | Trade-off                              |
|--------------------------------------------|--------------------------------------------------------|----------------------------------------|
| Monolith over microservices                 | Simpler deployment, no network overhead                | Horizontal scaling limited to HA mode  |
| ConnectRPC over pure gRPC                   | HTTP/1.1 compatibility, browser-friendly               | Extra proxy hop for REST gateway       |
| Go channels for message bus                 | Zero external dependency, low latency                  | Messages lost on crash                 |
| Embedded PostgreSQL option                  | Zero-dependency quick start                            | Not suitable for production HA         |
| Composite primary keys                      | Project-level data isolation                           | Complex query patterns, easy to miss   |
| LRU cache in-process                        | Simple, fast for single-node                           | Must disable in HA mode                |
| ANTLR4 for SQL parsing                      | Industry standard, multi-engine support                | Large binary size, complex grammar     |
| Vue→React migration (gradual)               | Modern ecosystem, better TypeScript support            | Dual framework complexity during migration |
| Protobuf JSON in JSONB columns              | Type-safe serialization, schema evolution              | camelCase vs snake_case confusion      |

---

## 12. Service Architecture Design (Gateway + Services)

### 12.1 Design Rationale

The monolithic `grpc_routes.go` (420 lines) and runner management scattered across `server.go` creates tight coupling. The Gateway + Services architecture decomposes this into isolated domain services while keeping the single-binary deployment model.

### 12.2 Key Interfaces

```go
// DomainService — lifecycle interface for all domain services.
type DomainService interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Listener() net.Listener      // bufconn for single-binary
    HTTPClient() *http.Client    // pre-configured for bufconn
    Healthy(ctx context.Context) error
}

// RunnerService — manages all background runners.
type RunnerService interface {
    Run(ctx context.Context)     // Start all runners (non-blocking)
    Wait()                       // Block until all finish
}

// Registry — service lifecycle management.
type Registry struct {
    services map[string]DomainService
    runner   RunnerService
}
```

### 12.3 Communication Patterns

| From → To | Method | Notes |
|-----------|--------|-------|
| Gateway → Domain Service | bufconn HTTP (ReverseProxy) | In-memory, zero network overhead |
| Service → Store | Direct Go call | Same process, same store instance |
| Service → Service | **Forbidden** | Architecture boundary test enforced |
| Runner → EventBus | Go channels / NATS | Async notifications |
| Frontend → Gateway | TCP HTTP/2 (H2C) | External boundary |

### 12.4 Dual-Mode Operation

| Feature | Legacy Mode (default) | Gateway Mode (`BB_USE_GATEWAY=true`) |
|---------|----------------------|--------------------------------------|
| Router | `configureGrpcRouters()` | `configureGatewayRouters()` |
| Service creation | Inline in grpc_routes.go | `initGatewayServices()` |
| Runner lifecycle | `runnerWG` + manual goroutines | `RunnerService.Run()` / `Wait()` |
| Shutdown | `runnerWG.Wait()` | `runnerService.Wait()` + `registry.StopAll()` |
| Interceptors | Inline | `gateway.BuildInterceptorChain()` |

### 12.5 Architecture Decision Records

1. **Feature flag rollout** — Gateway mode is behind `BB_USE_GATEWAY` to allow safe A/B testing and instant rollback
2. **No cross-service imports** — Enforced by `architecture_test.go`; services only share `store`, `component/`, and `api/v1/`
3. **bufconn over gRPC** — Using HTTP + bufconn instead of direct gRPC for maximum flexibility with REST/Connect/gRPC protocols
4. **Same interceptor chain** — `BuildInterceptorChain()` guarantees identical auth/ACL/audit behavior

---

> **Document generated**: 2026-05-08 — Updated 2026-05-15 with Gateway + Services architecture.

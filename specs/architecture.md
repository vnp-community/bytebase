# Architecture Document — Functional Layer Analysis
# Bytebase — Database CI/CD Platform

| Metadata       | Value                    |
|----------------|--------------------------|
| Document Date  | 2026-05-08               |
| Source         | Source code analysis      |

---

## 1. Layered Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  L1 — PRESENTATION LAYER                                            │
│  Frontend SPA (Vue 3 + React 19) + Protocol Adapters (LSP/MCP)      │
├─────────────────────────────────────────────────────────────────────┤
│  L2 — API GATEWAY LAYER                                             │
│  Echo v5 HTTP Server + ConnectRPC + gRPC-Gateway REST               │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Gateway (backend/gateway/) — HTTP Reverse Proxy              │   │
│  │  Routes to Domain Services via bufconn transport              │   │
│  │  Feature flag: BB_USE_GATEWAY=true (dual-mode with legacy)    │   │
│  └──────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│  L3 — SECURITY LAYER (Interceptor Chain)                            │
│  Validate → Auth → RateLimit → ACL (IAM) → Audit → Standby        │
├─────────────────────────────────────────────────────────────────────┤
│  L4 — DOMAIN SERVICE LAYER                                          │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────────┐   │
│  │  DCM (8)   │ │  SQL (8)   │ │ Admin (15) │ │ Runner Service │   │
│  │  Plan,Issue│ │ SQL,DB,Inst│ │ Auth,User  │ │ Task,Check,    │   │
│  │  Rollout.. │ │ Sheet..    │ │ Setting..  │ │ Sync,Approval  │   │
│  └────────────┘ └────────────┘ └────────────┘ └────────────────┘   │
│  Transport: bufconn (in-memory HTTP) — backend/transport/           │
├─────────────────────────────────────────────────────────────────────┤
│  L5 — COMPONENT LAYER (Shared Business Logic)                       │
│  IAM Manager, Webhook, DBFactory, Masker, Sheet, Export, Bus        │
├─────────────────────────────────────────────────────────────────────┤
│  L6 — RUNNER LAYER (Async Background Processing)                    │
│  TaskRun, PlanCheck, SchemaSync, Approval, Heartbeat, Cleaner       │
├─────────────────────────────────────────────────────────────────────┤
│  L7 — PLUGIN LAYER (Extensible Adapters)                            │
│  22 DB Drivers, SQL Advisor (9 engines), SQL Parser, IDP, Webhook   │
├─────────────────────────────────────────────────────────────────────┤
│  L8 — DATA ACCESS LAYER (Store)                                     │
│  PostgreSQL (pgx/v5) + LRU Cache + Composite PK + JSONB             │
├─────────────────────────────────────────────────────────────────────┤
│  L9 — ENTERPRISE LAYER                                              │
│  License Service + Feature Gates + Plan Matrix (FREE/TEAM/ENT)      │
├─────────────────────────────────────────────────────────────────────┤
│  L10 — INFRASTRUCTURE LAYER                                         │
│  Embedded PG, Migrator, Config, Telemetry, Logging, Prometheus      │
│  NATSBus (embedded NATS), OTel, Circuit Breaker, Middleware         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. L1 — Presentation Layer

**Path**: `frontend/src/`

### Chức năng
Giao diện web SPA cho tất cả user interactions: quản lý schema, SQL editor, admin dashboard, approval workflows.

### Components

| Sub-layer             | Technology              | Path                            |
|----------------------|-------------------------|---------------------------------|
| Vue 3 App (legacy)   | Vue 3.5, Pinia 3, Naive UI | `src/views/`, `src/components/` |
| React 19 App (new)   | React 19, Zustand, Base UI | `src/react/`                   |
| SQL Editor           | Monaco Editor (VSCode API) | Monaco + LSP client            |
| Schema Visualizer    | ELK.js, D3-shape        | Schema diagram rendering        |
| State Bridge         | `useVueState()` hook    | React subscribes to Vue/Pinia   |
| Router               | Vue Router 4            | `src/router/`                   |
| i18n                 | vue-i18n + react-i18next | `src/locales/`                 |
| API Client           | ConnectRPC (generated)  | `src/connect/`                  |
| Build                | Vite 7.3 + Tailwind v4  | `vite.config.ts`               |

### Protocol Adapters (Server-side L1)

| Adapter              | Path                     | Protocol                       |
|----------------------|--------------------------|--------------------------------|
| LSP Server           | `backend/api/lsp/`       | JSON-RPC 2.0 over WebSocket   |
| MCP Server           | `backend/api/mcp/`       | Model Context Protocol (SSE)  |
| OAuth2 Provider      | `backend/api/oauth2/`    | OAuth2 Authorization Code     |
| SCIM/DirectorySync   | `backend/api/directory-sync/` | SCIM 2.0 over REST       |
| Stripe Webhook       | `backend/api/stripe/`    | Stripe webhook events          |

### Dependency Direction
```
L1 → L2 (via HTTP/ConnectRPC)
L1 Protocol Adapters → L4 (Service Layer) + L8 (Store)
```

---

## 3. L2 — API Gateway Layer

**Path**: `backend/server/`

### Chức năng
HTTP server entry point — routing requests tới ConnectRPC handlers hoặc REST gateway proxy.

### Files

| File                           | Size    | Chức năng                              |
|-------------------------------|---------|----------------------------------------|
| `server.go`                   | 11.4KB  | Server lifecycle (New/Run/Shutdown)     |
| `grpc_routes.go`              | 16.6KB  | 30+ ConnectRPC service registration     |
| `echo_routes.go`              | 5.8KB   | HTTP middleware + protocol endpoints    |
| `server_frontend_embed.go`    | 1.8KB   | Embedded SPA serving (production)       |
| `server_frontend_routes.go`   | 2.7KB   | SPA fallback routing                    |

### Route Map

| Pattern                  | Handler                    | Transport      |
|-------------------------|----------------------------|----------------|
| `/bytebase.v1.*/`       | ConnectRPC handlers        | HTTP/2, HTTP/1 |
| `/v1/*`                 | gRPC-Gateway REST proxy    | REST JSON      |
| `/v1:adminExecute`      | WebSocket proxy            | WS streaming   |
| `/lsp`                  | LSP server                 | WebSocket      |
| `/hook/scim/*`          | SCIM directory sync        | REST           |
| `/hook/stripe/*`        | Stripe webhooks            | REST           |
| `/oauth2/*`             | OAuth2 flows               | REST           |
| `/mcp/*`                | MCP server                 | SSE            |
| `/healthz`              | Health check               | REST           |
| `/metrics`              | Prometheus metrics         | REST           |
| `/*` (fallback)         | Embedded SPA               | Static files   |

### Middleware Stack
1. `recoverMiddleware` — Panic recovery
2. `securityHeadersMiddleware` — CSP, HSTS, X-Frame-Options
3. CORS (dev mode only)
4. Request Logger (error-only)
5. Prometheus metrics

### Dependency Direction
```
L2 → L3 (Interceptors) → L4 (Services)
L2 → L1 Protocol Adapters (routing)
```

---

## 4. L3 — Security Layer

**Path**: `backend/api/auth/`, `backend/api/v1/acl.go`, `backend/api/v1/audit.go`

### Chức năng
Interceptor chain xử lý authentication, authorization, và audit logging cho mọi API request.

### Interceptor Chain (thứ tự thực thi)

| #  | Interceptor          | Size    | Chức năng                                    |
|----|---------------------|---------|----------------------------------------------|
| 1  | Validate            | lib     | Proto field validation (protovalidate)        |
| 2  | Auth                | —       | JWT/Cookie/API-key extraction & verification |
| 3  | ACL                 | 19.3KB  | IAM permission check (workspace + project)   |
| 4  | Audit               | 25.2KB  | Request/response logging to audit_log table  |

### Auth Methods Supported

| Method              | Token Location          | Verification                  |
|--------------------|-------------------------|-------------------------------|
| JWT (web)          | Cookie `access-token`   | HMAC-SHA256, secret from DB   |
| API Key            | `Authorization: Bearer` | Service account lookup        |
| OAuth2             | Authorization code flow | OIDC/SAML/LDAP via IDP plugin|
| Workload Identity  | OIDC JWT                | JWKS validation               |

### Dependency Direction
```
L3 → L5 (IAM Manager) → L8 (Store)
L3 → L9 (Enterprise — license check for features)
```

---

## 5. L4 — Service Layer

**Path**: `backend/api/v1/`  — **79 files, ~1MB+ total code**

### Chức năng
Triển khai business logic cho 30+ gRPC services. Mỗi service tương ứng với một proto definition.

### Service Catalog

| Service                    | File(s)                          | Size   | Domain                        |
|---------------------------|----------------------------------|--------|-------------------------------|
| **AuthService**           | `auth_service.go`                | 78KB   | Login, signup, MFA, SSO       |
| **SQLService**            | `sql_service.go` + ai + converter| 77KB   | Query, check, parse, export   |
| **DatabaseService**       | `database_service.go` + changelog| 57KB   | DB metadata, sync, changelog  |
| **IssueService**          | `issue_service.go` + converter   | 60KB   | Issue lifecycle management    |
| **RolloutService**        | `rollout_service.go` + converter + task | 82KB | Rollout execution       |
| **PlanService**           | `plan_service.go`                | 46KB   | Migration plan management     |
| **ProjectService**        | `project_service.go` + converter | 56KB   | Project CRUD, members, VCS    |
| **InstanceService**       | `instance_service.go` + converter| 64KB   | Instance CRUD, sync, test     |
| **SettingService**        | `setting_service.go` + converter | 68KB   | Workspace/org settings        |
| **DocumentMasking**       | `document_masking.go`            | 44KB   | NoSQL document masking        |
| **UserService**           | `user_service.go`                | 37KB   | User management               |
| **OrgPolicyService**      | `org_policy_service.go` + conv   | 35KB   | Policy CRUD (masking, access) |
| **ReleaseService**        | `release_service.go` + check + ai| 56KB  | Release management + AI lint  |
| **DatabaseConverter**     | `database_converter.go`          | 34KB   | Proto ↔ Store conversion      |
| **MaskingEvaluator**      | `masking_evaluator.go`           | 12KB   | Column-level masking engine   |
| **QueryResultMasker**     | `query_result_masker.go`         | 18KB   | Apply masking to query results|
| **ReviewConfigService**   | `review_config_service.go`       | 21KB   | SQL review rule configuration |
| **WorksheetService**      | `worksheet_service.go`           | 21KB   | Shared SQL scripts            |
| **IdpService**            | `idp_service.go`                 | 25KB   | Identity provider management  |
| **AuditLogService**       | `audit_log_service.go`           | 7KB    | Audit log querying            |
| **AccessGrantService**    | `access_grant_service.go`        | 17KB   | JIT access grants             |
| **WorkspaceService**      | `workspace_service.go`           | 14KB   | Workspace setup & settings    |
| **WorkloadIdentityService** | `workload_identity_service.go` | 14KB   | OIDC federation for CI/CD     |
| **SubscriptionService**   | `subscription_service.go`        | 17KB   | License & billing             |
| Remaining 8 services      | Various                          | ~60KB  | Group, Role, Sheet, CEL, etc. |

### Dependency Direction
```
L4 → L5 (Components: IAM, Webhook, DBFactory, Masker)
L4 → L7 (Plugins: DB drivers, advisors)
L4 → L8 (Store: data persistence)
L4 → L9 (Enterprise: feature gate checks)
```

---

## 6. L5 — Component Layer

**Path**: `backend/component/`

### Chức năng
Shared business logic components — stateful managers injected vào services và runners.

### Components

| Component         | Path                 | Chức năng                                          |
|------------------|----------------------|----------------------------------------------------|
| **IAM Manager**  | `component/iam/`     | RBAC permission check, role→permission resolution, group membership |
| **Webhook**      | `component/webhook/` | IM notification dispatch (Slack, DingTalk, Feishu, Teams) |
| **DBFactory**    | `component/dbfactory/` | Database connection factory — creates Driver from Instance config |
| **Bus**          | `component/bus/`     | In-process message bus (Go channels) for runner coordination |
| **Masker**       | `component/masker/`  | Data masking algorithms (full, partial, hash, range) |
| **Sheet**        | `component/sheet/`   | Sheet/worksheet parsing and management             |
| **Export**       | `component/export/`  | Data export to CSV, Excel (excelize), JSON         |
| **Secret**       | `component/secret/`  | External secret manager (Vault, AWS SM, GCP SM)    |
| **Config**       | `component/config/`  | Server profile, runtime configuration              |
| **Telemetry**    | `component/telemetry/` | Anonymous usage metrics collection               |
| **Ghost**        | `component/ghost/`   | gh-ost integration for MySQL online schema change  |
| **SampleInstance**| `component/sampleinstance/` | Demo/sample database instance management    |

### Bus — Internal Event System

```go
type Bus struct {
    ApprovalCheckChan       chan IssueRef    // 1000 buffer
    PlanCheckTickleChan     chan int         // 1000 buffer
    TaskRunTickleChan       chan int         // 1000 buffer
    RolloutCreationChan     chan PlanRef     // 100 buffer
    PlanCompletionCheckChan chan PlanRef     // 1000 buffer
    RunningTaskRunsCancelFunc      sync.Map // cancel running tasks
    RunningPlanCheckRunsCancelFunc sync.Map // cancel running checks
}
```

### Dependency Direction
```
L5 → L7 (Plugins via DBFactory)
L5 → L8 (Store)
L5 → L9 (Enterprise: feature checks in IAM)
```

---

## 7. L6 — Runner Layer

**Path**: `backend/runner/`

### Chức năng
Async background goroutines — processing tasks, checks, syncing, monitoring.

### Runner Catalog

| Runner                | Path                    | Trigger                        | Chức năng                                    |
|----------------------|-------------------------|--------------------------------|----------------------------------------------|
| **TaskRun Scheduler** | `runner/taskrun/`       | Bus.TaskRunTickleChan          | Orchestrate pending→running→done task lifecycle |
| **PlanCheck Scheduler** | `runner/plancheck/`   | Bus.PlanCheckTickleChan        | Execute SQL review, compatibility checks     |
| **SchemaSync Syncer** | `runner/schemasync/`   | Periodic + on-demand           | Sync schema metadata from remote instances   |
| **Approval Runner**  | `runner/approval/`      | Bus.ApprovalCheckChan          | Auto-match approval templates, route approvals |
| **NotifyListener**   | `runner/notifylistener/`| PG LISTEN/NOTIFY               | Bridge PG notifications → Bus channels       |
| **DataCleaner**      | `runner/cleaner/`       | Periodic                       | Clean expired data (task runs, exports, etc.)|
| **Heartbeat Runner** | `runner/heartbeat/`     | Periodic                       | Report server health, replica status         |
| **MemoryMonitor**    | `runner/monitor/`       | Periodic                       | Monitor memory usage, trigger alerts         |

### TaskRun Internal Architecture

```
taskrun/
  ├── scheduler.go           — Main scheduler loop, Register executors
  ├── pending_scheduler.go   — Pick PENDING tasks, check dependencies
  ├── running_scheduler.go   — Monitor RUNNING tasks, handle timeouts
  ├── rollout_creator.go     — Auto-create rollouts from plans
  ├── executor.go            — Executor interface definition
  ├── database_create_executor.go   — CREATE DATABASE task
  ├── database_migrate_executor.go  — DDL/DML migration (37KB)
  └── data_export_executor.go       — Data export task
```

### Dependency Direction
```
L6 → L5 (Bus for coordination, Webhook for notifications)
L6 → L7 (DB Drivers for execution, Advisors for checks)
L6 → L8 (Store for task state management)
L6 → L9 (Enterprise: license checks for features)
```

---

## 8. L7 — Plugin Layer

**Path**: `backend/plugin/`

### Chức năng
Extensible adapters — mỗi plugin implement một interface chuẩn, register via `init()`.

### Plugin Catalog

| Plugin Type       | Path              | Interface           | Implementations                                  |
|------------------|-------------------|---------------------|---------------------------------------------------|
| **DB Drivers**   | `plugin/db/`      | `db.Driver`         | 22: pg, mysql, tidb, clickhouse, mongodb, redis, snowflake, oracle, mssql, spanner, bigquery, cockroachdb, cosmosdb, dynamodb, elasticsearch, hive, databricks, trino, starrocks, sqlite, cassandra, redshift |
| **SQL Advisor**  | `plugin/advisor/` | `Advisor`           | 9 engines: pg, mysql, tidb, oracle, mssql, snowflake, oceanbase, redshift |
| **SQL Parser**   | `plugin/parser/`  | Per-engine parser   | ANTLR4-based parsers for each SQL dialect         |
| **Schema**       | `plugin/schema/`  | Schema operations   | Schema diff, conversion utilities                  |
| **IDP**          | `plugin/idp/`     | IDP interface       | OIDC, SAML, LDAP                                  |
| **Mailer**       | `plugin/mailer/`  | Mailer interface    | SMTP email sending                                 |
| **Webhook**      | `plugin/webhook/` | Webhook interface   | IM platform integrations                           |
| **Stripe**       | `plugin/stripe/`  | Stripe API          | Billing integration                                |

### DB Driver Registration Pattern

```go
// In plugin/db/pg/driver.go
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}

// Factory usage
driver, err := db.Open(ctx, storepb.Engine_POSTGRES, connectionConfig)
```

### Dependency Direction
```
L7 → L8 (Store, for some plugins)
L7 ← L4, L6 (consumed by Services and Runners)
L7 has NO dependency on L3, L5 (clean plugin boundary)
```

---

## 9. L8 — Data Access Layer

**Path**: `backend/store/` — **74 files + model/**

### Chức năng
Tất cả data persistence operations qua PostgreSQL. Cung cấp typed methods cho mỗi entity.

### Store Files by Domain

| Domain              | Files                                          | Entities                |
|--------------------|------------------------------------------------|-------------------------|
| **Identity**       | `principal.go`, `service_account.go`, `workload_identity.go`, `group.go` | User, ServiceAccount, WorkloadIdentity, Group |
| **Project**        | `project.go`, `project_webhook.go`             | Project, ProjectWebhook |
| **Database**       | `database.go`, `database_group.go`, `db_schema.go` | Database, DatabaseGroup, Schema |
| **Instance**       | `instance.go`                                  | Instance, DataSource    |
| **Change Mgmt**    | `issue.go`, `issue_comment.go`, `plan.go`, `task.go`, `task_run.go`, `task_run_log.go`, `plan_check_run.go`, `release.go`, `revision.go`, `changelog.go` | Issue, Plan, Task, TaskRun, PlanCheckRun, Release, Revision, Changelog |
| **Security**       | `policy.go`, `access_grant.go`, `role.go`, `predefined_roles.go` | Policy, AccessGrant, Role |
| **Auth**           | `web_refresh_token.go`, `email_verification_code.go`, `oauth2_*.go` | RefreshToken, OAuth2Client |
| **Settings**       | `setting.go`, `environment.go`, `workspace.go`, `server_config.go` | Setting, Environment, Workspace |
| **SQL Editor**     | `sheet.go`, `worksheet.go`, `query_history.go` | Sheet, Worksheet, QueryHistory |
| **Audit**          | `audit_log.go`                                 | AuditLog                |
| **Infrastructure** | `store.go`, `db_connection.go`, `db_metrics.go`, `db_tracer.go`, `advisory_lock.go`, `signal.go`, `stats.go`, `filter.go`, `id.go` | Store, Connection, Lock |

### Caching Strategy

| Cache Type     | Library              | Capacity | TTL     | Invalidation     |
|---------------|----------------------|----------|---------|------------------|
| LRU (no-TTL)  | `hashicorp/golang-lru` | 1K-32K  | None    | Write-through    |
| Expirable LRU | `hashicorp/golang-lru/expirable` | 128-4K | 1-5min | Auto-expire + manual purge |

### Dependency Direction
```
L8 → PostgreSQL (external)
L8 → generated-go/store (protobuf types)
L8 ← ALL upper layers (L3-L7 depend on Store)
```

---

## 10. L9 — Enterprise Layer

**Path**: `backend/enterprise/`

### Chức năng
License validation và feature gating — controls which features are available per plan.

### Files

| File           | Size   | Chức năng                              |
|---------------|--------|----------------------------------------|
| `license.go`  | 19KB   | License parsing, validation, expiry    |
| `plan.yaml`   | 6.1KB  | Feature matrix (FREE/TEAM/ENTERPRISE)  |
| `keys/`       | —      | Public keys for license verification   |
| `pricing/`    | —      | Pricing configuration                  |

### Feature Gate Pattern

```go
// Check if feature is enabled for workspace
err := licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_DATA_MASKING)
if err != nil {
    return nil, status.Errorf(codes.PermissionDenied, "feature requires Enterprise plan")
}
```

### Plan Matrix Summary

| Plan       | Instances | Seats     | Key Exclusive Features                    |
|------------|-----------|-----------|-------------------------------------------|
| FREE       | 10        | 20        | Basic DCM, SQL Editor, Community support  |
| TEAM       | 10        | Unlimited | SSO (Google/GitHub), Audit Log, Batch Query |
| ENTERPRISE | Unlimited | Unlimited | Data Masking, Custom Roles, 2FA, SCIM, Approval Workflow |

---

## 11. L10 — Infrastructure Layer

### Components

| Component           | Path                       | Chức năng                              |
|--------------------|----------------------------|----------------------------------------|
| **Embedded PG**    | `backend/resources/postgres/` | Start/stop embedded PostgreSQL for non-HA |
| **Migrator**       | `backend/migrator/`        | Self-migration for Bytebase metadata schema |
| **Config**         | `backend/component/config/`| Profile, flags, runtime settings       |
| **Common Utils**   | `backend/common/`          | CEL, resource names, error types, engine mapping |
| **Generated Code** | `backend/generated-go/`    | Proto-generated Go code (store + v1)   |
| **Proto Defs**     | `proto/`                   | Source protobuf definitions            |
| **Demo Data**      | `backend/demo/`            | Demo data loader                       |
| **Test Infra**     | `backend/tests/`           | Integration tests with testcontainers  |

---

## 12. Cross-Layer Data Flow Diagrams

### 12.1 Schema Migration Flow

```
Developer (L1)
  → CreatePlan API (L2→L3→L4: PlanService)
    → Store plan (L8)
    → Trigger PlanCheck (L5: Bus → L6: PlanCheckScheduler)
      → SQL Review (L7: Advisor plugin)
      → Results → Store (L8)
    → Trigger Approval (L5: Bus → L6: ApprovalRunner)
      → Match template → Update issue (L8)
    → [Approved] Trigger Rollout (L5: Bus → L6: TaskRunScheduler)
      → Execute migration (L7: DB Driver plugin)
      → Schema sync (L6: SchemaSync → L7: DB Driver)
      → Webhook notification (L5: WebhookManager → L7: Webhook plugin)
      → Audit log (L8)
```

### 12.2 SQL Query Flow

```
User (L1: SQL Editor)
  → Query API (L2→L3→L4: SQLService)
    → ACL check (L3 → L5: IAM Manager)
    → Feature check (L9: Enterprise)
    → Open DB connection (L5: DBFactory → L7: DB Driver)
    → Execute query (L7: Driver.QueryConn)
    → Evaluate masking (L4: MaskingEvaluator → L8: policies)
    → Apply masking (L5: Masker)
    → Return masked results (L4 → L2 → L1)
    → Log to query_history (L8)
    → Audit log (L3: AuditInterceptor → L8)
```

### 12.3 Authentication Flow

```
User (L1: Login page)
  → Login API (L2 → L4: AuthService)
    → Verify credentials (L8: principal table)
    → [SSO] Redirect to IDP (L7: IDP plugin)
    → [2FA] Verify TOTP (L4: pquerna/otp)
    → Generate JWT (L4: golang-jwt/v5)
    → Store refresh token (L8: web_refresh_token)
    → Set Cookie (L2: response headers)
    → Audit log (L3 → L8)
```

---

## 13. Dependency Matrix

```
         L1  L2  L3  L4  L5  L6  L7  L8  L9  L10
L1  (UI)  —   ●               ●               
L2  (GW)      —   ●                           ●
L3  (Sec)         —       ●           ●   ●   
L4  (Svc)             —   ●       ●   ●   ●   
L5  (Cmp)                 —       ●   ●   ●   
L6  (Run)                 ●   —   ●   ●   ●   
L7  (Plg)                             —   ●       
L8  (DAL)                                 —       ●
L9  (Ent)                                 ●   —   
L10 (Inf)                                         —

● = depends on
```

**Key rules**:
- L7 (Plugin) has NO upward dependency → clean plugin boundary
- L8 (Store) is the most depended-upon layer
- L1 (Frontend) communicates only through L2 (API Gateway)
- L9 (Enterprise) is checked by L3, L4, L5, L6

---

## 12. Gateway + Domain Services Architecture (v2)

### 12.1 Overview

The Bytebase backend has been refactored from a monolithic service layer into a **Gateway + Domain Services** architecture. This is controlled by the `BB_USE_GATEWAY` environment variable and runs within a **single binary**.

### 12.2 Domain Services

| Service | Package | Handlers | Responsibility |
|---------|---------|----------|----------------|
| DCM | `backend/service/dcm/` | 8 | Plan, Issue, Rollout, Release, Revision, ReviewConfig, AccessGrant, OrgPolicy |
| SQL | `backend/service/sqlsvc/` | 8 | SQL, Database, DatabaseCatalog, DatabaseGroup, Instance, InstanceRole, Sheet, Worksheet |
| Admin | `backend/service/admin/` | 15 | Auth, User, ServiceAccount, WorkloadIdentity, Role, Group, IdP, Setting, Workspace, Project, Subscription, Actuator, AuditLog, Cel, AI |
| Runner | `backend/service/runner/` | — | All background task orchestration (TaskScheduler, PlanCheck, SchemaSync, Approval, etc.) |

### 12.3 Transport Layer

Services communicate via **bufconn** — an in-memory `net.Listener` that allows HTTP communication without TCP sockets. The `BufconnTransport` (`backend/transport/`) creates a listener and matching `http.Client` for each service.

### 12.4 Gateway

The Gateway (`backend/gateway/`) is an HTTP reverse proxy that:
1. Routes ConnectRPC requests by service path prefix to the correct domain service proxy
2. Preserves the REST gateway (`/v1/*`) via gRPC-Gateway
3. Handles gRPC reflection for all 31 services
4. Extracts the interceptor chain (`Validate → Auth → RateLimit → ACL → Audit → Standby`)

### 12.5 New Directory Structure

```
backend/
├── gateway/              # HTTP reverse proxy + interceptors + routes
│   ├── gateway.go        # ConnectRPC path routing via bufconn proxies
│   ├── interceptors.go   # Extracted interceptor chain
│   └── routes.go         # REST gateway + echo route registration
├── service/              # Domain service interfaces + implementations
│   ├── service.go        # DomainService, ServiceRouter, RunnerService interfaces
│   ├── registry.go       # Service lifecycle management
│   ├── dcm/              # Change management domain
│   ├── sqlsvc/           # SQL/Database operations domain
│   ├── admin/            # Administrative services domain
│   └── runner/           # Background runner orchestration
├── transport/            # Transport abstractions
│   └── bufconn.go        # In-memory HTTP transport
├── component/            # Shared infrastructure
│   ├── bus/              # EventBus + NATSBus
│   ├── otel/             # OpenTelemetry tracing
│   ├── metrics/          # Prometheus service metrics
│   ├── middleware/        # Panic recovery, HMAC auth, logging
│   ├── errors/           # Standardized error types
│   ├── circuitbreaker/   # Per-service circuit breakers
│   └── config/           # Service configuration
├── server/               # Main server (dual-mode: legacy/gateway)
│   ├── server.go         # Server struct + lifecycle (Run/Shutdown)
│   ├── gateway_init.go   # Gateway-mode initialization
│   └── grpc_routes.go    # Legacy monolithic router (fallback)
└── ...
```

### 12.6 Feature Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `BB_USE_GATEWAY` | `false` | Enable gateway routing mode |
| `BB_USE_NATS_BUS` | `false` | Use NATS event bus instead of Go channels |
| `BB_ENABLE_TRACING` | `false` | Enable OpenTelemetry distributed tracing |
| `BB_ENABLE_CIRCUIT_BREAKER` | `false` | Enable per-service circuit breakers |
| `BB_INTERNAL_AUTH_ENABLED` | `false` | Enable HMAC auth for internal service calls |

---

> **Document generated**: 2026-05-08 — Updated 2026-05-15 with Gateway + Services architecture.

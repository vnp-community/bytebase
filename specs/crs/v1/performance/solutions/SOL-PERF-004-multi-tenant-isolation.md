# Solution: CR-PERF-004 — Multi-Tenant Workspace Isolation

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-004                              |
| **Solution ID**| SOL-PERF-004                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L3 (Security), L5 (IAM), L8 (Store), L9 (Enterprise) |
| **TDD Refs**   | §3 Request Processing, §7 IAM & Authorization |

---

## 1. Solution Overview

Implement tenant isolation cho 100+ banks trên shared deployment. Từ TDD §3.1, request pipeline đi qua 4 interceptors: Validate → Auth → ACL → Audit. Giải pháp thêm **Rate Limit** và **Quota Check** vào interceptor chain, ngay sau ACL.

Từ Architecture L3, workspace isolation hiện chỉ qua SQL WHERE filters. Giải pháp thêm:
- **Quota enforcement** tại Store layer
- **Rate limiting** tại API Gateway layer
- **Per-tenant metrics** tại Observability layer

---

## 2. Detailed Technical Design

### 2.1 Tenant Quota Manager

**File**: `backend/component/quota/manager.go` (new)

```go
package quota

import (
    "context"
    "sync"
    "time"

    "github.com/bytebase/bytebase/backend/store"
)

// ResourceType defines quotable resource types.
type ResourceType string

const (
    ResourceInstance  ResourceType = "instance"
    ResourceDatabase  ResourceType = "database"
    ResourceProject   ResourceType = "project"
    ResourceUser      ResourceType = "user"
)

// QuotaConfig defines limits for a workspace.
type QuotaConfig struct {
    MaxInstances  int `json:"maxInstances"`
    MaxDatabases  int `json:"maxDatabases"`
    MaxProjects   int `json:"maxProjects"`
    MaxUsers      int `json:"maxUsers"`
}

// DefaultQuota is the default quota for new workspaces.
var DefaultQuota = QuotaConfig{
    MaxInstances: 100,
    MaxDatabases: 5000,
    MaxProjects:  50,
    MaxUsers:     200,
}

// Manager manages tenant resource quotas with in-memory cache.
type Manager struct {
    store       *store.Store
    mu          sync.RWMutex
    quotaCache  map[string]*QuotaConfig      // workspace → quota
    usageCache  map[string]map[ResourceType]int // workspace → resource → count
    lastRefresh time.Time
}

func NewManager(s *store.Store) *Manager {
    return &Manager{
        store:      s,
        quotaCache: make(map[string]*QuotaConfig),
        usageCache: make(map[string]map[ResourceType]int),
    }
}

// CheckQuota verifies if the workspace can create more of the given resource.
// Returns nil if allowed, error with RESOURCE_EXHAUSTED if quota exceeded.
func (m *Manager) CheckQuota(ctx context.Context, workspace string, resource ResourceType) error {
    quota := m.getQuota(workspace)
    usage := m.getUsage(ctx, workspace, resource)

    var limit int
    switch resource {
    case ResourceInstance:
        limit = quota.MaxInstances
    case ResourceDatabase:
        limit = quota.MaxDatabases
    case ResourceProject:
        limit = quota.MaxProjects
    case ResourceUser:
        limit = quota.MaxUsers
    }

    if usage >= limit {
        return status.Errorf(codes.ResourceExhausted,
            "workspace %q exceeded %s quota: %d/%d",
            workspace, resource, usage, limit)
    }
    return nil
}

// getUsage returns current resource count, cached for 30s.
func (m *Manager) getUsage(ctx context.Context, workspace string, resource ResourceType) int {
    m.mu.RLock()
    if ws, ok := m.usageCache[workspace]; ok {
        if count, ok := ws[resource]; ok {
            m.mu.RUnlock()
            return count
        }
    }
    m.mu.RUnlock()

    // Cache miss: query actual count
    count := m.queryResourceCount(ctx, workspace, resource)

    m.mu.Lock()
    if m.usageCache[workspace] == nil {
        m.usageCache[workspace] = make(map[ResourceType]int)
    }
    m.usageCache[workspace][resource] = count
    m.mu.Unlock()

    return count
}

func (m *Manager) queryResourceCount(ctx context.Context, workspace string, resource ResourceType) int {
    var table string
    switch resource {
    case ResourceInstance:
        table = "instance"
    case ResourceDatabase:
        table = "db"
    case ResourceProject:
        table = "project"
    case ResourceUser:
        table = "principal"
    }

    var count int
    query := fmt.Sprintf(
        "SELECT COUNT(1) FROM %s WHERE workspace = $1 AND deleted = false", table)
    m.store.GetDB().QueryRowContext(ctx, query, workspace).Scan(&count)
    return count
}

// InvalidateUsage clears cached usage for a workspace/resource after mutations.
func (m *Manager) InvalidateUsage(workspace string, resource ResourceType) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if ws, ok := m.usageCache[workspace]; ok {
        delete(ws, resource)
    }
}
```

### 2.2 Rate Limiter

Sử dụng token bucket per-workspace, in-memory. Từ TDD §3.1, rate limit check nên ở vị trí sau Auth (đã biết workspace) và trước ACL (tránh CPU cho rejected requests).

**File**: `backend/component/ratelimit/limiter.go` (new)

```go
package ratelimit

import (
    "sync"
    "time"

    "golang.org/x/time/rate"
)

// WorkspaceLimiter provides per-workspace rate limiting.
type WorkspaceLimiter struct {
    mu       sync.RWMutex
    limiters map[string]*rate.Limiter
    config   Config
}

type Config struct {
    ReadRatePerSecond  float64 // Default: 1000
    WriteRatePerSecond float64 // Default: 100
    ReadBurst          int     // Default: 2000
    WriteBurst         int     // Default: 200
}

var DefaultConfig = Config{
    ReadRatePerSecond:  1000,
    WriteRatePerSecond: 100,
    ReadBurst:          2000,
    WriteBurst:         200,
}

func New(config Config) *WorkspaceLimiter {
    return &WorkspaceLimiter{
        limiters: make(map[string]*rate.Limiter),
        config:   config,
    }
}

// Allow checks if the request should be allowed for the given workspace.
func (wl *WorkspaceLimiter) Allow(workspace string, isWrite bool) bool {
    limiter := wl.getOrCreateLimiter(workspace, isWrite)
    return limiter.Allow()
}

func (wl *WorkspaceLimiter) getOrCreateLimiter(workspace string, isWrite bool) *rate.Limiter {
    key := workspace + ":read"
    rateLimit := rate.Limit(wl.config.ReadRatePerSecond)
    burst := wl.config.ReadBurst
    if isWrite {
        key = workspace + ":write"
        rateLimit = rate.Limit(wl.config.WriteRatePerSecond)
        burst = wl.config.WriteBurst
    }

    wl.mu.RLock()
    if l, ok := wl.limiters[key]; ok {
        wl.mu.RUnlock()
        return l
    }
    wl.mu.RUnlock()

    wl.mu.Lock()
    defer wl.mu.Unlock()
    l := rate.NewLimiter(rateLimit, burst)
    wl.limiters[key] = l
    return l
}
```

### 2.3 Interceptor Integration

Từ TDD §3.1, interceptor chain: Validate → Auth → ACL → Audit. Thêm RateLimit + Quota vào chain.

**File**: `backend/server/grpc_routes.go` — modify interceptor registration

```go
// Updated interceptor chain:
// Validate → Auth → RateLimit → ACL → Quota → Audit
interceptors := connect.WithInterceptors(
    validate.NewInterceptor(),
    s.authInterceptor,
    s.rateLimitInterceptor,    // NEW: per-tenant rate limit
    s.aclInterceptor,
    s.auditInterceptor,
)
```

**File**: `backend/api/v1/ratelimit_interceptor.go` (new)

```go
package v1

import (
    "context"

    "connectrpc.com/connect"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    "github.com/bytebase/bytebase/backend/api/auth"
    "github.com/bytebase/bytebase/backend/component/ratelimit"
)

type RateLimitInterceptor struct {
    limiter *ratelimit.WorkspaceLimiter
}

func NewRateLimitInterceptor(limiter *ratelimit.WorkspaceLimiter) *RateLimitInterceptor {
    return &RateLimitInterceptor{limiter: limiter}
}

func (i *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        workspace := auth.GetWorkspaceFromContext(ctx)
        if workspace == "" {
            return next(ctx, req) // No workspace → skip rate limit
        }

        isWrite := isWriteMethod(req.Spec().Procedure)
        if !i.limiter.Allow(workspace, isWrite) {
            return nil, status.Errorf(codes.ResourceExhausted,
                "rate limit exceeded for workspace %q", workspace)
        }

        return next(ctx, req)
    }
}

func isWriteMethod(procedure string) bool {
    // Methods starting with Create/Update/Delete/Batch are writes
    // List/Get/Search are reads
    // ...
    return false
}
```

### 2.4 Quota Check in Store Operations

**File**: `backend/store/database.go` — add quota check before create

```go
func (s *Store) CreateDatabaseDefault(ctx context.Context, create *DatabaseMessage) (*DatabaseMessage, error) {
    // Quota check (injected via QuotaChecker interface)
    if s.quotaChecker != nil {
        if err := s.quotaChecker.CheckQuota(ctx, create.Workspace, quota.ResourceDatabase); err != nil {
            return nil, err
        }
    }

    // ... existing create logic ...

    // After successful create, invalidate usage cache
    if s.quotaChecker != nil {
        s.quotaChecker.InvalidateUsage(create.Workspace, quota.ResourceDatabase)
    }
    return result, nil
}
```

### 2.5 Per-Tenant Prometheus Metrics

**File**: `backend/metrics/tenant_metrics.go` (new)

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    TenantAPIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_tenant_api_requests_total",
        Help: "Total API requests per workspace",
    }, []string{"workspace", "method", "status"})

    TenantDatabaseCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_tenant_databases_total",
        Help: "Total databases per workspace",
    }, []string{"workspace"})

    TenantQuotaUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_tenant_quota_usage_ratio",
        Help: "Quota usage ratio (0.0 - 1.0) per workspace per resource",
    }, []string{"workspace", "resource"})

    TenantRateLimitRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_tenant_rate_limit_rejected_total",
        Help: "Total rate-limited requests per workspace",
    }, []string{"workspace"})
)

func init() {
    prometheus.MustRegister(
        TenantAPIRequests, TenantDatabaseCount,
        TenantQuotaUsage, TenantRateLimitRejected,
    )
}
```

### 2.6 Quota Storage (Settings-based)

Tận dụng existing `setting` table (Architecture L8) thay vì tạo table mới:

```go
// Store quota as workspace setting
settingName := storepb.SettingName_SETTING_WORKSPACE_QUOTA
quota := &storepb.WorkspaceQuota{
    MaxInstances: 100,
    MaxDatabases: 5000,
    MaxProjects:  50,
    MaxUsers:     200,
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L2 (Gateway) | **LOW** | Add rate limit response headers |
| L3 (Security) | **HIGH** | New interceptor in chain |
| L5 (Component) | **HIGH** | New quota + ratelimit components |
| L8 (Store) | **MEDIUM** | Quota check hooks in create operations |
| L9 (Enterprise) | **LOW** | Quota feature gated to Enterprise plan |

---

## 4. Interceptor Chain — Updated Flow

```
Client Request
  │
  ▼
  1. Validate (protovalidate)
  2. Auth (JWT/Cookie/API-key → workspace context)
  3. RateLimit (per-workspace token bucket)     ← NEW
  4. ACL (IAM permission check)
  5. Audit (request/response logging)
  │
  ▼
  Service Handler → QuotaCheck → Store → Response
```

---

## 5. Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `TENANT_RATE_LIMIT_ENABLED` | `true` | Enable per-tenant rate limiting |
| `TENANT_READ_RATE` | `1000` | Read requests/sec per tenant |
| `TENANT_WRITE_RATE` | `100` | Write requests/sec per tenant |
| `TENANT_QUOTA_ENABLED` | `true` | Enable quota enforcement |
| `TENANT_DEFAULT_MAX_DBS` | `5000` | Default max databases per tenant |
| `TENANT_DEFAULT_MAX_INSTANCES` | `100` | Default max instances per tenant |

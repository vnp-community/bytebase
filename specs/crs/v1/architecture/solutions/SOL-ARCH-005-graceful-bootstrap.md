# Solution: Graceful Bootstrap — CR-ARCH-005

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-005                                             |
| **CR Reference**   | CR-ARCH-005                                              |
| **Title**          | Component Registry + Classification + Parallel Init     |
| **Affected Layers**| L2 (Server), L5 (Component), L10 (Protocol)              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §2 (L2 — Server Bootstrap):
- `NewServer()` runs 12 sequential init steps in `server.go:97-258`
- Any failure → `return nil, err` → server refuses to start

Per [TDD.md](../../TDD.md) §2 (Server Lifecycle):
- Single binary, all-or-nothing startup
- 5 protocol servers: LSP, SCIM, OAuth2, MCP, Stripe

---

## 2. Current Implementation Analysis

### 2.1 Sequential Init Chain (server.go:97-258)

```go
func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
    // CRITICAL (sequential, must succeed)
    stores, err := store.New(...)         // line 140
    migrator.MigrateSchema(...)           // line 151
    enterprise.NewLicenseService(...)     // line 158
    iam.NewManager(...)                   // line 209
    
    // OPTIONAL (sequential, same error handling)
    lsp.NewServer(...)                    // line 241 — failure kills server
    mcp.NewServer(...)                    // line 245 — failure kills server
    stripeapi.NewWebhookHandler(...)      // line 250 — failure kills server
}
```

---

## 3. Solution Design

### 3.1 Component Classification

**New file**: `backend/server/components.go`

```go
package server

import (
    "context"
    "log/slog"
    "sync"
    "time"
)

// ComponentClass defines the criticality of a server component.
type ComponentClass int

const (
    // Critical — server MUST abort if init fails.
    Critical ComponentClass = iota
    // Important — server starts degraded, component retries in background.
    Important
    // Optional — server starts without it, feature disabled.
    Optional
)

// ComponentStatus tracks the health of an individual component.
type ComponentStatus struct {
    Name    string
    Class   ComponentClass
    Status  string    // "healthy", "degraded", "disabled", "failed"
    Error   error
    StartedAt time.Time
}

// ComponentRegistry tracks all server components and their health.
type ComponentRegistry struct {
    mu         sync.RWMutex
    components map[string]*ComponentStatus
}

func NewComponentRegistry() *ComponentRegistry {
    return &ComponentRegistry{
        components: make(map[string]*ComponentStatus),
    }
}

func (r *ComponentRegistry) Register(name string, class ComponentClass) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.components[name] = &ComponentStatus{
        Name:   name,
        Class:  class,
        Status: "initializing",
    }
}

func (r *ComponentRegistry) SetHealthy(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if c, ok := r.components[name]; ok {
        c.Status = "healthy"
        c.Error = nil
        c.StartedAt = time.Now()
    }
}

func (r *ComponentRegistry) SetFailed(name string, err error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if c, ok := r.components[name]; ok {
        c.Status = "failed"
        c.Error = err
    }
}

func (r *ComponentRegistry) SetDisabled(name string, err error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if c, ok := r.components[name]; ok {
        c.Status = "disabled"
        c.Error = err
    }
}

// IsReady returns true if all Critical components are healthy.
func (r *ComponentRegistry) IsReady() bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, c := range r.components {
        if c.Class == Critical && c.Status != "healthy" {
            return false
        }
    }
    return true
}

// HealthReport returns the status of all components.
func (r *ComponentRegistry) HealthReport() map[string]*ComponentStatus {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make(map[string]*ComponentStatus, len(r.components))
    for k, v := range r.components {
        cp := *v
        result[k] = &cp
    }
    return result
}
```

### 3.2 Modified Server Bootstrap

**Modified file**: `backend/server/server.go`

```go
type Server struct {
    // ... existing fields ...
    registry *ComponentRegistry
}

func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
    s := &Server{
        profile:   profile,
        startedTS: time.Now().Unix(),
        registry:  NewComponentRegistry(),
    }
    
    // ============================================================
    // CRITICAL COMPONENTS — failure = abort
    // ============================================================
    s.registry.Register("store", Critical)
    s.registry.Register("migrator", Critical)
    s.registry.Register("license", Critical)
    s.registry.Register("iam", Critical)
    s.registry.Register("echo", Critical)

    // Store init
    stores, err := store.New(ctx, pgURL, cacheBackend, redisURL)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to new store")
    }
    s.registry.SetHealthy("store")

    // Migration
    if err := migrator.MigrateSchema(ctx, stores.GetDB()); err != nil {
        return nil, errors.Wrapf(err, "failed to migrate schema")
    }
    s.registry.SetHealthy("migrator")

    // License + IAM (critical — required for auth)
    s.licenseService, err = enterprise.NewLicenseService(...)
    if err != nil {
        return nil, errors.Wrap(err, "failed to create license service")
    }
    s.registry.SetHealthy("license")

    s.iamManager, err = iam.NewManager(stores, s.licenseService, profile.SaaS)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to create iam manager")
    }
    s.registry.SetHealthy("iam")

    // Echo (critical — HTTP server)
    s.echoServer = echo.New()
    s.registry.SetHealthy("echo")

    // ============================================================
    // IMPORTANT COMPONENTS — init with fallback
    // ============================================================
    s.registry.Register("webhook", Important)
    s.registry.Register("dbfactory", Important)

    s.webhookManager = webhook.NewManager(stores, profile)
    s.registry.SetHealthy("webhook")
    s.dbFactory = dbfactory.New(s.store, s.licenseService)
    s.registry.SetHealthy("dbfactory")

    // ============================================================
    // OPTIONAL COMPONENTS — parallel init, failure = disabled
    // ============================================================
    s.registry.Register("lsp", Optional)
    s.registry.Register("mcp", Optional)
    s.registry.Register("stripe", Optional)
    s.registry.Register("scim", Optional)
    s.registry.Register("oauth2", Optional)
    s.registry.Register("sample_instance", Optional)

    type optResult struct {
        name string
        err  error
    }
    optCh := make(chan optResult, 6)
    var optWG sync.WaitGroup

    // Parallel init for independent optional components
    optWG.Add(1)
    go func() {
        defer optWG.Done()
        s.lspServer = lsp.NewServer(s.store, profile, secret, s.bus, s.iamManager, s.licenseService)
        optCh <- optResult{"lsp", nil}  // LSP NewServer doesn't return error
    }()

    optWG.Add(1)
    go func() {
        defer optWG.Done()
        mcpSrv, err := mcp.NewServer(stores, profile, secret)
        if err != nil {
            optCh <- optResult{"mcp", err}
            return
        }
        mcpServer = mcpSrv
        optCh <- optResult{"mcp", nil}
    }()

    optWG.Add(1)
    go func() {
        defer optWG.Done()
        // Stripe webhook — non-critical
        stripeHandler := stripeapi.NewWebhookHandler(s.store, s.licenseService, profile.StripeWebhookSecret)
        stripeWebhookHandler = stripeHandler
        optCh <- optResult{"stripe", nil}
    }()

    optWG.Add(1)
    go func() {
        defer optWG.Done()
        dirSyncSrv := directorysync.NewService(s.store, s.licenseService, s.iamManager, profile)
        directorySyncServer = dirSyncSrv
        optCh <- optResult{"scim", nil}
    }()

    optWG.Add(1)
    go func() {
        defer optWG.Done()
        oauth2Svc := oauth2.NewService(stores, profile, secret)
        oauth2Service = oauth2Svc
        optCh <- optResult{"oauth2", nil}
    }()

    // Wait for all optional inits
    go func() {
        optWG.Wait()
        close(optCh)
    }()

    for result := range optCh {
        if result.err != nil {
            slog.Warn("Optional component disabled",
                "component", result.name,
                "error", result.err,
            )
            s.registry.SetDisabled(result.name, result.err)
        } else {
            s.registry.SetHealthy(result.name)
        }
    }

    // Configure routers (uses whatever components are available)
    if err := configureGrpcRouters(ctx, s.echoServer, ...); err != nil {
        return nil, errors.Wrapf(err, "failed to configure gRPC routers")
    }
    configureEchoRouters(s.echoServer, s.lspServer, directorySyncServer,
        oauth2Service, mcpServer, stripeWebhookHandler, profile, s.registry)

    serverStarted = true
    return s, nil
}
```

### 3.3 Noop Implementations for Disabled Components

**New file**: `backend/api/mcp/noop.go`

```go
package mcp

import (
    "net/http"
    "github.com/labstack/echo/v5"
)

type NoopServer struct{}

func NewNoopServer() *NoopServer { return &NoopServer{} }

func (s *NoopServer) Handler(c *echo.Context) error {
    return c.JSON(http.StatusServiceUnavailable, map[string]string{
        "error":   "MCP server is not available",
        "reason":  "Component initialization failed",
    })
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/server/components.go` | L2 | **NEW** | Component registry |
| `backend/server/server.go` | L2 | **MODIFY** | Classification + parallel init |
| `backend/api/mcp/noop.go` | L10 | **NEW** | Noop MCP handler |
| `backend/api/lsp/noop.go` | L10 | **NEW** | Noop LSP handler |

---

## 5. Rollback Plan

1. Remove parallel init, restore sequential → single file revert
2. ComponentRegistry is additive — removing it has no behavioral impact

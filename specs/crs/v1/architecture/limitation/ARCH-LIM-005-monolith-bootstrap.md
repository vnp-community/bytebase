# ARCH-LIM-005 — Monolithic Bootstrap Coupling

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Structural Trade-off)          |
| **Layer**      | L2→L10 (Cross-cutting)                    |
| **Impact**     | Startup Time, Partial Failure Resilience   |
| **Severity**   | Medium                                     |

---

## 1. Description

`NewServer()` (server.go:97-258) thực hiện **sequential initialization** cho tất cả components. Bất kỳ component nào fail → toàn bộ server fail to start. Không có partial startup hay health degradation.

### Evidence (server.go:97-258 — sequential chain)

```go
func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
    // 1. Embedded PG (if needed) — blocks until PG ready
    // 2. store.New() → PG connection + cache init
    // 3. migrator.MigrateSchema() → DDL execution  
    // 4. enterprise.NewLicenseService()
    // 5. iam.NewManager() + ReloadCache() 
    // 6. webhook.NewManager()
    // 7. dbfactory.New()
    // 8. echo.New()
    // 9. 8 runners initialized
    // 10. 5 protocol servers (LSP, SCIM, OAuth2, MCP, Stripe)
    // 11. configureGrpcRouters() — 30+ services
    // 12. configureEchoRouters() — HTTP routes
    
    // ANY failure above → return nil, err → server doesn't start
}
```

### Coupling Chain

```
Embedded PG → Store → Migrator → License → IAM → Webhook → Runners → Services
     ↓ fail    ↓ fail    ↓ fail    ↓ fail    ↓ fail
   全部 fail   全部 fail  全部 fail  全部 fail  全部 fail
```

---

## 2. Metrics

- **12 sequential init steps** — all must succeed
- **8 background runners** started in `Run()` — all or nothing
- **30+ services** registered in `configureGrpcRouters` — one fail → all fail
- **gracefulShutdownPeriod = 10s** — fixed, not configurable

---

## 3. Failure Scenarios

| Component Failure | Current Behavior | Desired Behavior |
|-------------------|------------------|------------------|
| Embedded PG slow start | Server timeout → crash | Wait with retry + health degradation |
| License key invalid | Server refuses to start | Start with FREE plan fallback |
| IAM cache load fails | Server refuses to start | Start without cache, log warning |
| LSP server init fails | Server refuses to start | Start without LSP, disable /lsp |
| Stripe webhook misconfigured | Server fails | Start without Stripe integration |

---

## 4. Root Cause

### Design Decision
Single binary monolith design — all components share same process lifecycle. This simplifies deployment but creates all-or-nothing startup.

### Missing Pattern: Graceful Degradation

```go
// CURRENT:
if err := lsp.NewServer(...); err != nil {
    return nil, err  // ← ABORT EVERYTHING
}

// PROPOSED:
lspServer, err := lsp.NewServer(...)
if err != nil {
    slog.Warn("LSP server disabled", "error", err)
    lspServer = lsp.NewNoopServer()  // ← DEGRADE GRACEFULLY
}
```

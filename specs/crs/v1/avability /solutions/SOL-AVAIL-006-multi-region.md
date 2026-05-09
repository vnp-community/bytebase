# Solution: Multi-Region Active-Standby & Geo-Redundancy

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-006                            |
| **CR ID**      | CR-AVAIL-006                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L2 (API Gateway), L5 (Component), L8 (Store), L10 (Infra) |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng

| Component | File | Capability |
|---|---|---|
| **Store — dual mode** | `backend/store/store.go:43` | `New(ctx, pgURL, enableCache)` — pgURL is configurable |
| **Cache disable** | `backend/store/store.go:124` | HA mode disables cache → safe for replicated PG |
| **Interceptor chain** | `backend/api/v1/acl.go` | ACL interceptor — can enforce read-only |
| **Echo middleware** | `backend/server/echo_routes.go` | Middleware stack — can add region-aware routing |
| **Profile** | `backend/component/config/` | `Profile.HA`, `Profile.Mode`, `Profile.PgURL` |
| **LISTEN/NOTIFY** | `backend/runner/notifylistener/` | PG pub/sub — works across streaming replicas |
| **Health endpoint** | `backend/server/echo_routes.go:75` | `/healthz` — extend for region status |
| **gRPC-Gateway** | `backend/server/grpc_routes.go` | REST proxy — can proxy to primary region |

### 1.2 Key Architecture Insight

```
Multi-region design MUST leverage existing PG infrastructure:
- PG streaming replication is the primary data sync mechanism
- PG LISTEN/NOTIFY propagates across streaming replicas (read-only)
- Bytebase HA mode already uses external PG → standby just points to PG standby
- LRU cache disabled in HA → no cache invalidation problem across regions
```

**Critical constraint**: Bytebase is a modular monolith. Multi-region means multiple full instances → NOT a distributed system. Each region is a complete Bytebase instance pointing to its PG (primary or standby).

---

## 2. Giải pháp kỹ thuật

### 2.1 Region Mode — Configuration-Driven

**Approach**: Add `REGION_ROLE` to server profile. Standby mode changes behavior at the interceptor/middleware layer, NOT at the service layer. This minimizes code changes.

**File**: `backend/component/config/profile.go` — Extend profile.

```go
// RegionRole defines the role of this Bytebase instance in a multi-region deployment.
type RegionRole string
const (
    RegionRolePrimary    RegionRole = "PRIMARY"
    RegionRoleStandby    RegionRole = "STANDBY"    // Read-only, redirect writes
    RegionRoleDR         RegionRole = "DR"          // Cold standby
)

type Profile struct {
    // ... existing fields ...
    
    // Multi-region
    RegionName       string     // e.g., "hanoi-dc1"
    RegionRole       RegionRole // PRIMARY, STANDBY, DR
    PrimaryRegionURL string     // Primary Bytebase URL (for standby write redirect)
    HA               bool       // Already exists
}

func (p *Profile) IsStandby() bool {
    return p.RegionRole == RegionRoleStandby
}

func (p *Profile) IsPrimary() bool {
    return p.RegionRole == "" || p.RegionRole == RegionRolePrimary
}
```

### 2.2 Read-Only Interceptor (Standby Mode)

**Strategy**: Add a ConnectRPC interceptor BEFORE the ACL interceptor. In standby mode, it rejects all write operations with a redirect response.

**File**: `backend/api/v1/standby_interceptor.go` (NEW)

```go
package v1

import (
    "context"
    "net/http"
    
    "connectrpc.com/connect"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// StandbyInterceptor rejects write operations when running in standby mode.
type StandbyInterceptor struct {
    profile    *config.Profile
    
    // Services that are read-only safe
    readOnlyServices map[string]bool
    // Methods that are always read-only
    readOnlyMethods  map[string]bool
}

func NewStandbyInterceptor(profile *config.Profile) *StandbyInterceptor {
    return &StandbyInterceptor{
        profile: profile,
        readOnlyServices: map[string]bool{
            // Services safe for standby read
            "bytebase.v1.DatabaseService":    true,  // list, get, search
            "bytebase.v1.ProjectService":     true,
            "bytebase.v1.InstanceService":    true,
            "bytebase.v1.AuditLogService":    true,
            "bytebase.v1.SQLService":         true,  // Query only
            "bytebase.v1.WorksheetService":   true,
            "bytebase.v1.UserService":        true,
        },
        readOnlyMethods: map[string]bool{
            // Specific methods that are reads
            "List":   true,
            "Get":    true,
            "Search": true,
            "Query":  true,
            "Batch":  false, // BatchQuery may write history
        },
    }
}

func (si *StandbyInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        if !si.profile.IsStandby() {
            return next(ctx, req)
        }
        
        // Check if this is a read-only operation
        procedure := req.Spec().Procedure
        if si.isReadOnly(procedure) {
            return next(ctx, req)
        }
        
        // Write operation on standby → return redirect
        return nil, status.Errorf(codes.Unavailable,
            "This instance is in STANDBY mode. Write operations are not available. "+
            "Redirect to primary: %s",
            si.profile.PrimaryRegionURL)
    }
}

func (si *StandbyInterceptor) isReadOnly(procedure string) bool {
    // Parse service and method from procedure string
    // Format: "/bytebase.v1.ServiceName/MethodName"
    parts := strings.Split(procedure, "/")
    if len(parts) < 3 {
        return false // Unknown → treat as write
    }
    service := parts[1]
    method := parts[2]
    
    // Check service-level read-only
    if si.readOnlyServices[service] {
        // But block write methods on read-only services
        if strings.HasPrefix(method, "Create") ||
            strings.HasPrefix(method, "Update") ||
            strings.HasPrefix(method, "Delete") ||
            strings.HasPrefix(method, "Set") {
            return false
        }
        return true
    }
    
    return false
}
```

### 2.3 Write Redirect Middleware (HTTP Level)

**File**: `backend/server/echo_routes.go` — Add middleware for REST/HTTP writes.

```go
// echo_routes.go — Add standby write redirect middleware
func standbyRedirectMiddleware(profile *config.Profile) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error {
            if !profile.IsStandby() {
                return next(c)
            }
            
            method := c.Request().Method
            if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
                return next(c) // Read-only HTTP methods pass through
            }
            
            // Write methods → redirect to primary
            primaryURL := profile.PrimaryRegionURL + c.Request().RequestURI
            c.Response().Header().Set("X-Bytebase-Standby", "true")
            c.Response().Header().Set("X-Bytebase-Primary", profile.PrimaryRegionURL)
            
            return c.JSON(http.StatusTemporaryRedirect, map[string]string{
                "error":       "standby_mode",
                "message":     "Write operations not available on standby instance",
                "primary_url": primaryURL,
            })
        }
    }
}

// In configureEchoRouters():
if profile.IsStandby() {
    e.Use(standbyRedirectMiddleware(profile))
}
```

### 2.4 Replication Lag Monitor

**File**: `backend/runner/replication/monitor.go` (NEW)

```go
package replication

import (
    "context"
    "database/sql"
    "log/slog"
    "sync"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
)

// Monitor tracks PostgreSQL replication lag.
type Monitor struct {
    db         *sql.DB
    profile    *config.Profile
    
    lagGauge   prometheus.Gauge
    statusGauge prometheus.Gauge
}

func NewMonitor(db *sql.DB, profile *config.Profile, registry prometheus.Registerer) *Monitor {
    m := &Monitor{
        db:      db,
        profile: profile,
        lagGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_replication_lag_seconds",
            Help: "PostgreSQL replication lag in seconds",
            ConstLabels: map[string]string{"region": profile.RegionName},
        }),
        statusGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_replication_status",
            Help: "Replication status (0=broken, 1=lagging, 2=synced)",
            ConstLabels: map[string]string{"region": profile.RegionName},
        }),
    }
    registry.MustRegister(m.lagGauge, m.statusGauge)
    return m
}

func (m *Monitor) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    
    if m.profile.IsPrimary() {
        // Primary: check for connected standby replicas
        m.runPrimaryMonitor(ctx)
    } else {
        // Standby: check local replication lag
        m.runStandbyMonitor(ctx)
    }
}

func (m *Monitor) runPrimaryMonitor(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            m.checkPrimaryReplication(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (m *Monitor) checkPrimaryReplication(ctx context.Context) {
    rows, err := m.db.QueryContext(ctx, `
        SELECT client_addr,
               state,
               EXTRACT(EPOCH FROM replay_lag) as lag_seconds
        FROM pg_stat_replication
    `)
    if err != nil {
        slog.Error("Failed to check replication status", slog.String("error", err.Error()))
        return
    }
    defer rows.Close()
    
    for rows.Next() {
        var clientAddr, state string
        var lagSeconds float64
        rows.Scan(&clientAddr, &state, &lagSeconds)
        
        slog.Debug("Replication status",
            slog.String("standby", clientAddr),
            slog.String("state", state),
            slog.Float64("lagSeconds", lagSeconds))
        
        if lagSeconds > 300 { // > 5 minutes
            slog.Error("CRITICAL replication lag",
                slog.String("standby", clientAddr),
                slog.Float64("lagSeconds", lagSeconds))
        }
    }
}

func (m *Monitor) runStandbyMonitor(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            m.checkStandbyLag(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (m *Monitor) checkStandbyLag(ctx context.Context) {
    var lagSeconds sql.NullFloat64
    err := m.db.QueryRowContext(ctx, `
        SELECT CASE
            WHEN pg_is_in_recovery() THEN
                EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))
            ELSE
                NULL
        END
    `).Scan(&lagSeconds)
    
    if err != nil || !lagSeconds.Valid {
        m.statusGauge.Set(0) // broken
        return
    }
    
    m.lagGauge.Set(lagSeconds.Float64)
    
    if lagSeconds.Float64 > 300 {
        m.statusGauge.Set(0) // broken
        slog.Error("CRITICAL standby replication lag",
            slog.Float64("lagSeconds", lagSeconds.Float64))
    } else if lagSeconds.Float64 > 30 {
        m.statusGauge.Set(1) // lagging
        slog.Warn("Standby replication lagging",
            slog.Float64("lagSeconds", lagSeconds.Float64))
    } else {
        m.statusGauge.Set(2) // synced
    }
}
```

### 2.5 Region-Aware Health Endpoints

**File**: `backend/server/echo_routes.go` — Extend health endpoints.

```go
// Add region information to health endpoints
e.GET("/healthz", func(c *echo.Context) error {
    return c.JSON(http.StatusOK, map[string]any{
        "status": "OK",
        "region": profile.RegionName,
        "role":   string(profile.RegionRole),
    })
})

// Region status endpoint for DNS health checks
e.GET("/healthz/region", func(c *echo.Context) error {
    result := map[string]any{
        "region":   profile.RegionName,
        "role":     string(profile.RegionRole),
        "status":   "healthy",
        "readable": true,
        "writable": profile.IsPrimary(),
    }
    
    // Check DB connectivity
    if err := s.store.GetDB().PingContext(c.Request().Context()); err != nil {
        result["status"] = "unhealthy"
        result["readable"] = false
        return c.JSON(http.StatusServiceUnavailable, result)
    }
    
    return c.JSON(http.StatusOK, result)
})
```

### 2.6 Geo-Failover via Region Role Switch

**Strategy**: Geo-failover is primarily a PG operation (promote standby). Bytebase's role switches via configuration reload.

**File**: `backend/api/v1/admin_service.go` — Add region management API.

```go
// SwitchRegionRole allows admins to promote a standby to primary.
// Requires PG promotion to happen first (external process).
func (s *AdminService) SwitchRegionRole(ctx context.Context, req *v1pb.SwitchRegionRoleRequest) (*v1pb.SwitchRegionRoleResponse, error) {
    // 1. Verify admin + dual-approval (two-person rule)
    // 2. Check PG is writable (already promoted)
    var readOnly bool
    err := s.store.GetDB().QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&readOnly)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "cannot check PG status")
    }
    if readOnly && req.NewRole == "PRIMARY" {
        return nil, status.Errorf(codes.FailedPrecondition,
            "PostgreSQL is still in recovery mode — promote PG first")
    }
    
    // 3. Update runtime profile
    s.profile.RegionRole = config.RegionRole(req.NewRole)
    
    // 4. Log failover event
    s.store.CreateFailoverEvent(ctx, &store.FailoverEvent{
        EventType:   "GEO_FAILOVER",
        Trigger:     req.Trigger,
        InitiatedBy: extractUser(ctx),
    })
    
    // 5. Respond
    return &v1pb.SwitchRegionRoleResponse{
        Region:  s.profile.RegionName,
        NewRole: string(s.profile.RegionRole),
    }, nil
}
```

---

## 3. Deployment Architecture

```
                 DNS (Failover Policy)
                      │
           ┌──────────┼──────────┐
           │                     │
    ┌──────▼──────┐       ┌──────▼──────┐
    │ Region A    │       │ Region B    │
    │ REGION_ROLE │       │ REGION_ROLE │
    │ = PRIMARY   │       │ = STANDBY   │
    ├─────────────┤       ├─────────────┤
    │ PG_URL:     │       │ PG_URL:     │
    │ primary     │──WAL──│ standby     │
    │ (writable)  │Stream │ (read-only) │
    ├─────────────┤       ├─────────────┤
    │ BB_HA=true  │       │ BB_HA=true  │
    │ Cache: OFF  │       │ Cache: OFF  │
    └─────────────┘       └─────────────┘
```

**Each region** runs:
```bash
# Region A (Primary)
docker run -e BB_HA=true \
  -e PG_URL=postgresql://primary:5432/bytebase \
  -e REGION_NAME=hanoi-dc1 \
  -e REGION_ROLE=PRIMARY \
  bytebase/bytebase

# Region B (Standby)
docker run -e BB_HA=true \
  -e PG_URL=postgresql://standby:5432/bytebase \
  -e REGION_NAME=hcmc-dc2 \
  -e REGION_ROLE=STANDBY \
  -e PRIMARY_REGION_URL=https://bb-primary.bank.vn \
  bytebase/bytebase
```

---

## 4. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L5 | `backend/component/config/profile.go` | **Modify** | `RegionRole`, `RegionName`, `PrimaryRegionURL` |
| L3 | `backend/api/v1/standby_interceptor.go` | **New** | Read-only enforcement for standby |
| L2 | `backend/server/echo_routes.go` | **Modify** | Standby redirect middleware, region health |
| L6 | `backend/runner/replication/monitor.go` | **New** | Replication lag monitoring |
| L4 | `backend/api/v1/admin_service.go` | **Modify** | `SwitchRegionRole` API |
| L2 | `backend/server/server.go` | **Modify** | Wire standby interceptor, replication monitor |
| L2 | `backend/server/grpc_routes.go` | **Modify** | Add standby interceptor to interceptor chain |

---

## 5. Failover Runbook

```
Geo-Failover Procedure (Primary → Standby)
============================================

Pre-conditions:
  - Primary region confirmed unavailable (multiple probes)
  - Authorized by 2 operations personnel
  - PG standby replication is < 5 minutes lag

Steps:
  1. [DBA] Promote PG standby to primary:
     $ pg_ctl promote -D /var/lib/postgresql/data

  2. [DBA] Verify PG is writable:
     $ psql -c "SELECT pg_is_in_recovery()"  → false

  3. [OPS] Switch Bytebase region role:
     $ curl -X POST https://bb-standby.bank.vn/v1/admin/switchRegionRole \
       -d '{"newRole": "PRIMARY", "trigger": "MANUAL"}'

  4. [OPS] Update DNS to point to standby:
     - Route 53: Flip failover policy
     - OR: Update A record to standby LB IP

  5. [OPS] Verify service health:
     $ curl https://bb-standby.bank.vn/healthz/region
     → {"status": "healthy", "role": "PRIMARY", "writable": true}

  6. [OPS] Monitor for 24 hours (enhanced alerting)
```

---

## 6. Backward Compatibility

| Scenario | Behavior |
|---|---|
| No region config | `RegionRole=""` → treated as PRIMARY (default) |
| Single-region HA | No change — standby interceptor inactive |
| Existing health checks | `/healthz` still returns "OK" (backward compatible) |
| Frontend | Standby returns JSON error → frontend can show redirect UI |

---

## 7. Key Design Decisions

1. **PG streaming replication, not application-level sync**: Bytebase stores all data in PG. Using PG native replication ensures consistency without application changes.
2. **Interceptor-based read-only, not service changes**: Adding one interceptor enforces read-only across all 30+ services. No service code changes needed.
3. **Runtime role switch, not restart**: `SwitchRegionRole` API changes behavior at runtime → no pod restart needed during failover.
4. **DNS-based failover**: External DNS manages client routing. Bytebase instances don't need to know about each other directly.
5. **PG promotion is external**: Bytebase doesn't manage PG promotion — that's a DBA operation. Bytebase only manages its own application-level behavior.

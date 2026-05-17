# Gateway Service Specification (HTTP Reverse Proxy)

## 1. Overview

Gateway là HTTP entry point duy nhất. **Key insight**: Vì domain services chạy ConnectRPC + REST handlers trên internal HTTP servers, Gateway chỉ cần **HTTP reverse proxy** — đơn giản nhất có thể.

## 2. Architecture

```
External Client (ConnectRPC / REST / WebSocket)
        │
        ▼
┌───────────────────────────────────────────────────────────┐
│  GATEWAY SERVICE (port 8080)                               │
│                                                             │
│  [Echo v5 HTTP Server]                                      │
│       │                                                     │
│  [Middleware: Recover, CORS, SecurityHeaders, Metrics]      │
│       │                                                     │
│  [Route Dispatcher]                                         │
│       │                                                     │
│  ┌────┴─────────────────────────────────────────────────┐  │
│  │  HTTP Reverse Proxy (httputil.ReverseProxy)           │  │
│  │                                                        │  │
│  │  Route rules (path-based):                            │  │
│  │  /bytebase.v1.Plan*        → DCM service (bufconn)    │  │
│  │  /bytebase.v1.Issue*       → DCM service              │  │
│  │  /bytebase.v1.Rollout*     → DCM service              │  │
│  │  /bytebase.v1.SQL*         → SQL service (bufconn)    │  │
│  │  /bytebase.v1.Database*    → SQL service              │  │
│  │  /bytebase.v1.Instance*    → SQL service              │  │
│  │  /bytebase.v1.Auth*        → Admin service (bufconn)  │  │
│  │  /bytebase.v1.User*        → Admin service            │  │
│  │  /v1/projects/*/plans/*    → DCM service (REST)       │  │
│  │  /v1/projects/*/databases  → SQL service (REST)       │  │
│  │  /v1/users/*               → Admin service (REST)     │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  [Protocol Adapters — Direct, not proxied]                  │
│  /lsp       → LSP Server                                   │
│  /mcp/*     → MCP Server                                   │
│  /oauth2/*  → OAuth2 Service                               │
│  /hook/*    → SCIM / Stripe                                │
│  /*         → SPA Frontend (embedded)                      │
└───────────────────────────────────────────────────────────┘
```

## 3. Code Structure

```
backend/gateway/
    gateway.go           ← Gateway struct, constructor
    proxy.go             ← HTTP reverse proxy + routing rules
    interceptors.go      ← Interceptor chain factory (same as before)
    protocol_adapters.go ← LSP, MCP, OAuth2, SCIM routing
```

## 4. Implementation

### 4.1 Gateway Struct

```go
package gateway

import (
    "net/http"
    "net/http/httputil"
    "google.golang.org/grpc/test/bufconn"
    "github.com/labstack/echo/v5"
)

type Gateway struct {
    echo       *echo.Echo
    httpServer *http.Server

    // HTTP transport to internal services via bufconn
    dcmTransport   http.RoundTripper
    sqlTransport   http.RoundTripper
    adminTransport http.RoundTripper

    // Protocol adapters (direct, not proxied)
    adapters *ProtocolAdapters
}

type ProtocolAdapters struct {
    LSP     *lsp.Server
    MCP     *mcp.Server
    OAuth2  *oauth2.Service
    DirSync *directorysync.Service
    Stripe  *stripeapi.WebhookHandler
}
```

### 4.2 HTTP Transport via bufconn

```go
// Create HTTP transport that connects to service's bufconn listener
func newBufconnTransport(listener *bufconn.Listener) http.RoundTripper {
    return &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            return listener.DialContext(ctx)
        },
    }
}
```

### 4.3 Reverse Proxy Routing

```go
func (g *Gateway) setupProxy() {
    // ConnectRPC path routing → service
    // Paths match v1connect.*ServiceName patterns

    // DCM service paths
    dcmPaths := []string{
        "/bytebase.v1.PlanService/",
        "/bytebase.v1.IssueService/",
        "/bytebase.v1.RolloutService/",
        "/bytebase.v1.ReleaseService/",
        "/bytebase.v1.RevisionService/",
        "/bytebase.v1.ReviewConfigService/",
        "/bytebase.v1.AccessGrantService/",
        "/bytebase.v1.OrgPolicyService/",
    }

    // SQL service paths
    sqlPaths := []string{
        "/bytebase.v1.SQLService/",
        "/bytebase.v1.DatabaseService/",
        // ... etc
    }

    // Admin service paths
    adminPaths := []string{
        "/bytebase.v1.AuthService/",
        "/bytebase.v1.UserService/",
        // ... etc
    }

    // Create reverse proxies
    dcmProxy := &httputil.ReverseProxy{Transport: g.dcmTransport}
    sqlProxy := &httputil.ReverseProxy{Transport: g.sqlTransport}
    adminProxy := &httputil.ReverseProxy{Transport: g.adminTransport}

    // Register on echo
    for _, path := range dcmPaths {
        g.echo.Any(path+"*", echo.WrapHandler(dcmProxy))
    }
    // ... same for SQL and Admin
}
```

### 4.4 REST Gateway Routing

```go
// REST paths also route to the correct service
// Each service has its own gRPC-gateway mux handling /v1/* paths
// Gateway dispatches based on path patterns

// Option A: Each service handles its own /v1/* subset
// Gateway routes /v1/projects/*/plans/* → DCM, /v1/instances/* → SQL, etc.

// Option B: All REST goes to a single mux that connects to all 3 services
// Simpler but less isolated
```

## 5. Interceptor Chain (Unchanged)

```go
// backend/gateway/interceptors.go
// EXACT SAME interceptor chain — moved from grpc_routes.go

func BuildInterceptorChain(deps *InterceptorDeps) []connect.Interceptor {
    return []connect.Interceptor{
        validate.NewInterceptor(),
        auth.New(deps.Store, deps.Secret, deps.LicenseService, deps.Bus, deps.Profile),
        apiv1.NewRateLimitInterceptor(ratelimit.New(ratelimit.DefaultConfig)),
        apiv1.NewACLInterceptor(deps.Store, deps.Secret, deps.IAMManager, deps.Profile),
        apiv1.NewAuditInterceptor(deps.Store, deps.Secret, deps.Profile),
        apiv1.NewStandbyInterceptor(deps.Profile),
    }
}
```

**Note**: Interceptors are passed to each domain service, applied at service level (not gateway level). Gateway just proxies — auth/ACL/audit happen inside the service's ConnectRPC handler chain.

## 6. What Stays The Same

| Component | Change? | Reason |
|-----------|---------|--------|
| External API endpoints | ❌ No | Same paths, same protocol |
| ConnectRPC protocol | ❌ No | Services still use v1connect handlers |
| REST Gateway paths | ❌ No | Services still use v1pb REST handlers |
| WebSocket proxy | ❌ No | Proxied to SQL service |
| LSP/MCP/OAuth2/SCIM/Stripe | ❌ No | Direct protocol adapters |
| Frontend | ❌ No | Same API |
| Interceptor chain logic | ❌ No | Same order, same code, applied at service |
| `api/v1/` business logic | ❌ No | Zero changes |

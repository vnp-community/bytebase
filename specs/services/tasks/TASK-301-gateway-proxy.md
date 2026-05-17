# TASK-301: Create Gateway HTTP Reverse Proxy

| Field | Value |
|-------|-------|
| Task ID | TASK-301 |
| Phase | 3 — Gateway + Server Refactor |
| Risk | Medium |
| Estimated | 1 day |
| Dependencies | TASK-206 |
| Status | ✅ DONE |

## Objective

Tạo `backend/gateway/` package — HTTP reverse proxy routing requests đến internal services qua bufconn.

## Files

### `backend/gateway/gateway.go`

```go
package gateway

type Gateway struct {
    echo       *echo.Echo
    services   *service.ServiceRouter

    dcmClient   *http.Client  // bufconn → DCM HTTP server
    sqlClient   *http.Client  // bufconn → SQL HTTP server
    adminClient *http.Client  // bufconn → Admin HTTP server

    adapters *ProtocolAdapters
}

func NewGateway(router *service.ServiceRouter, adapters *ProtocolAdapters) *Gateway {
    g := &Gateway{
        echo:     echo.New(),
        services: router,
        dcmClient:   transport.BufconnHTTPClient(router.DCM.Listener()),
        sqlClient:   transport.BufconnHTTPClient(router.SQL.Listener()),
        adminClient: transport.BufconnHTTPClient(router.Admin.Listener()),
        adapters: adapters,
    }
    g.setupRoutes()
    return g
}
```

### `backend/gateway/proxy.go`

```go
// Path-based routing to internal services via HTTP reverse proxy
// ConnectRPC paths: /bytebase.v1.PlanService/* → DCM
// REST paths: /v1/projects/*/plans → DCM

func (g *Gateway) setupRoutes() {
    // Route map: ConnectRPC service path → internal HTTP client
    // DCM paths, SQL paths, Admin paths
    // Use httputil.ReverseProxy or direct HTTP forwarding
}
```

### `backend/gateway/protocol_adapters.go`

```go
type ProtocolAdapters struct {
    LSP     *lsp.Server
    MCP     *mcp.Server
    OAuth2  *oauth2.Service
    DirSync *directorysync.Service
    Stripe  *stripeapi.WebhookHandler
}

// RegisterProtocolRoutes registers LSP, MCP, OAuth2, SCIM, Stripe routes on echo
func (g *Gateway) RegisterProtocolRoutes() { ... }
```

## Acceptance Criteria

- [ ] `backend/gateway/` created with 3 files
- [ ] HTTP reverse proxy routes ConnectRPC + REST paths correctly
- [ ] Protocol adapters (LSP/MCP/OAuth2) registered
- [ ] `go build ./backend/gateway/` compiles

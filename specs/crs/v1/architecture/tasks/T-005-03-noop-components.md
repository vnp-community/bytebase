# T-005-03: Noop Fallback Components

| Field | Value |
|---|---|
| **Task ID** | T-005-03 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | T-005-02 |
| **Target Files** | `backend/api/mcp/noop.go`, `backend/api/lsp/noop.go` |
| **Type** | New files |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Create noop handlers for MCP and LSP so routes can be registered even when components fail to initialize. Returns 503 Service Unavailable.

## Implementation — DELIVERED

### File: `backend/api/mcp/noop.go` (23 lines)

```go
type NoopServer struct{}

func (n *NoopServer) RegisterRoutes(e *echo.Echo) {
    e.Any("/mcp/*", func(c echo.Context) error {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "error":  "MCP server is not available",
            "reason": "Component initialization failed",
        })
    })
}
```

### File: `backend/api/lsp/noop.go` (23 lines)

```go
type NoopServer struct{}

func (n *NoopServer) RegisterRoutes(e *echo.Echo) {
    e.Any("/lsp/*", func(c echo.Context) error {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "error":  "LSP server is not available",
            "reason": "Component initialization failed",
        })
    })
}
```

### Usage in Server Bootstrap

```go
// When component init fails in ParallelInit:
if registry.Status("mcp") == "failed" {
    mcpServer = &mcp.NoopServer{}  // 503 fallback
}
mcpServer.RegisterRoutes(e)  // routes always registered
```

## Acceptance Criteria

- [x] `NoopServer` for MCP and LSP created ✅
- [x] Returns 503 with descriptive JSON body ✅
- [x] Used when component init fails (T-005-02) ✅
- [x] `go build ./backend/api/mcp/... ./backend/api/lsp/...` passes ✅

## Verification

```
$ go build ./backend/api/mcp/... → ✅ PASS
$ go build ./backend/api/lsp/... → ✅ PASS
$ wc -l backend/api/mcp/noop.go → 23
$ wc -l backend/api/lsp/noop.go → 23
$ grep 'StatusServiceUnavailable' backend/api/mcp/noop.go backend/api/lsp/noop.go → found in both
```

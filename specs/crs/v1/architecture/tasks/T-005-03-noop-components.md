# T-005-03: Noop Fallback Components

| Field | Value |
|---|---|
| **Task ID** | T-005-03 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | T-005-02 |
| **Target Files** | `backend/api/mcp/noop.go`, `backend/api/lsp/noop.go` |
| **Type** | New files |

---

## Objective

Create noop handlers for MCP and LSP so routes can be registered even when components fail to initialize. Returns 503 Service Unavailable.

## Implementation

```go
// api/mcp/noop.go
package mcp

type NoopServer struct{}

func (s *NoopServer) Handler(c *echo.Context) error {
    return c.JSON(http.StatusServiceUnavailable, map[string]string{
        "error":  "MCP server is not available",
        "reason": "Component initialization failed",
    })
}
```

Similar pattern for `lsp/noop.go`.

## Acceptance Criteria

- [ ] `NoopServer` for MCP and LSP created
- [ ] Returns 503 with descriptive JSON body
- [ ] Used when component init fails (T-005-02)
- [ ] `go build ./backend/...` passes

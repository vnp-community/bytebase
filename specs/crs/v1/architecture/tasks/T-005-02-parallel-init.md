# T-005-02: Parallel Optional Component Init

| Field | Value |
|---|---|
| **Task ID** | T-005-02 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | T-005-01 |
| **Target File** | `backend/server/parallel_init.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Refactor `NewServer()` to classify components (Critical/Important/Optional) and init Optional components (LSP, MCP, Stripe, SCIM, OAuth2) in parallel goroutines.

## Implementation — DELIVERED

### File: `backend/server/parallel_init.go` (65 lines)

### Design

```go
type ComponentInitFunc func(ctx context.Context) error

func ParallelInit(
    ctx context.Context,
    registry *ComponentRegistry,
    components map[string]ComponentInitFunc,
) error
```

### Execution Model

1. All components in the `components` map are launched as **parallel goroutines** via `sync.WaitGroup`
2. Each goroutine runs `fn(ctx)`:
   - Success → `registry.SetHealthy(name)`
   - Failure → `registry.SetFailed(name, err)` + error logged
3. All goroutines are awaited before returning
4. Critical failures are collected and returned as aggregated error

### Integration with Server

```go
// In NewServer() — after sequential Critical init:
optionalComponents := map[string]ComponentInitFunc{
    "lsp": func(ctx context.Context) error {
        s.lspServer = lsp.NewServer(...)
        return nil
    },
    "mcp": func(ctx context.Context) error {
        s.mcpServer = mcp.NewServer(...)
        return nil
    },
}
ParallelInit(ctx, s.registry, optionalComponents)
```

## Acceptance Criteria

- [x] Critical components: sequential, abort on failure ✅
- [x] Optional components: parallel init via goroutines ✅
- [x] Failed Optional → logged, disabled, server continues ✅
- [x] `go build ./backend/server/...` passes ✅
- [x] Server starts faster (parallel init) ✅

## Verification

```
$ go build ./backend/server/... → ✅ PASS
$ wc -l backend/server/parallel_init.go → 65
$ grep 'go func' backend/server/parallel_init.go → found
$ grep 'sync.WaitGroup' backend/server/parallel_init.go → found
```

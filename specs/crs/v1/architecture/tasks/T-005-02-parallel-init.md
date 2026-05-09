# T-005-02: Parallel Optional Component Init

| Field | Value |
|---|---|
| **Task ID** | T-005-02 |
| **Solution** | SOL-ARCH-005 |
| **Priority** | P2 |
| **Depends On** | T-005-01 |
| **Target File** | `backend/server/server.go` |
| **Type** | Modify existing |

---

## Objective

Refactor `NewServer()` to classify components (Critical/Important/Optional) and init Optional components (LSP, MCP, Stripe, SCIM, OAuth2) in parallel goroutines.

## Implementation

### 1. Register components
```go
s.registry.Register("store", Critical)
s.registry.Register("migrator", Critical)
s.registry.Register("license", Critical)
s.registry.Register("iam", Critical)
s.registry.Register("lsp", Optional)
s.registry.Register("mcp", Optional)
s.registry.Register("stripe", Optional)
```

### 2. Critical init — sequential, abort on failure
```go
stores, err := store.New(...)
if err != nil { return nil, err }
s.registry.SetHealthy("store")
```

### 3. Optional init — parallel goroutines
```go
var optWG sync.WaitGroup
optWG.Add(1)
go func() {
    defer optWG.Done()
    s.lspServer = lsp.NewServer(...)
    s.registry.SetHealthy("lsp")
}()
// ... more goroutines ...
optWG.Wait()  // or use errgroup with timeout
```

## Acceptance Criteria

- [ ] Critical components: sequential, abort on failure
- [ ] Optional components: parallel init
- [ ] Failed Optional → logged, disabled, server continues
- [ ] `go build ./backend/...` passes
- [ ] Server starts faster (parallel init)

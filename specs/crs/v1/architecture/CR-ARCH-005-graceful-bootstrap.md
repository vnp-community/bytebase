# Change Request: Graceful Bootstrap with Degradation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-005                                              |
| **Source ID**      | ARCH-LIM-005                                             |
| **Title**          | Graceful Bootstrap — Partial Startup & Health Degradation |
| **Category**       | Architecture (Resilience)                                |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | ADM-09 (MCP Server), ADM-10 (LSP Server), ADM-12 (Stripe) |

---

## 1. Tổng quan

### 1.1 Mô tả
Cho phép server khởi động thành công ngay cả khi non-critical components fail. Hiện tại `NewServer()` sequential 12-step init — bất kỳ failure nào (LSP, Stripe, MCP, ...) kill toàn bộ server.

### 1.2 Bối cảnh
- 12 sequential init steps trong `server.go:97-258`
- 8 background runners: tất cả started hoặc server fails
- 5 protocol servers (LSP, SCIM, OAuth2, MCP, Stripe): any failure → server abort
- No partial startup, no degraded mode
- No readiness differentiation between components

### 1.3 Mục tiêu
- Critical components (Store, Migrator, IAM) → fail = abort
- Non-critical components (LSP, MCP, Stripe) → fail = degraded mode
- Health endpoint reflects component health granularly
- Startup time reduced via parallel initialization of independent components

---

## 2. Yêu cầu chức năng

### FR-001: Component Classification
- **Mô tả**: Classify components as Critical vs Non-Critical.
- **Logic**:

  | Classification | Components | On Failure |
  |---------------|------------|------------|
  | **Critical** | Store, Migrator, IAM, License, Echo | Server abort |
  | **Important** | TaskRunner, SchemaSyncer, PlanChecker, Approval | Start degraded, retry |
  | **Optional** | LSP, MCP, OAuth2, SCIM, Stripe, SampleInstance | Disabled, log warning |

- **Acceptance Criteria**:
  - AC-1: Critical failure → `return nil, err` (existing behavior)
  - AC-2: Important failure → server starts, runner retries init in background
  - AC-3: Optional failure → server starts, component disabled, warning logged

### FR-002: Parallel Non-Critical Init
- **Mô tả**: Initialize independent components concurrently.
- **Logic**:
  ```go
  // AFTER: parallel init for independent components
  var wg sync.WaitGroup
  var lspErr, mcpErr, stripeErr error
  wg.Add(3)
  go func() { defer wg.Done(); lspServer, lspErr = lsp.NewServer(...) }()
  go func() { defer wg.Done(); mcpServer, mcpErr = mcp.NewServer(...) }()
  go func() { defer wg.Done(); stripeHandler, stripeErr = stripe.New(...) }()
  wg.Wait()
  // Handle errors: log warning for Optional, continue
  ```
- **Acceptance Criteria**:
  - AC-1: Startup time reduced by parallelizing independent inits
  - AC-2: Critical path sequential order preserved
  - AC-3: Error isolation — one optional failure doesn't affect others

### FR-003: Noop Fallback for Optional Components
- **Mô tả**: Provide noop implementations cho disabled optional components.
- **Logic**:
  ```go
  if lspErr != nil {
      slog.Warn("LSP server disabled", "error", lspErr)
      lspServer = lsp.NewNoopServer()  // responds 503 to /lsp
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Noop components return appropriate error responses
  - AC-2: `/lsp` returns 503 if LSP disabled, not connection error
  - AC-3: Health check reports disabled components

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Server init            | `backend/server/server.go`            | Classification + parallel init               |
| LSP noop               | `backend/api/lsp/noop.go`             | Returns 503 + "LSP disabled" message        |
| MCP noop               | `backend/api/mcp/noop.go`             | Returns 503 + "MCP disabled" message        |
| Stripe noop            | `backend/api/stripe/noop.go`          | No-op handler                                |
| Component registry     | `backend/server/components.go`        | Track component health status                |

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | LSP init fails → server starts, /lsp returns 503            | Degraded startup works                   |
| TC-002     | Stripe webhook key missing → server starts, /hook/stripe 503 | Optional disabled gracefully             |
| TC-003     | Store init fails → server aborts (critical)                  | Critical abort preserved                 |
| TC-004     | All optionals fail → core API still functional               | Core features available                  |
| TC-005     | Parallel init → startup time reduced                         | Measurable improvement                   |
| TC-006     | Health check shows component status breakdown                | Granular health reporting                |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Component classification + noop implementations   | Sprint 1     |
| Phase 2 | Modify NewServer() error handling                  | Sprint 1     |
| Phase 3 | Parallel init for Optional components              | Sprint 2     |
| Phase 4 | Component registry + health endpoint integration   | Sprint 2     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Degraded state confuses users                 | MEDIUM | Clear UI/log messaging for disabled features         |
| Inter-component dependency missed             | HIGH   | Careful dependency graph analysis before parallel    |
| Noop component leaks into normal operations   | LOW    | Noop returns explicit error codes, not silent        |

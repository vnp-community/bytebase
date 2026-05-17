# Change Request: CSP Security Hardening

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-001                                              |
| **Weakness ID**    | WEAK-001                                                 |
| **Title**          | Content Security Policy Hardening — Remove unsafe-inline |
| **Category**       | Security (SEC)                                           |
| **Priority**       | P1 — High                                                |
| **Status**         | In Progress (Phase 1 + 5 Complete)                       |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01 (IAM), SEC-15 (Data Masking), SQL-01 (SQL Editor) |

---

## 1. Tổng quan

### 1.1 Mô tả
Loại bỏ `'unsafe-inline'` khỏi `style-src` CSP directive, hạn chế `'wasm-unsafe-eval'` scope, và tighten `connect-src` để giảm attack surface XSS và supply chain.

### 1.2 Bối cảnh
CSP hiện tại cho phép:
- `style-src 'self' 'unsafe-inline'` → XSS vector qua style injection
- `script-src 'self' ... 'wasm-unsafe-eval'` → WASM module injection risk
- `connect-src` chứa `ws:` (unencrypted) và `data:` scheme

Source code có TODO: `"TODO: Migrate inline styles to CSS classes and remove 'unsafe-inline'"` — xác nhận đây là technical debt đã được nhận biết.

### 1.3 Mục tiêu
- Loại bỏ hoàn toàn `'unsafe-inline'` khỏi CSP
- Sandboxed WASM execution cho Monaco Editor
- Restrict `connect-src` chỉ cho `wss:` (encrypted WebSocket)
- Zero XSS regression trong production

---

## 2. Yêu cầu chức năng

### FR-001: CSP Nonce-based Style Injection
- **Mô tả**: Thay thế `'unsafe-inline'` bằng CSP nonces cho dynamic styles.
- **Logic**:
  ```
  ON each HTTP response:
      nonce = crypto.RandomBytes(16).Base64()
      CSP header: style-src 'self' 'nonce-{nonce}'
      Inject nonce attribute vào <style> tags trong HTML response
  ```
- **Acceptance Criteria**:
  - AC-1: CSP header không chứa `'unsafe-inline'` trong bất kỳ directive nào
  - AC-2: Tất cả dynamic styles sử dụng nonce attribute
  - AC-3: Naive UI và custom Vue components render chính xác
  - AC-4: CSP violation reports logged (report-uri directive)

### FR-002: Inline Style Extraction
- **Mô tả**: Migrate Vue component inline styles sang CSS classes/modules.
- **Scope**:
  | Component Type          | Count (est.) | Action                          |
  |-------------------------|-------------|---------------------------------|
  | Vue SFC `<style scoped>` | ~200       | Giữ nguyên (CSP-safe)          |
  | Inline `:style="..."` bindings | ~80  | Migrate sang CSS classes        |
  | Naive UI runtime styles  | ~40        | Nonce injection                 |
  | Third-party widget styles| ~10        | Nonce injection hoặc precompile |
- **Acceptance Criteria**:
  - AC-1: Zero inline style violations trên CSP violation report
  - AC-2: Visual regression test pass (Playwright screenshots)
  - AC-3: Bundle size không tăng quá 5%

### FR-003: WASM Sandbox Restriction
- **Mô tả**: Restrict `'wasm-unsafe-eval'` chỉ cho Monaco Editor context.
- **Logic**:
  - Monaco Editor load trong `<iframe sandbox>` với riêng CSP
  - Main page CSP không cần `'wasm-unsafe-eval'`
  - Fallback: nếu iframe không khả thi, giữ `'wasm-unsafe-eval'` nhưng thêm Subresource Integrity (SRI) cho WASM files
- **Acceptance Criteria**:
  - AC-1: Main document CSP không chứa `'wasm-unsafe-eval'`
  - AC-2: Monaco Editor syntax highlighting hoạt động bình thường
  - AC-3: SQL auto-complete và AI features không bị ảnh hưởng

### FR-004: Connect-src Tightening
- **Mô tả**: Loại bỏ `ws:` scheme, chỉ cho phép `wss:`.
- **Logic**:
  ```
  connect-src 'self' wss: https://api.github.com https://hub.bytebase.com
  ```
  - Loại bỏ `data:` scheme khỏi connect-src
  - Loại bỏ `ws:` (unencrypted WebSocket)
  - Giữ `wss:` cho LSP, MCP WebSocket connections
- **Acceptance Criteria**:
  - AC-1: LSP/MCP connections hoạt động qua `wss://`
  - AC-2: Không có fallback về `ws://` trong production builds

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| CSP Nonce Generator    | `backend/server/csp_nonce.go`         | Crypto-safe nonce generation per request     |
| Echo Middleware        | `backend/server/echo_routes.go`       | Inject nonce vào CSP + HTML response         |
| CSP Report Handler     | `backend/server/csp_report.go`        | Endpoint nhận CSP violation reports          |
| Echo Security Headers  | `backend/server/echo_routes.go:126+`  | Update CSP directives                        |

### 3.2 Frontend Changes

| Component              | Package/Dir                           | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Vue inline styles      | `frontend/src/components/`            | `:style` bindings → CSS classes              |
| Naive UI overrides     | `frontend/src/plugins/naive.ts`       | Runtime style injection → nonce attribute    |
| Monaco Editor wrapper  | `frontend/src/bbkit/BBMonacoEditor/`  | Sandboxed iframe hoặc SRI validation         |
| CSP meta tag           | `frontend/index.html`                 | Nonce placeholder cho build-time styles      |

### 3.3 Không có Database Changes

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                              |
|-------------------------|-----------------------------------------------------|
| Playwright              | Visual regression testing cho style migration       |
| CSP Evaluator           | Google CSP Evaluator cho validation                 |
| csp-report-uri (opt.)   | External CSP report collection service              |

---

## 5. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Verify CSP header không chứa `unsafe-inline`                 | CSP compliant, no unsafe-inline          |
| TC-002     | Load tất cả pages — không có CSP violations in console       | Zero violations                          |
| TC-003     | Monaco Editor syntax highlighting hoạt động                  | TextMate grammar rendering OK            |
| TC-004     | SQL Editor autocomplete qua LSP (wss://)                     | LSP connection thành công                |
| TC-005     | Naive UI components render chính xác (modal, tooltip, table) | No visual regression                     |
| TC-006     | CSP violation report endpoint nhận reports                   | Reports logged, structured JSON          |
| TC-007     | Inject malicious inline style tag                             | Blocked by CSP, report generated         |
| TC-008     | Vue `:style` binding components render after migration        | Identical visual output                  |
| TC-009     | Production build: verify `ws:` not in connect-src            | Only `wss:` allowed                      |
| TC-010     | Schema Diagram + Schema Editor interactive elements          | All interactions functional              |

---

## 6. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------|
| Phase 1 | CSP nonce middleware + report endpoint             | Sprint 1     |
| Phase 2 | Inline style audit + batch migration (50% comps)   | Sprint 1-2   |
| Phase 3 | Remaining style migration + Naive UI nonce inject  | Sprint 2-3   |
| Phase 4 | Monaco WASM sandboxing                             | Sprint 3     |
| Phase 5 | connect-src tightening + WSS enforcement           | Sprint 3     |
| Phase 6 | Visual regression testing + CSP validation         | Sprint 4     |
| Phase 7 | Production deploy + CSP report monitoring          | Sprint 4     |

---

## 7. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Naive UI generates dynamic inline styles      | HIGH   | Nonce injection middleware, audit all Naive UI usage|
| Monaco Editor breaks without wasm-unsafe-eval | HIGH   | Iframe sandbox or keep directive with SRI           |
| Visual regression in 200+ components          | MEDIUM | Playwright screenshot comparison CI pipeline        |
| Third-party widgets inject inline styles      | LOW    | CSP report monitoring → patch incrementally         |

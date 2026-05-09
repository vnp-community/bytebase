# Change Request: API Service File Decomposition

| Field              | Value                                                         |
|--------------------|---------------------------------------------------------------|
| **CR ID**          | CR-AI-001                                                     |
| **Issue IDs**      | AI-BLOCKER-001, AI-BLOCKER-008                                |
| **Title**          | Decompose Oversized Service & Model Files for AI Readability  |
| **Category**       | Architecture (ARCH)                                           |
| **Priority**       | P0 — Critical                                                 |
| **Status**         | Draft                                                         |
| **Created**        | 2026-05-09                                                    |
| **Author**         | VNP AI Ops Team                                               |
| **PRD Refs**       | DCM-01 (Issue/Plan/Rollout), SEC-01 (IAM), SQL-01 (SQL Editor), ADM-08 (API Integration) |

---

## 1. Tổng quan

### 1.1 Mô tả
Phân tách các file dịch vụ API trong `backend/api/v1/` vượt quá 1000 LOC và file model `store/model/database.go` (1290 LOC) thành các module nhỏ hơn ≤500 LOC, tối ưu cho LLM context window và giảm hallucination risk trong quá trình AI-assisted development.

### 1.2 Bối cảnh
Hệ thống Bytebase hỗ trợ 22+ database engines (PRD §5) qua 30+ gRPC services (PRD §2). Các file dịch vụ hiện tại gộp nhiều domain logic, khiến AI agent phải load 12K+ tokens cho một file duy nhất — chiếm 30-60% context window và giảm độ chính xác code generation.

**Thống kê hiện tại:**

| File | LOC | Domains Mixed |
|------|-----|---------------|
| `auth_service.go` | 1930 | Session, MFA, OAuth2, SSO, rate limiting |
| `sql_service.go` | 1876 | Query execution, masking, access control, admin streams |
| `document_masking.go` | 1385 | Masking rules, deep nesting |
| `rollout_service.go` | 1278 | Rollout lifecycle, task state machines |
| `project_service.go` | 1275 | Project CRUD, IAM, webhooks |
| `plan_service.go` | 1259 | Plan creation, spec management |
| `database_service.go` | 1247 | Schema sync, metadata, catalog |
| `issue_service.go` | 1242 | Issue lifecycle, comments, approvals |
| `instance_service.go` | 1181 | Instance management, activation |
| `store/model/database.go` | 1290 | 6 metadata types + all CRUD operations |

### 1.3 Mục tiêu
- Mỗi file service ≤500 LOC (fits 4K-token LLM chunk)
- Giữ nguyên gRPC contract — handler signatures stay in main file
- Zero API regression — chỉ refactor internal organization
- Giảm merge conflict rate khi multiple AI agents edit đồng thời

---

## 2. Yêu cầu chức năng

### FR-001: Auth Service Decomposition
- **Mô tả**: Tách `auth_service.go` (1930 LOC) thành 4 domain files
- **Logic**:
  ```
  auth_service.go        → gRPC handlers (thin dispatchers) ≤300 LOC
  auth_session.go        → Session creation/validation/refresh
  auth_mfa.go            → MFA enrollment, TOTP verification
  auth_oauth.go          → OAuth2/SSO flows (Google, GitHub, OIDC/SAML)
  auth_ratelimit.go      → Rate limiting logic
  ```
- **PRD Alignment**: SEC-01 (IAM), SEC-05 (SSO), SEC-11 (Enterprise SSO), SEC-12 (2FA)
- **Acceptance Criteria**:
  - AC-1: `auth_service.go` chỉ chứa gRPC method signatures + dispatch calls
  - AC-2: Mỗi file extracted ≤500 LOC
  - AC-3: Tất cả existing tests pass (`go test ./backend/api/v1/...`)
  - AC-4: `go build` thành công cho cả 3 build profiles (ultimate, enterprise_core, minidemo)

### FR-002: SQL Service Decomposition
- **Mô tả**: Tách `sql_service.go` (1876 LOC) thành 3 domain files
- **Logic**:
  ```
  sql_service.go         → gRPC handlers ≤300 LOC
  sql_execution.go       → Query execution, result formatting
  sql_admin_execute.go   → AdminExecute bidi-stream, long-running queries
  ```
- **PRD Alignment**: SQL-01 (SQL Editor), SQL-02 (Admin Mode), SQL-12 (Batch Query)
- **Acceptance Criteria**:
  - AC-1: AdminExecute streaming flow hoạt động chính xác
  - AC-2: Data masking integration không bị break (SEC-15)

### FR-003: Rollout/Plan/Issue Service Decomposition
- **Mô tả**: Tách 3 services liên quan DCM workflow
- **Logic**:
  ```
  rollout_service.go     → gRPC handlers ≤400 LOC
  rollout_task_runner.go → Task state machine logic

  plan_service.go        → gRPC handlers ≤400 LOC
  plan_spec.go           → Spec creation/validation logic

  issue_service.go       → gRPC handlers ≤400 LOC
  issue_lifecycle.go     → Issue state transitions, approvals
  ```
- **PRD Alignment**: DCM-01 (Issue/Plan/Rollout), DCM-10 (Progressive Deployment), SEC-09 (Approval Workflow)

### FR-004: Store Model Decomposition
- **Mô tả**: Tách `store/model/database.go` (1290 LOC) thành 4 files
- **Logic**:
  ```
  database_metadata.go   → DatabaseMetadata struct + schema-level operations
  schema_metadata.go     → SchemaMetadata + table/view/sequence lookups
  table_metadata.go      → TableMetadata + column/index operations
  ddl_operations.go      → Create/Drop/Rename for all object types
  ```
- **Acceptance Criteria**:
  - AC-1: Mỗi file ≤400 LOC
  - AC-2: Tất cả `store/model/*_test.go` pass
  - AC-3: Case-sensitivity normalization logic không bị duplicate

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                 | File                                      | Thay đổi                                         |
|---------------------------|-------------------------------------------|---------------------------------------------------|
| Auth Service              | `backend/api/v1/auth_service.go`          | Extract to 4 files, keep gRPC dispatch            |
| SQL Service               | `backend/api/v1/sql_service.go`           | Extract to 3 files                                |
| Rollout Service           | `backend/api/v1/rollout_service.go`       | Extract task logic to `rollout_task_runner.go`     |
| Plan Service              | `backend/api/v1/plan_service.go`          | Extract spec logic to `plan_spec.go`               |
| Issue Service             | `backend/api/v1/issue_service.go`         | Extract lifecycle to `issue_lifecycle.go`           |
| Project Service           | `backend/api/v1/project_service.go`       | Extract webhook/IAM to `project_iam.go`            |
| Database Service          | `backend/api/v1/database_service.go`      | Extract sync logic to `database_sync.go`           |
| Instance Service          | `backend/api/v1/instance_service.go`      | Extract activation to `instance_activation.go`     |
| Store Model               | `backend/store/model/database.go`         | Split into 4 domain files                          |

### 3.2 Decomposition Pattern
Mỗi service file áp dụng pattern:
```go
// auth_service.go — thin gRPC dispatcher (≤300 LOC)
func (s *AuthService) Login(ctx context.Context, req *connect.Request[...]) (*connect.Response[...], error) {
    return s.handleLogin(ctx, req.Msg)  // dispatches to auth_session.go
}

// auth_session.go — domain implementation
func (s *AuthService) handleLogin(ctx context.Context, req *v1pb.LoginRequest) (*connect.Response[...], error) {
    // actual business logic here
}
```

### 3.3 Không có Database Changes
### 3.4 Không có API Contract Changes — gRPC proto files unchanged

---

## 4. Phụ thuộc

| Dependency           | Mô tả                                                      |
|----------------------|--------------------------------------------------------------|
| Go package system    | All extracted files stay in same package `v1` / `model`      |
| Build tags           | Verify compilation across all 3 build profiles               |
| Existing tests       | Must pass without modification                               |

---

## 5. Test Cases

| Test ID | Mô tả                                                          | Expected Result                  |
|---------|------------------------------------------------------------------|----------------------------------|
| TC-001  | `go build ./backend/...` với tất cả build tags                  | Zero compilation errors          |
| TC-002  | `go test ./backend/api/v1/...` — existing tests                | All pass                         |
| TC-003  | `go test ./backend/store/model/...` — model tests              | All pass                         |
| TC-004  | `go vet ./backend/api/v1/...` — no vet issues                  | Clean                            |
| TC-005  | Verify `auth_service.go` ≤300 LOC after extraction             | Max 300 lines                    |
| TC-006  | Verify no file in `api/v1/` exceeds 500 LOC (excluding tests)  | All ≤500 LOC                     |
| TC-007  | gRPC endpoint smoke test — Login, Query, CreatePlan             | Functional                       |
| TC-008  | AdminExecute bidi-stream still operational                      | Stream works end-to-end          |

---

## 6. Rollout Plan

| Phase   | Mô tả                                                | Timeline  |
|---------|--------------------------------------------------------|-----------|
| Phase 1 | Extract `auth_service.go` (highest priority, 1930 LOC) | Sprint 1  |
| Phase 2 | Extract `sql_service.go` + `document_masking.go`       | Sprint 1  |
| Phase 3 | Extract rollout/plan/issue services                    | Sprint 2  |
| Phase 4 | Extract project/database/instance services             | Sprint 2  |
| Phase 5 | Split `store/model/database.go`                        | Sprint 2  |
| Phase 6 | Full regression test suite                              | Sprint 3  |

---

## 7. Risks & Mitigations

| Risk                                        | Impact | Mitigation                                          |
|---------------------------------------------|--------|------------------------------------------------------|
| Import cycle sau khi split files            | HIGH   | Giữ tất cả files trong cùng package                 |
| Method receiver confusion                   | MEDIUM | Giữ cùng struct `*AuthService` trên tất cả files    |
| Merge conflicts với ongoing development     | MEDIUM | Thực hiện trong feature branch, squash merge         |
| Missing private functions/variables          | LOW    | Go cho phép access trong cùng package                |

---

## 8. Success Metrics

| Metric                          | Before  | Target   |
|---------------------------------|---------|----------|
| Max API service file LOC        | 1930    | ≤500     |
| Max store model file LOC        | 1290    | ≤400     |
| LLM context usage per file      | ~12K tokens | ~3K tokens |
| Total files in `api/v1/`        | ~60     | ~75      |

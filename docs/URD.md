# User Requirements Document (URD)
# Bytebase — Database CI/CD for DevOps Teams

| Metadata       | Value                                         |
|----------------|-----------------------------------------------|
| Product Name   | Bytebase                                      |
| Document Date  | 2026-05-08                                    |
| Status         | Synthesized from source code analysis         |

---

## 1. Giới thiệu

### 1.1 Mục đích tài liệu
Tài liệu này mô tả yêu cầu người dùng (User Requirements) cho nền tảng Bytebase — công cụ Database CI/CD cho DevOps teams. Nội dung được tổng hợp từ phân tích mã nguồn, cấu trúc tính năng, pricing plan, và enterprise feature gates của hệ thống.

### 1.2 Phạm vi
Tài liệu bao phủ tất cả các nhóm người dùng chính và yêu cầu của họ khi tương tác với Bytebase, bao gồm:
- Quản lý vòng đời schema database
- Phát triển và thực thi SQL
- Bảo mật và tuân thủ
- Vận hành và giám sát
- Tích hợp với hệ sinh thái DevOps

---

## 2. Nhóm người dùng và Personas

### 2.1 Database Administrator (DBA)

**Hồ sơ**: Chuyên gia quản trị cơ sở dữ liệu, chịu trách nhiệm đảm bảo tính sẵn sàng, hiệu năng, và bảo mật cho tất cả database instances.

**Mục tiêu**:
- Quản lý tập trung tất cả database instances từ một giao diện duy nhất
- Kiểm soát và phê duyệt tất cả thay đổi schema trước khi deploy lên production
- Giám sát và audit mọi hoạt động truy cập database
- Thiết lập chính sách bảo mật (masking, access control) đồng nhất

**Thách thức hiện tại**:
- Quản lý phân tán nhiều database engines (PostgreSQL, MySQL, MongoDB, v.v.)
- Thiếu công cụ review tập trung cho schema changes
- Khó truy vết ai đã thay đổi gì, khi nào

---

### 2.2 Application Developer

**Hồ sơ**: Lập trình viên phát triển ứng dụng, cần thay đổi database schema và query dữ liệu thường xuyên.

**Mục tiêu**:
- Tạo và quản lý schema migrations dễ dàng, tích hợp vào CI/CD
- Viết và kiểm thử SQL trực tiếp trên web
- Nhận phản hồi nhanh về chất lượng SQL (lint, review) trước khi deploy
- Cộng tác với DBA qua luồng phê duyệt trực quan

**Thách thức hiện tại**:
- Phải viết migration scripts thủ công
- Không có IDE SQL trên web để nhanh chóng debug
- CI/CD pipeline không bao phủ database changes

---

### 2.3 Platform Engineer

**Hồ sơ**: Kỹ sư hạ tầng/nền tảng, chịu trách nhiệm tự động hóa và chuẩn hóa infrastructure.

**Mục tiêu**:
- Quản lý Bytebase resources qua Infrastructure as Code (Terraform)
- Tích hợp API vào CI/CD pipeline (GitHub Actions, GitLab CI)
- Triển khai Bytebase trên Kubernetes với HA
- Tích hợp với AI agents qua MCP (Model Context Protocol)

**Thách thức hiện tại**:
- Database provisioning và configuration thủ công
- Thiếu API cho automation
- Khó scale khi số lượng environments và databases tăng

---

### 2.4 Security & Compliance Officer

**Hồ sơ**: Nhân viên bảo mật và tuân thủ, đảm bảo tổ chức tuân thủ các quy định về dữ liệu.

**Mục tiêu**:
- Kiểm soát ai được truy cập dữ liệu gì (column-level)
- Che dấu dữ liệu nhạy cảm (PII, financial data) cho các môi trường non-production
- Duy trì audit trail đầy đủ cho mọi hoạt động
- Thiết lập chính sách mật khẩu và xác thực đa yếu tố

**Thách thức hiện tại**:
- Dữ liệu production bị expose trong môi trường dev/staging
- Không có hệ thống audit tập trung cho database activities
- Quản lý quyền truy cập phân tán, khó kiểm soát

---

### 2.5 Team Lead / Project Manager

**Hồ sơ**: Quản lý dự án, cần giám sát tiến độ thay đổi database và phê duyệt các thay đổi quan trọng.

**Mục tiêu**:
- Theo dõi trạng thái tất cả database change requests (Issues)
- Thiết lập luồng phê duyệt phù hợp với mức độ rủi ro
- Nhận thông báo kịp thời qua IM (Slack, Teams, v.v.)
- Dashboard tổng quan về hoạt động database

**Thách thức hiện tại**:
- Thiếu visibility vào tiến độ schema changes
- Luồng phê duyệt thủ công qua email/chat
- Không có cách phân loại mức độ rủi ro tự động

---

## 3. Yêu cầu chức năng theo nhóm người dùng

### 3.1 Yêu cầu của DBA

| ID       | Yêu cầu                                                  | Ưu tiên  | Nguồn code                                      |
|----------|----------------------------------------------------------|----------|--------------------------------------------------|
| UR-D01   | Quản lý tập trung instances (add, edit, remove, sync)    | Critical | `backend/api/v1/instance_service.go`             |
| UR-D02   | Tự động phát hiện schema drift giữa environments         | High     | `backend/runner/schemasync/`                     |
| UR-D03   | Phê duyệt/từ chối schema changes qua giao diện web      | Critical | `backend/runner/approval/`                        |
| UR-D04   | Thiết lập SQL review policies (200+ rules)               | High     | `backend/plugin/advisor/`                         |
| UR-D05   | Xem audit log đầy đủ hoạt động database                  | High     | `backend/api/v1/audit_log_service.go`            |
| UR-D06   | Quản lý environment hierarchy (dev→staging→prod)          | Critical | `backend/store/environment.go`                   |
| UR-D07   | Thiết lập data masking rules theo column                  | High     | `backend/api/v1/document_masking.go`             |
| UR-D08   | Backup tự động trước khi thay đổi data                   | High     | `backend/runner/taskrun/`                         |
| UR-D09   | Rollback one-click khi có sự cố                          | Critical | DCM-08 feature                                   |
| UR-D10   | Quản lý replica heartbeat và health                       | Medium   | `backend/runner/heartbeat/`                       |
| UR-D11   | Clean up dữ liệu cũ tự động                              | Medium   | `backend/runner/cleaner/`                         |
| UR-D12   | Admin mode cho SQL Editor (direct execute)                | High     | `backend/api/v1/sql_service.go`                  |

### 3.2 Yêu cầu của Developer

| ID       | Yêu cầu                                                  | Ưu tiên  | Nguồn code                                      |
|----------|----------------------------------------------------------|----------|--------------------------------------------------|
| UR-V01   | Tạo schema migration từ web UI hoặc GitOps               | Critical | `backend/api/v1/plan_service.go`                 |
| UR-V02   | SQL Editor trên web với auto-complete, syntax highlight   | Critical | `frontend/src/` (Monaco Editor)                  |
| UR-V03   | SQL review tự động khi submit (lint + best practice)      | High     | `backend/api/v1/release_service_check.go`        |
| UR-V04   | Xem schema diagram trực quan                              | Medium   | `frontend/src/components/`                       |
| UR-V05   | Chuyển NL→SQL bằng AI                                     | Medium   | `backend/api/v1/sql_service_ai.go`               |
| UR-V06   | Xem query execution plan (EXPLAIN)                        | Medium   | `frontend/explain-visualizer.html`               |
| UR-V07   | Lưu và chia sẻ SQL scripts (Worksheet)                    | High     | `backend/api/v1/worksheet_service.go`            |
| UR-V08   | Export kết quả query ra CSV/Excel/JSON                    | Medium   | `backend/component/export/`                      |
| UR-V09   | Batch changes trên nhiều database cùng lúc                | High     | DCM-09 feature                                   |
| UR-V10   | Xem changelog chi tiết cho mỗi database                  | Medium   | `backend/api/v1/database_service_changelog.go`   |
| UR-V11   | Declarative migration (define desired state)              | High     | DCM-03 feature                                   |
| UR-V12   | Scheduled deployment (đặt lịch deploy)                    | Medium   | DCM-11 feature                                   |

### 3.3 Yêu cầu của Platform Engineer

| ID       | Yêu cầu                                                  | Ưu tiên  | Nguồn code                                      |
|----------|----------------------------------------------------------|----------|--------------------------------------------------|
| UR-P01   | Full REST API cho tất cả operations                       | Critical | `proto/v1/v1/*.proto` (36 service definitions)    |
| UR-P02   | Terraform Provider cho resource provisioning              | High     | ADM-03 feature                                   |
| UR-P03   | gRPC API cho high-performance integration                 | High     | ConnectRPC services                              |
| UR-P04   | Webhook notifications cho pipeline integration            | High     | `backend/component/webhook/`                     |
| UR-P05   | Service Account cho machine-to-machine auth               | High     | `backend/api/v1/service_account_service.go`      |
| UR-P06   | Workload Identity (OIDC) cho CI/CD                        | High     | `backend/api/v1/workload_identity_service.go`    |
| UR-P07   | MCP Server cho AI agent integration                       | Medium   | `backend/api/mcp/`                               |
| UR-P08   | Helm chart cho Kubernetes deployment                      | High     | `helm-charts/`                                   |
| UR-P09   | Docker image cho quick deployment                         | Critical | `deployment/`                                    |
| UR-P10   | LSP Server cho IDE integration                            | Medium   | `backend/api/lsp/`                               |
| UR-P11   | OAuth2 Provider cho downstream apps                       | Medium   | `backend/api/oauth2/`                            |

### 3.4 Yêu cầu của Security & Compliance

| ID       | Yêu cầu                                                  | Ưu tiên  | Nguồn code                                      |
|----------|----------------------------------------------------------|----------|--------------------------------------------------|
| UR-S01   | RBAC với project/workspace-level permissions               | Critical | `backend/component/iam/`                         |
| UR-S02   | Column-level data masking cho sensitive data               | Critical | `backend/api/v1/masking_evaluator.go`            |
| UR-S03   | Audit log đầy đủ cho mọi API call                         | Critical | `backend/api/v1/audit.go`                        |
| UR-S04   | SSO integration (OIDC, SAML, LDAP)                        | High     | `backend/plugin/idp/`                            |
| UR-S05   | Two-Factor Authentication (TOTP)                           | High     | `backend/api/v1/auth_service.go` + pquerna/otp   |
| UR-S06   | Data classification (PII, sensitive)                       | High     | SEC-16 feature                                   |
| UR-S07   | Password policy enforcement                                | Medium   | SEC-13 feature                                   |
| UR-S08   | SCIM 2.0 / Directory Sync                                 | Medium   | `backend/api/directory-sync/`                    |
| UR-S09   | External secret management (Vault/AWS/GCP)                 | High     | `backend/component/secret/`                      |
| UR-S10   | SSL/TLS cho tất cả database connections                    | Critical | `backend/plugin/db/*/` (per driver)              |
| UR-S11   | JIT access grants (time-limited)                           | Medium   | `backend/api/v1/access_grant_service.go`         |
| UR-S12   | Risk assessment tự động cho schema changes                 | High     | SEC-08 feature                                   |
| UR-S13   | Custom approval workflow theo risk level                   | High     | SEC-09 feature                                   |
| UR-S14   | Email domain restriction cho user registration             | Medium   | SEC-21 area                                      |
| UR-S15   | Token duration control                                     | Medium   | SEC-21 area                                      |

### 3.5 Yêu cầu của Team Lead / Manager

| ID       | Yêu cầu                                                  | Ưu tiên  | Nguồn code                                      |
|----------|----------------------------------------------------------|----------|--------------------------------------------------|
| UR-M01   | Dashboard tổng quan issues/plans/rollouts                  | High     | `frontend/src/views/`                            |
| UR-M02   | Luồng phê duyệt cho database changes                     | Critical | `backend/runner/approval/`                        |
| UR-M03   | Thông báo IM (Slack, DingTalk, Feishu, Teams)             | High     | `backend/plugin/webhook/`                        |
| UR-M04   | Project-based organization                                 | Critical | `backend/api/v1/project_service.go`              |
| UR-M05   | Quản lý roles và permissions                               | High     | `backend/api/v1/role_service.go`                 |
| UR-M06   | Xem review config và SQL policy                           | High     | `backend/api/v1/review_config_service.go`        |
| UR-M07   | Database group management                                  | Medium   | `backend/api/v1/database_group_service.go`       |
| UR-M08   | Custom announcement cho workspace                          | Low      | ADM-06 feature area                              |

---

## 4. Yêu cầu phi chức năng

### 4.1 Hiệu năng

| ID      | Yêu cầu                                                     | Target                      |
|---------|--------------------------------------------------------------|------------------------------|
| NF-P01  | API response time cho CRUD operations                        | p99 < 500ms                 |
| NF-P02  | SQL query execution timeout configurable                     | Configurable per-query      |
| NF-P03  | Schema sync cho large databases (>10,000 tables)             | < 5 minutes                 |
| NF-P04  | Concurrent schema changes across databases                   | ≥ 50 concurrent tasks       |
| NF-P05  | REST API max response size                                   | 100MB (configurable)        |

### 4.2 Khả năng mở rộng

| ID      | Yêu cầu                                                     | Thực hiện                    |
|---------|--------------------------------------------------------------|------------------------------|
| NF-S01  | Horizontal scaling (HA mode)                                 | External PostgreSQL + multiple replicas |
| NF-S02  | Plugin-based database engine support                          | Driver interface pattern     |
| NF-S03  | Extensible SQL review rules                                  | Advisor plugin per engine    |
| NF-S04  | Multi-workspace isolation                                     | Workspace-level data partitioning |

### 4.3 Bảo mật

| ID      | Yêu cầu                                                     | Thực hiện                    |
|---------|--------------------------------------------------------------|------------------------------|
| NF-SE01 | Encryption at rest cho sensitive data                         | Database-level encryption    |
| NF-SE02 | Encryption in transit cho tất cả connections                  | TLS/SSL, H2C                |
| NF-SE03 | Secrets không lưu plaintext                                   | External secret manager      |
| NF-SE04 | Session management an toàn                                    | JWT + refresh token + cookie |
| NF-SE05 | CSP (Content Security Policy) cho frontend                    | `vite-plugin-export-csp-hashes.ts` |

### 4.4 Khả năng sử dụng

| ID      | Yêu cầu                                                     | Thực hiện                    |
|---------|--------------------------------------------------------------|------------------------------|
| NF-U01  | Internationalization (i18n)                                   | Vue i18n + react-i18next     |
| NF-U02  | Responsive web UI                                             | Tailwind CSS v4              |
| NF-U03  | Schema diagram visualization                                  | ELK.js + D3                  |
| NF-U04  | Dark/Light mode                                               | CSS custom properties        |

### 4.5 Triển khai & Vận hành

| ID      | Yêu cầu                                                     | Thực hiện                    |
|---------|--------------------------------------------------------------|------------------------------|
| NF-O01  | Zero-dependency deployment (embedded PostgreSQL)              | Embedded pg for non-HA mode  |
| NF-O02  | Single binary deployment                                      | Frontend embedded in Go binary |
| NF-O03  | Health check endpoint                                         | Actuator service             |
| NF-O04  | Metrics collection (Prometheus)                               | `prometheus/client_golang`   |
| NF-O05  | Graceful shutdown                                             | 10-second shutdown period    |
| NF-O06  | Memory monitoring                                             | `backend/runner/monitor/`    |
| NF-O07  | Database self-migration                                       | `backend/migrator/`          |

---

## 5. User Journey Maps

### 5.1 DBA: Phê duyệt Schema Change

```
DBA nhận notification
  → Mở Issue trên Bytebase
    → Xem SQL diff và review comments
      → Chạy SQL review (200+ lint rules)
        → [Pass] Approve → Auto/Manual Rollout
        → [Fail] Reject → Developer nhận feedback
          → Developer sửa → Re-submit
```

### 5.2 Developer: Database-as-Code Workflow

```
Developer thay đổi SQL migration file trên Git
  → Push/PR trigger GitOps webhook
    → Bytebase tự động tạo Issue + Plan
      → SQL Review chạy tự động
        → DBA review & approve
          → Auto rollout theo environment order
            → Changelog được cập nhật tự động
```

### 5.3 Developer: SQL Editor Session

```
Developer mở SQL Editor
  → Chọn database instance & database
    → Viết SQL (auto-complete + syntax highlight)
      → Execute query
        → Data masking applied (nếu có policy)
          → Xem results → Export nếu cần
            → Query tự động lưu vào history
```

### 5.4 Platform Engineer: API Integration

```
CI/CD Pipeline trigger
  → Authenticate via Service Account / Workload Identity
    → Call Bytebase API (REST/gRPC)
      → Create Plan → Create Rollout
        → Wait for approval (webhook callback)
          → Check rollout status
            → Pipeline continues / fails
```

### 5.5 Security Officer: Data Access Audit

```
Compliance check period
  → Mở Audit Log dashboard
    → Filter by user/action/time range
      → Export audit records
        → Cross-reference với access grants
          → Identify unauthorized access patterns
            → Update masking/policy rules
```

---

## 6. Ràng buộc và Giới hạn

| Ràng buộc                                            | Chi tiết                                              |
|------------------------------------------------------|-------------------------------------------------------|
| PostgreSQL là bắt buộc cho metadata storage           | Embedded PG cho dev, external PG cho production HA    |
| HA mode yêu cầu external PostgreSQL                   | Không hỗ trợ HA với embedded database                 |
| Enterprise features yêu cầu license key               | Feature gates trong `backend/enterprise/license.go`   |
| Frontend đang trong giai đoạn chuyển đổi Vue → React  | Code mới phải viết bằng React                         |
| SQL Review rules theo từng engine                      | Không phải tất cả rules đều áp dụng cho mọi engine   |
| Max API response size: 100MB                           | Cấu hình cứng trong gRPC options                      |

---

## 7. Traceability Matrix

| User Requirement | Functional Requirement (PRD) | gRPC Service                    |
|------------------|------------------------------|---------------------------------|
| UR-D01           | Instance management          | InstanceService                 |
| UR-D03           | Approval workflow            | IssueService, PlanService       |
| UR-D04           | SQL Review                   | ReviewConfigService, ReleaseService |
| UR-V01           | Schema migration             | PlanService, RolloutService     |
| UR-V02           | SQL Editor                   | SQLService, SheetService        |
| UR-V05           | NL→SQL                      | AIService                       |
| UR-P01           | REST API                     | All 30+ services via gateway    |
| UR-S01           | RBAC                         | RoleService, WorkspaceService   |
| UR-S02           | Data Masking                 | DatabaseCatalogService, OrgPolicyService |
| UR-S03           | Audit Log                    | AuditLogService                 |
| UR-S04           | SSO                          | IdentityProviderService, AuthService |
| UR-M04           | Project management           | ProjectService                  |

---

> **Document generated**: 2026-05-08 — Based on source code analysis of Bytebase repository.

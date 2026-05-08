# Product Requirements Document (PRD)
# Bytebase — Database CI/CD for DevOps Teams

| Metadata       | Value                                                |
|----------------|------------------------------------------------------|
| Product Name   | Bytebase                                             |
| Version        | Latest (analyzed from source)                        |
| License        | BSL (Business Source License) + Enterprise           |
| Document Date  | 2026-05-08                                           |
| Status         | Active — CNCF Landscape project                      |
| Repository     | github.com/bytebase/bytebase                         |

---

## 1. Tổng quan sản phẩm

### 1.1 Tầm nhìn
Bytebase là nền tảng Database DevOps mã nguồn mở duy nhất được CNCF Landscape và Platform Engineering công nhận. Sản phẩm cung cấp workspace cộng tác dựa trên web, giúp DBA và Developer quản lý toàn bộ vòng đời schema của database ứng dụng — từ phát triển, review, deploy đến giám sát.

### 1.2 Vấn đề cần giải quyết
| # | Vấn đề                                      | Ảnh hưởng                                              |
|---|----------------------------------------------|--------------------------------------------------------|
| 1 | Thay đổi schema thủ công, thiếu kiểm soát   | Lỗi production, downtime, mất dữ liệu                 |
| 2 | Thiếu CI/CD cho database                     | Deploy chậm, không nhất quán giữa environments          |
| 3 | Thiếu audit trail cho database activities    | Không tuân thủ compliance, khó truy vết sự cố          |
| 4 | Data exposure do thiếu masking               | Rủi ro bảo mật, vi phạm quy định dữ liệu              |
| 5 | Quản lý phân tán khi nhiều database engines  | Phức tạp vận hành, tốn nhân lực                         |

### 1.3 Đối tượng người dùng

| Persona              | Mô tả                                               | Nhu cầu chính                                           |
|----------------------|------------------------------------------------------|----------------------------------------------------------|
| **Database Admin**   | Quản trị viên CSDL                                   | Centralized management, policy enforcement, audit        |
| **Developer**        | Lập trình viên ứng dụng                              | Schema versioning, CI/CD automation, SQL editor          |
| **Platform Engineer**| Kỹ sư nền tảng                                      | IaC (Terraform), API integration, multi-environment     |
| **Security/Compliance** | Đội bảo mật và tuân thủ                           | Data masking, access control, audit logging              |
| **Team Lead/Manager**| Quản lý dự án                                        | Review workflow, approval policy, dashboard               |

---

## 2. Mô hình kiến trúc tổng quan

```
┌──────────────────────────────────────────────────────────────────┐
│                        Frontend (Vue 3 + React 19)               │
│   ├── Web SQL Editor (Monaco)                                    │
│   ├── Schema Diagram / Schema Editor                             │
│   ├── Issue/Plan/Rollout Management UI                           │
│   ├── Project/Environment/Instance Management                    │
│   └── Settings & Administration                                  │
├──────────────────────────────────────────────────────────────────┤
│              API Layer (ConnectRPC + gRPC-Gateway REST)           │
│   ├── 30+ gRPC Services (Protobuf v1)                            │
│   ├── Auth Interceptor (JWT + Cookie + OAuth2)                   │
│   ├── ACL Interceptor (IAM + RBAC)                               │
│   ├── Audit Interceptor                                          │
│   └── Validation Interceptor (protovalidate)                     │
├──────────────────────────────────────────────────────────────────┤
│                    Core Backend (Go 1.26)                         │
│   ├── Store Layer (PostgreSQL via pgx/v5)                        │
│   ├── Component Layer (IAM, Webhook, DBFactory, Masker, ...)     │
│   ├── Runner Layer (TaskRun, PlanCheck, SchemaSync, Approval, ..)│
│   ├── Plugin Layer (22+ DB Drivers, SQL Advisor, Parser, IDP)    │
│   ├── Enterprise Layer (License, Feature Gates)                  │
│   └── Migrator (Schema Migration for Bytebase itself)            │
├──────────────────────────────────────────────────────────────────┤
│              Protocol Layer (MCP, LSP, OAuth2, SCIM, Stripe)     │
├──────────────────────────────────────────────────────────────────┤
│                    Database Engines (22+)                         │
│   PostgreSQL, MySQL, TiDB, ClickHouse, MongoDB, Redis,           │
│   Snowflake, Oracle, SQL Server, Spanner, BigQuery, Cassandra,   │
│   CosmosDB, DynamoDB, Elasticsearch, Hive, Databricks, Trino,   │
│   StarRocks, SQLite, MariaDB/CockroachDB, Redshift               │
└──────────────────────────────────────────────────────────────────┘
```

---

## 3. Phân nhóm tính năng

### 3.1 Database Change Management (DCM)

| ID       | Tính năng                              | Plan       | Mô tả                                                                  |
|----------|----------------------------------------|------------|-------------------------------------------------------------------------|
| DCM-01   | Database Change (Issue/Plan/Rollout)   | FREE       | Workflow thay đổi schema: tạo Issue → Plan → Rollout → Task execution  |
| DCM-02   | Git-based Schema Version Control       | FREE       | Tích hợp GitHub/GitLab cho database-as-code                            |
| DCM-03   | Declarative Schema Migration           | FREE       | Định nghĩa trạng thái mong muốn, Bytebase tự tính diff                |
| DCM-04   | Compare & Sync Schema                  | FREE       | So sánh và đồng bộ schema giữa các database                           |
| DCM-05   | Online Schema Change (gh-ost)          | FREE       | Thay đổi schema không downtime cho MySQL (via gh-ost)                  |
| DCM-06   | Pre-deployment SQL Review              | FREE       | 200+ lint rules kiểm tra SQL trước khi deploy                         |
| DCM-07   | Automatic Backup Before Data Changes   | FREE       | Tự động backup trước khi thay đổi dữ liệu                             |
| DCM-08   | One-click Data Rollback                | FREE       | Rollback dữ liệu bằng một click                                       |
| DCM-09   | Multi-database Batch Changes           | FREE       | Apply thay đổi hàng loạt trên nhiều database                          |
| DCM-10   | Progressive Environment Deployment     | FREE       | Deploy tuần tự qua các environment (dev → staging → prod)             |
| DCM-11   | Scheduled Rollout Time                 | FREE       | Lập lịch thời điểm deploy                                              |
| DCM-12   | Database Changelog                     | FREE       | Lịch sử đầy đủ các thay đổi database                                  |
| DCM-13   | Rollout Policy                         | FREE       | Chính sách kiểm soát rollout                                           |

### 3.2 SQL Editor & Development

| ID       | Tính năng                              | Plan       | Mô tả                                                                  |
|----------|----------------------------------------|------------|-------------------------------------------------------------------------|
| SQL-01   | Web-based SQL Editor                   | FREE       | IDE trên web với Monaco Editor                                          |
| SQL-02   | Admin Mode                             | FREE       | Chế độ quản trị cho DBA                                                 |
| SQL-03   | Natural Language to SQL (AI)           | FREE       | Chuyển ngôn ngữ tự nhiên thành SQL                                     |
| SQL-04   | AI Query Explanation                   | FREE       | Giải thích query bằng AI                                               |
| SQL-05   | AI Query Suggestions                   | FREE       | Đề xuất cải thiện query bằng AI                                         |
| SQL-06   | Auto-complete                          | FREE       | Hoàn thành tự động (table, column, keyword)                            |
| SQL-07   | Schema Diagram                         | FREE       | Biểu đồ trực quan hóa schema                                          |
| SQL-08   | Schema Editor                          | FREE       | Chỉnh sửa schema qua giao diện kéo-thả                                |
| SQL-09   | Data Export                            | FREE       | Xuất dữ liệu query ra các định dạng                                   |
| SQL-10   | Query History                          | FREE       | Lưu lịch sử query                                                      |
| SQL-11   | Saved & Shared SQL Scripts             | FREE       | Lưu và chia sẻ script SQL                                              |
| SQL-12   | Batch Query                            | TEAM       | Query đồng thời trên nhiều database                                    |
| SQL-13   | Read-only Connection                   | TEAM       | Kết nối read-only tách biệt cho SQL Editor                             |
| SQL-14   | Query Policy                           | TEAM       | Chính sách kiểm soát query                                             |
| SQL-15   | Restrict Copying Data                  | ENTERPRISE | Ngăn sao chép dữ liệu từ SQL Editor                                   |

### 3.3 Security & Compliance

| ID       | Tính năng                              | Plan       | Mô tả                                                                  |
|----------|----------------------------------------|------------|-------------------------------------------------------------------------|
| SEC-01   | IAM (Identity & Access Management)     | FREE       | Quản lý người dùng và quyền cơ bản                                    |
| SEC-02   | Instance SSL Connection                | FREE       | Kết nối SSL tới database instances                                     |
| SEC-03   | SSH Tunnel                             | FREE       | Kết nối qua SSH tunnel                                                  |
| SEC-04   | IAM Authentication (Cloud)             | FREE       | Xác thực qua Cloud IAM (GCP, AWS)                                      |
| SEC-05   | Google & GitHub SSO                    | TEAM       | Đăng nhập qua Google/GitHub                                            |
| SEC-06   | User Groups                            | TEAM       | Quản lý nhóm người dùng                                                |
| SEC-07   | Audit Log (Limited)                    | TEAM       | Nhật ký kiểm toán (giới hạn)                                          |
| SEC-08   | Risk Assessment                        | ENTERPRISE | Đánh giá rủi ro cho database changes                                   |
| SEC-09   | Approval Workflow                      | ENTERPRISE | Luồng phê duyệt tùy chỉnh                                            |
| SEC-10   | Audit Log (Full)                       | ENTERPRISE | Nhật ký kiểm toán đầy đủ                                              |
| SEC-11   | Enterprise SSO (OIDC/SAML/LDAP)        | ENTERPRISE | SSO doanh nghiệp (OIDC, SAML, LDAP)                                   |
| SEC-12   | Two-Factor Authentication (2FA)        | ENTERPRISE | Xác thực hai yếu tố (TOTP)                                            |
| SEC-13   | Password Restrictions                  | ENTERPRISE | Chính sách mật khẩu nghiêm ngặt                                       |
| SEC-14   | Custom Roles                           | ENTERPRISE | Vai trò tùy chỉnh                                                      |
| SEC-15   | Data Masking                           | ENTERPRISE | Column-level data masking cho dữ liệu nhạy cảm                        |
| SEC-16   | Data Classification                    | ENTERPRISE | Phân loại dữ liệu (PII, sensitive, etc.)                              |
| SEC-17   | SCIM / Directory Sync                  | ENTERPRISE | Đồng bộ danh bạ qua SCIM 2.0                                          |
| SEC-18   | External Secret Manager                | ENTERPRISE | Tích hợp Vault, AWS SM, GCP SM                                         |
| SEC-19   | Workload Identity (OIDC Federation)    | ENTERPRISE | Xác thực không mật khẩu cho CI/CD                                      |
| SEC-20   | JIT (Just-In-Time) Access              | ENTERPRISE | Cấp quyền tạm thời                                                     |
| SEC-21   | Request Role Workflow                  | ENTERPRISE | Luồng yêu cầu cấp quyền                                              |

### 3.4 Administration & Integration

| ID       | Tính năng                              | Plan       | Mô tả                                                                  |
|----------|----------------------------------------|------------|-------------------------------------------------------------------------|
| ADM-01   | Environment Management                 | FREE       | Quản lý multi-environment (dev/staging/prod)                           |
| ADM-02   | IM Notifications                       | FREE       | Thông báo qua Slack, DingTalk, Feishu, Teams, v.v.                    |
| ADM-03   | Terraform Provider                     | FREE       | Infrastructure as Code cho Bytebase resources                          |
| ADM-04   | Database Groups                        | TEAM       | Nhóm database theo logical group                                       |
| ADM-05   | Environment Tiers                      | ENTERPRISE | Phân tầng environment (production tier)                                |
| ADM-06   | Custom Logo / Branding                 | ENTERPRISE | Tùy chỉnh logo và branding                                            |
| ADM-07   | Watermark                              | ENTERPRISE | Watermark cho SQL Editor                                               |
| ADM-08   | API Integration (REST + gRPC)          | ALL        | Full API access qua ConnectRPC + REST gateway                          |
| ADM-09   | MCP Server                             | ALL        | Model Context Protocol cho AI agent integration                        |
| ADM-10   | LSP Server                             | ALL        | Language Server Protocol cho SQL editing                               |
| ADM-11   | OAuth2 Provider                        | ALL        | OAuth2 authorization server                                            |
| ADM-12   | Stripe Billing Integration             | SaaS       | Thanh toán tự động qua Stripe                                          |

---

## 4. Pricing Tiers & Feature Gates

| Dimension                | FREE      | TEAM       | ENTERPRISE    |
|--------------------------|-----------|------------|---------------|
| **Maximum Instances**    | 10        | 10         | Unlimited     |
| **Maximum Seats**        | 20        | Unlimited  | Unlimited     |
| **SSO**                  | —         | Google/GitHub | Full (OIDC/SAML/LDAP) |
| **Data Masking**         | —         | —          | ✅             |
| **Approval Workflow**    | —         | —          | ✅             |
| **Audit Log**            | —         | Limited    | Full           |
| **Custom Roles**         | —         | —          | ✅             |
| **2FA**                  | —         | —          | ✅             |
| **SCIM/Directory Sync**  | —         | —          | ✅             |
| **Support**              | Community | Email      | Dedicated SLA |

---

## 5. Supported Database Engines

| Category        | Engines                                                               |
|-----------------|-----------------------------------------------------------------------|
| **Relational**  | PostgreSQL, MySQL, MariaDB, SQL Server, Oracle, SQLite, CockroachDB   |
| **NewSQL**      | TiDB, Spanner, StarRocks                                              |
| **Cloud DW**    | Snowflake, BigQuery, Redshift, Databricks, ClickHouse, Trino, Hive    |
| **NoSQL**       | MongoDB, Redis, CosmosDB, DynamoDB, Elasticsearch, Cassandra          |

Mỗi engine được triển khai dưới dạng **plugin driver** (`backend/plugin/db/<engine>/`) với interface chuẩn hóa:
- `Open()` / `Close()` / `Ping()`
- `Execute()` / `QueryConn()`
- `SyncInstance()` / `SyncDBSchema()`
- `Dump()` (schema export)

---

## 6. Deployment Options

| Mode                | Mô tả                                              |
|---------------------|-----------------------------------------------------|
| **Docker**          | Single container, embedded hoặc external PostgreSQL |
| **Kubernetes**      | Helm chart, HA mode với external PostgreSQL          |
| **Cloud (SaaS)**    | Managed bởi Bytebase Inc.                           |
| **Self-hosted**     | On-premise, build from source                       |

---

## 7. Technology Stack

### Backend
| Component       | Technology                                              |
|-----------------|---------------------------------------------------------|
| Language        | Go 1.26                                                 |
| HTTP Framework  | Echo v5                                                 |
| RPC Framework   | ConnectRPC + gRPC-Gateway v2                            |
| Serialization   | Protocol Buffers v3 (buf.build)                         |
| Database        | PostgreSQL (metadata store, via pgx/v5)                 |
| Authentication  | JWT (golang-jwt/v5), OAuth2, OIDC (go-jose/v4)         |
| Authorization   | CEL (Common Expression Language) based IAM               |
| Secret Mgmt     | HashiCorp Vault, AWS/GCP Secret Manager                 |
| Parser          | ANTLR v4 (custom SQL parsers per engine)                |
| Telemetry       | OpenTelemetry, Prometheus                               |

### Frontend
| Component       | Technology                                              |
|-----------------|---------------------------------------------------------|
| Framework       | Vue 3.5 (legacy) + React 19 (new code)                 |
| State Mgmt      | Pinia 3.x (Vue) + Zustand 5.x (React)                  |
| UI Library      | Naive UI 2.44 (Vue) + Base UI (React)                   |
| CSS             | Tailwind CSS v4                                          |
| Code Editor     | Monaco Editor (via VSCode API)                          |
| Build Tool      | Vite 7.3                                                |
| i18n            | vue-i18n + react-i18next                                 |
| Type System     | TypeScript 6.0                                          |

### Protocol & API
| Component       | Technology                                              |
|-----------------|---------------------------------------------------------|
| API Protocol    | ConnectRPC (HTTP/2 + HTTP/1.1)                          |
| REST Gateway    | gRPC-Gateway v2 (auto-generated from proto)             |
| Code Gen        | buf (protoc alternative)                                 |
| MCP             | Model Context Protocol (Go SDK)                         |
| LSP             | Language Server Protocol (JSON-RPC 2.0)                 |
| Webhook         | HTTP callbacks cho change notifications                  |

---

## 8. Roadmap Indicators (from codebase analysis)

| Signal                               | Insight                                               |
|--------------------------------------|-------------------------------------------------------|
| Vue → React migration in progress    | Frontend đang chuyển đổi sang React + Base UI          |
| MCP Server integration               | Sẵn sàng cho AI agent ecosystem                       |
| Workload Identity service            | OIDC federation cho CI/CD pipelines                    |
| Release Service + AI Lint            | AI-powered SQL review (release_service_ai_lint.go)     |
| Stripe integration                   | SaaS billing infrastructure                            |
| OAuth2 Provider                      | Bytebase-as-identity-provider                          |

---

## 9. Success Metrics

| Metric                           | Target                                           |
|----------------------------------|--------------------------------------------------|
| Schema change deployment time    | Giảm ≥80% so với thủ công                       |
| SQL review violation rate        | Phát hiện ≥95% lỗi trước deployment              |
| Audit compliance coverage        | 100% database activities được log                |
| Mean time to rollback            | < 5 phút cho data rollback                       |
| Multi-database consistency       | 100% schema sync across environments             |

---

> **Document generated**: 2026-05-08 — Based on source code analysis of Bytebase repository.

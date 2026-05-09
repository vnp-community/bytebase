# Change Request: External Secret Manager

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-015                                               |
| **Feature ID**     | SEC-18                                                   |
| **Title**          | External Secret Manager Integration                      |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Tích hợp với **External Secret Managers** (HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager) để quản lý database credentials và sensitive configuration thay vì lưu trực tiếp trong Bytebase database.

### 1.2 Mục tiêu
- Centralized secret management
- Automatic secret rotation support
- Eliminate plaintext credentials in Bytebase store
- Compliance với security best practices

---

## 2. Yêu cầu chức năng

### FR-001: Supported Secret Managers
- **Mô tả**: Tích hợp với các external secret managers.

| Provider                     | Auth Methods                              | Features              |
|------------------------------|-------------------------------------------|-----------------------|
| **HashiCorp Vault**          | Token, AppRole, Kubernetes, AWS IAM       | KV v2, Transit, PKI   |
| **AWS Secrets Manager**      | IAM Role, Access Key                      | Auto-rotation, versioning |
| **GCP Secret Manager**       | Service Account, Workload Identity        | Versioning, IAM       |

- **Acceptance Criteria**:
  - AC-1: Configuration UI cho mỗi provider type
  - AC-2: Connection test button
  - AC-3: Multiple providers simultaneously (e.g., Vault for DB creds, AWS SM for API keys)

### FR-002: Secret Reference System
- **Mô tả**: Database instance credentials tham chiếu tới external secrets.
- **Reference Format**:
  ```
  # Vault
  vault://secret/data/bytebase/prod-pg#password

  # AWS Secrets Manager
  aws-sm://arn:aws:secretsmanager:us-east-1:123:secret:prod-pg#password

  # GCP Secret Manager
  gcp-sm://projects/my-project/secrets/prod-pg/versions/latest
  ```
- **Acceptance Criteria**:
  - AC-1: Instance config supports secret references thay vì plaintext
  - AC-2: Secrets resolved at runtime (not cached long-term)
  - AC-3: Cache with configurable TTL (default: 5 min)
  - AC-4: Fallback to local credentials if external manager unavailable

### FR-003: Secret Rotation Support
- **Mô tả**: Hỗ trợ automatic secret rotation.
- **Acceptance Criteria**:
  - AC-1: Detect rotated secrets (Vault lease, AWS rotation)
  - AC-2: Refresh database connections when secrets rotate
  - AC-3: No downtime during rotation
  - AC-4: Rotation event logged to audit

### FR-004: Migration from Local to External
- **Mô tả**: Tool để migrate existing local credentials to external secret manager.
- **Acceptance Criteria**:
  - AC-1: One-click migration wizard
  - AC-2: Verify external secret accessibility trước migration
  - AC-3: Rollback capability if migration fails
  - AC-4: Local credentials wiped after successful migration

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                         | Thay đổi                                          |
|------------------------------|--------------------------------------|----------------------------------------------------|
| Secret Manager Plugin        | `backend/plugin/secret/`             | Vault, AWS SM, GCP SM clients                     |
| Instance Service             | `backend/api/v1/instance_service.go` | Resolve secret references                          |
| DB Factory                   | `backend/component/dbfactory/`       | Use resolved credentials for connections           |
| Feature Gate                 | `enterprise/feature.go`              | Define `FeatureExternalSecretManager`              |
| Settings Service             | `backend/api/v1/setting_service.go`  | Secret manager configuration                       |

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Configure Vault connection                              | Connection test success               |
| TC-002     | Create instance with Vault secret ref                   | Credentials resolved from Vault       |
| TC-003     | Secret rotated in Vault                                 | Connection refreshed automatically    |
| TC-004     | External manager unavailable                            | Graceful degradation, cached creds    |
| TC-005     | Migrate local creds to AWS SM                           | Credentials migrated, local wiped     |
| TC-006     | Non-ENTERPRISE: external SM hidden                      | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Vault integration                    | Sprint 1       |
| Phase 2 | AWS Secrets Manager                  | Sprint 2       |
| Phase 3 | GCP Secret Manager                   | Sprint 2       |
| Phase 4 | Migration wizard                     | Sprint 3       |
| Phase 5 | Rotation support                     | Sprint 3       |

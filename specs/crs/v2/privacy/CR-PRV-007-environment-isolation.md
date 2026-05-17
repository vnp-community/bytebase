# Change Request: Environment Data Isolation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-007                                               |
| **Feature ID**     | ADM-05, UR-D06 (extends), NF-SE03                       |
| **Title**          | Environment Data Isolation                               |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-019 (Env Tiers), CR-PRV-001, CR-PRV-002          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **data isolation framework** đảm bảo dữ liệu production (chứa PII thật) không bị sao chép nguyên trạng sang các môi trường dev/staging. Khi clone hoặc sync data từ production, hệ thống tự động áp dụng anonymization/masking.

### 1.2 Bối cảnh từ PRD/URD
- **PRD ADM-05**: Environment Tiers — phân tầng environment nhưng chưa có data isolation policy
- **URD UR-D06**: Quản lý environment hierarchy — chưa enforce data privacy across tiers
- **URD Section 2.4**: Security Officer challenge: "Dữ liệu production bị expose trong dev/staging"
- **PRD DCM-10**: Progressive deployment — data flows across environments cần privacy controls

### 1.3 Mục tiêu
- Automatic data anonymization khi clone prod → non-prod
- Environment-based data access policies
- Cross-environment data flow monitoring
- Production data never exists in lower environments without anonymization

---

## 2. Yêu cầu chức năng

### FR-001: Data Isolation Policy
- **Mô tả**: Policy engine ngăn chặn production data leak sang lower environments.
- **Policy Rules**:

| Rule                              | Mô tả                                          | Enforcement    |
|-----------------------------------|--------------------------------------------------|----------------|
| **No Raw Clone**                  | Không clone prod data mà không anonymization    | Hard block     |
| **Auto-Anonymize**                | Tự động anonymize khi sync prod → staging/dev   | Auto-apply     |
| **Classification Gate**           | Block sync cho L3/L4 classified columns         | Configurable   |
| **Volume Limit**                  | Limit data volume sync to non-prod              | Configurable   |
| **Approval Required**             | Sync prod data cần approval                     | Configurable   |

- **Acceptance Criteria**:
  - AC-1: Policy per environment tier (production, staging, development)
  - AC-2: Hard block for raw production data clone (non-overridable for L4)
  - AC-3: Approval workflow cho exceptions
  - AC-4: Policy violations generate security alerts

### FR-002: Cross-Environment Sync with Privacy
- **Mô tả**: Khi sync/clone data across environments, pipeline privacy tự động chạy.
- **Pipeline**:
  ```
  Sync Request (prod → staging) → Validate Isolation Policy
    → Scan target columns for PII (CR-PRV-001)
      → Apply anonymization rules (CR-PRV-002)
        → Execute sync with transformed data
          → Verify no raw PII in destination
            → Audit log
  ```
- **Acceptance Criteria**:
  - AC-1: Zero production PII in dev/staging (verification scan post-sync)
  - AC-2: Sync performance overhead ≤ 20%
  - AC-3: Dry-run mode to preview data transformation

### FR-003: Data Flow Monitoring
- **Mô tả**: Monitor và alert khi data flows between environments.
- **Features**:
  - Real-time data flow visualization (prod ↔ staging ↔ dev)
  - Alert on unauthorized cross-environment data access
  - Data lineage tracking across environments
- **Acceptance Criteria**:
  - AC-1: Dashboard showing active data flows
  - AC-2: Alert within 5 minutes of policy violation
  - AC-3: Weekly data isolation compliance report

---

## 3. Yêu cầu kỹ thuật

| Component                 | File/Package                                | Thay đổi                          |
|----------------------------|---------------------------------------------|-----------------------------------|
| Isolation Policy Engine    | `backend/component/privacy/isolation.go`    | Environment data isolation rules  |
| Sync Privacy Pipeline      | `backend/runner/schemasync/privacy.go`      | Privacy pipeline for env sync     |
| Data Flow Monitor          | `backend/runner/monitor/data_flow.go`       | Cross-env data flow tracking      |
| Isolation Dashboard        | `frontend/src/views/DataIsolation.tsx`      | Isolation policy management UI    |
| Feature Gate               | `enterprise/feature.go`                     | `FeatureDataIsolation`            |

---

## 4. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Data isolation policy engine              | Sprint 1       |
| Phase 2 | Cross-environment sync pipeline           | Sprint 2       |
| Phase 3 | Post-sync verification                    | Sprint 3       |
| Phase 4 | Data flow monitoring + dashboard          | Sprint 3       |

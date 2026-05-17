# Change Request: Database Credential Rotation Automation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-008                                               |
| **Feature ID**     | SEC-18                                                   |
| **Title**          | Database Credential Rotation Automation                  |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Tự động hóa quy trình rotation credentials cho tất cả managed database instances. Bao gồm scheduled rotation, zero-downtime credential swap, integration với External Secret Manager, và emergency rotation khi phát hiện credential leak.

### 1.2 Bối cảnh
Bytebase quản lý credentials cho 22+ database engines (PRD Section 5). Credential rotation thủ công gây rủi ro bảo mật cao và downtime. External Secret Manager (SEC-18, CR-ENT-015) cung cấp foundation nhưng chưa có automation layer.

---

## 2. Yêu cầu chức năng

### FR-001: Scheduled Credential Rotation
- **Configuration**:
  ```yaml
  credential_rotation:
    schedule: "0 2 * * 0"        # Weekly Sunday 2 AM
    instances:
      - pattern: "prod-*"
        interval: 30d
      - pattern: "staging-*"
        interval: 90d
    pre_rotation_test: true       # Test new credential before swap
    notification_before: 7d       # Alert before scheduled rotation
  ```
- **Acceptance Criteria**:
  - AC-1: Per-instance rotation schedule
  - AC-2: Pattern-based batch rotation
  - AC-3: Pre-rotation connectivity test
  - AC-4: Zero-downtime credential swap (dual-credential window)
  - AC-5: Rollback to previous credential on failure
  - AC-6: Support all 22+ database engines

### FR-002: Emergency Rotation
- **Acceptance Criteria**:
  - AC-1: One-click emergency rotation for single instance
  - AC-2: Bulk emergency rotation for all instances
  - AC-3: Emergency rotation bypasses schedule
  - AC-4: Immediate notification to all admins
  - AC-5: Forced disconnect of existing connections using old credential

### FR-003: Vault Dynamic Secrets Integration
- **Acceptance Criteria**:
  - AC-1: HashiCorp Vault database secret engine integration
  - AC-2: Dynamic credentials with TTL per connection
  - AC-3: Lease renewal for long-running operations
  - AC-4: AWS RDS IAM authentication support
  - AC-5: GCP Cloud SQL IAM authentication support

### FR-004: Rotation Audit & Compliance
- **Acceptance Criteria**:
  - AC-1: Complete rotation history per instance
  - AC-2: Credential age dashboard (identify stale credentials)
  - AC-3: Compliance alert: credential older than policy threshold
  - AC-4: Report exportable for compliance audits

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Credential Rotation (new)    | `backend/runner/credential_rotation/`       | Scheduled rotation engine                   |
| Secret Component             | `backend/component/secret/`                 | Vault dynamic secrets, rotation API         |
| Instance Service             | `backend/api/v1/instance_service.go`        | Emergency rotation endpoint                 |
| DB Factory                   | `backend/component/dbfactory/`              | Dual-credential connection handling         |
| Rotation Dashboard           | `frontend/src/views/CredentialRotation.vue`  | Rotation status + credential age           |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Scheduled rotation executes                          | New credential active, old valid |
| TC-002  | Pre-rotation test fails                              | Rotation aborted, admin alerted  |
| TC-003  | Emergency rotation                                   | Immediate credential swap        |
| TC-004  | Vault dynamic secret requested                       | Short-lived credential issued    |
| TC-005  | Credential age exceeds threshold                     | Compliance alert triggered       |
| TC-006  | Rotation rollback on failure                         | Previous credential restored     |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Scheduled rotation engine            | Sprint 1       |
| Phase 2 | Emergency rotation                   | Sprint 2       |
| Phase 3 | Vault dynamic secrets                | Sprint 3       |
| Phase 4 | Audit dashboard + compliance         | Sprint 4       |

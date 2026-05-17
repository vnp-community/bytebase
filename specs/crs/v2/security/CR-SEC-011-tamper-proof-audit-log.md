# Change Request: Tamper-Proof Audit Log

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-011                                               |
| **Feature ID**     | SEC-07, SEC-10                                           |
| **Title**          | Tamper-Proof Audit Log                                   |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai tamper-proof audit log đảm bảo tính toàn vẹn (integrity) và không thể chối bỏ (non-repudiation) của audit records. Sử dụng hash chain (blockchain-inspired) và digital signatures để phát hiện bất kỳ sự can thiệp nào vào audit logs.

### 1.2 Bối cảnh
Bytebase Audit Log (SEC-07/SEC-10, CR-ENT-003) ghi nhận activities nhưng logs lưu trong PostgreSQL có thể bị admin database modify/delete. Compliance frameworks (SOC 2, ISO 27001, GDPR) yêu cầu immutable audit trail.

---

## 2. Yêu cầu chức năng

### FR-001: Hash Chain Integrity
- **Architecture**:
  ```
  Log Entry N:
    hash = SHA-256(previous_hash + entry_data + timestamp + sequence)
    signature = ECDSA_Sign(hash, server_private_key)
  ```
- **Acceptance Criteria**:
  - AC-1: Mỗi audit entry chứa hash of previous entry (chain)
  - AC-2: Hash chain verification — detect gaps/modifications
  - AC-3: Digital signature per entry using server key
  - AC-4: Sequence number monotonically increasing
  - AC-5: Periodic checkpoint hashes stored externally

### FR-002: Immutable Storage
- **Acceptance Criteria**:
  - AC-1: Database-level protection: audit table is append-only (no UPDATE/DELETE)
  - AC-2: Optional external immutable storage (AWS S3 Object Lock, Azure Immutable Blob)
  - AC-3: Dual-write: primary DB + external immutable store
  - AC-4: Retention policy with configurable period (min 1 year, max 7 years)
  - AC-5: Storage encryption at rest

### FR-003: Integrity Verification
- **Acceptance Criteria**:
  - AC-1: On-demand integrity check — verify full hash chain
  - AC-2: Scheduled integrity verification (daily/weekly)
  - AC-3: Tamper detection alerts to admin
  - AC-4: Verification report exportable for auditors
  - AC-5: Cross-reference verification between primary and external store

### FR-004: Non-Repudiation
- **Acceptance Criteria**:
  - AC-1: All entries digitally signed with server key
  - AC-2: Signature verification by external auditors
  - AC-3: Key management: signing key rotation with continuity
  - AC-4: Timestamping service integration (RFC 3161) — optional

### FR-005: Audit Log Protection
- **Acceptance Criteria**:
  - AC-1: Even workspace admin cannot delete audit logs
  - AC-2: Audit log access requires separate "audit_viewer" permission
  - AC-3: Accessing audit logs is itself audited (meta-audit)
  - AC-4: Bulk export requires multi-admin approval

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Audit Log Service            | `backend/api/v1/audit_log_service.go`       | Hash chain, digital signature               |
| Audit Store                  | `backend/store/audit_log.go`                | Append-only enforcement, external sync      |
| Integrity Checker (new)      | `backend/runner/audit_integrity/`           | Scheduled verification                      |
| External Storage (new)       | `backend/component/immutable_store/`        | S3 Object Lock / Azure Immutable Blob       |
| Audit Verification UI        | `frontend/src/views/AuditVerification.vue`  | Integrity check + report                    |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Direct DB UPDATE on audit table                      | Blocked by database trigger      |
| TC-002  | Hash chain verification after normal operations      | Chain valid, no tampering        |
| TC-003  | Tampered entry detected                              | Verification failure + alert     |
| TC-004  | External store sync                                  | Dual copies consistent           |
| TC-005  | Audit log export for compliance                      | Signed report generated          |
| TC-006  | Admin tries to delete audit logs                     | Permission denied                |
| TC-007  | Signature verification by external auditor           | Signature valid                  |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Hash chain implementation            | Sprint 1       |
| Phase 2 | Digital signatures                   | Sprint 2       |
| Phase 3 | Immutable external storage           | Sprint 3       |
| Phase 4 | Integrity verification + UI          | Sprint 4       |

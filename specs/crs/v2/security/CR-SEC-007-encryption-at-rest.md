# Change Request: Encryption at Rest — Application-Level

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-007                                               |
| **Feature ID**     | SEC-18                                                   |
| **Title**          | Application-Level Encryption at Rest                     |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai application-level encryption at rest (ALE) cho dữ liệu nhạy cảm được lưu trong Bytebase metadata database. Khác với database-level encryption (NF-SE01), ALE bảo vệ dữ liệu ngay cả khi database bị compromise.

### 1.2 Bối cảnh
Bytebase lưu nhiều dữ liệu nhạy cảm trong PostgreSQL metadata store: database connection credentials, API keys, user passwords, audit logs. NF-SE01 chỉ mention database-level encryption — chưa có application-level protection.

---

## 2. Yêu cầu chức năng

### FR-001: Envelope Encryption
- **Architecture**:
  ```
  Data → Encrypt with DEK (Data Encryption Key)
    → DEK encrypted with KEK (Key Encryption Key)
      → KEK stored in External Secret Manager (Vault/AWS KMS/GCP KMS)
  ```
- **Acceptance Criteria**:
  - AC-1: AES-256-GCM cho data encryption
  - AC-2: Unique DEK per sensitive data category
  - AC-3: KEK managed by External Secret Manager (link CR-ENT-015)
  - AC-4: Fallback KEK from environment variable (for non-Vault deployments)
  - AC-5: Key versioning — support key rotation without data re-encryption

### FR-002: Sensitive Data Identification
- **Data Categories**:

| Category                    | Fields                                    | Encryption Level |
|-----------------------------|-------------------------------------------|-----------------|
| Database Credentials        | Connection passwords, SSL certs           | Mandatory       |
| API Keys / Service Account  | API key hashes, tokens                    | Mandatory       |
| User Passwords              | Password hashes (already bcrypt)          | Mandatory       |
| SSO Secrets                 | OIDC client secrets, SAML keys            | Mandatory       |
| Webhook Secrets             | Webhook signing keys                      | Mandatory       |
| Audit Log Metadata          | IP addresses, user actions                | Optional        |
| Backup Data                 | Schema backups, data snapshots            | Conditional     |

- **Acceptance Criteria**:
  - AC-1: All database connection credentials encrypted at application level
  - AC-2: All external integration secrets encrypted
  - AC-3: Encryption/decryption transparent to application logic (store layer)
  - AC-4: Encrypted fields identifiable in database (prefix/marker)

### FR-003: Key Rotation
- **Acceptance Criteria**:
  - AC-1: Online key rotation — no downtime
  - AC-2: Background re-encryption worker cho old DEKs
  - AC-3: Key rotation audit trail
  - AC-4: Rollback capability — revert to previous key version
  - AC-5: Admin-triggered and scheduled rotation

### FR-004: Encryption-at-Rest Validation
- **Acceptance Criteria**:
  - AC-1: Admin dashboard showing encryption status per data category
  - AC-2: Health check endpoint verifying encryption service availability
  - AC-3: Alert on unencrypted sensitive data detection
  - AC-4: Compliance report: encryption coverage percentage

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Encryption Service (new)     | `backend/component/encryption/`             | Envelope encryption engine                  |
| Store Layer                  | `backend/store/`                            | Transparent encrypt/decrypt hooks           |
| Secret Component             | `backend/component/secret/`                 | KEK integration with Vault/KMS             |
| Key Rotation Runner (new)    | `backend/runner/keyrotation/`               | Background re-encryption worker             |
| Migration                    | `backend/migrator/`                         | Migrate existing plaintext → encrypted      |

---

## 4. Security Considerations

| Concern                     | Mitigation                                                    |
|-----------------------------|---------------------------------------------------------------|
| KEK compromise              | KEK in HSM/KMS, never in application memory long-term         |
| DEK exposure in memory      | Clear DEK from memory after use, use Go's memguard patterns   |
| Performance overhead         | AES-NI hardware acceleration, cache decrypted values briefly  |
| Migration safety             | Dual-write during migration, rollback capability              |

---

## 5. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Store database credential                            | Encrypted in PostgreSQL          |
| TC-002  | Retrieve database credential                         | Decrypted transparently          |
| TC-003  | Direct DB query on encrypted field                   | Returns ciphertext               |
| TC-004  | Key rotation triggered                               | New DEK, old data re-encrypted   |
| TC-005  | KMS unavailable                                      | Graceful degradation, alert      |
| TC-006  | Encryption status dashboard                          | Shows coverage percentage        |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Encryption service + envelope model  | Sprint 1       |
| Phase 2 | Store layer integration              | Sprint 2       |
| Phase 3 | Migration of existing data           | Sprint 3       |
| Phase 4 | Key rotation + validation dashboard  | Sprint 4       |

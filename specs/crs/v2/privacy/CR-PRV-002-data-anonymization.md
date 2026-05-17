# Change Request: Data Anonymization & Pseudonymization

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-002                                               |
| **Feature ID**     | SEC-15 (extends), UR-S02                                 |
| **Title**          | Data Anonymization & Pseudonymization                    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-012 (Data Masking), CR-PRV-001, CR-PRV-007       |

---

## 1. Tổng quan

### 1.1 Mô tả
Mở rộng khả năng Data Masking (CR-ENT-012) với **anonymization** (xóa bỏ hoàn toàn khả năng liên kết dữ liệu với cá nhân) và **pseudonymization** (thay thế PII bằng mã định danh giả, có thể reverse với key). Đây là yêu cầu bắt buộc theo GDPR Article 4(5) và PDPA.

### 1.2 Bối cảnh từ PRD/URD
- **PRD SEC-15**: Data Masking chỉ hỗ trợ display-level masking — chưa có true anonymization
- **URD UR-S02**: Security Officer cần che dấu PII cho non-production environments
- **PRD DCM-07**: Automatic backup trước data changes — backup cũng cần anonymization

### 1.3 Mục tiêu
- Anonymization pipeline cho data export và environment provisioning
- Pseudonymization với deterministic token mapping (consistent across tables)
- Format-preserving encryption (FPE) cho dữ liệu cần giữ format
- Integration với Data Masking engine hiện tại (CR-ENT-012)

---

## 2. Yêu cầu chức năng

### FR-001: Anonymization Techniques
- **Techniques**:

| Technique               | Mô tả                                     | Use Case                        |
|--------------------------|---------------------------------------------|--------------------------------|
| **Generalization**       | Thay giá trị cụ thể bằng range             | Age: 25 → 20-30               |
| **Suppression**          | Xóa bỏ hoàn toàn giá trị                   | SSN: → NULL                   |
| **Noise Addition**       | Thêm noise vào numeric data                 | Salary: 5000 → 5127           |
| **Synthetic Data**       | Generate dữ liệu giả thay thế              | Name: Nguyễn Văn A → Trần B   |
| **K-Anonymity**          | Đảm bảo mỗi record không unique            | Quasi-identifiers grouped      |
| **Data Swapping**        | Hoán đổi giá trị giữa records              | Address swapped between rows   |

- **Acceptance Criteria**:
  - AC-1: Anonymized data irreversible (không thể truy ngược)
  - AC-2: Referential integrity maintained across related tables
  - AC-3: Configurable per-column technique selection

### FR-002: Pseudonymization Engine
- **Mô tả**: Thay PII bằng token, deterministic mapping qua HMAC.
- **Features**:
  - Deterministic: cùng input → cùng token (cho JOIN consistency)
  - Key rotation support
  - Separate key storage (External Secret Manager — CR-ENT-015)
  - Re-identification chỉ bởi authorized personnel
- **Acceptance Criteria**:
  - AC-1: Token mapping key lưu trong External Secret Manager
  - AC-2: Audit log cho mọi re-identification request
  - AC-3: Key rotation không break existing pseudonymized data

### FR-003: Format-Preserving Encryption (FPE)
- **Mô tả**: Mã hóa giữ nguyên format (email vẫn là email, phone vẫn là phone).
- **Supported Formats**: Email, Phone, Credit Card, National ID, Date
- **Acceptance Criteria**:
  - AC-1: Output format identical to input format
  - AC-2: FPE algorithm: FF1 hoặc FF3-1 (NIST SP 800-38G)

### FR-004: Anonymization Policies
- **Mô tả**: Policy-based anonymization rules.
- **Policy Configuration**:
  ```yaml
  anonymization_policies:
    - name: "non-prod-policy"
      target_environments: ["dev", "staging"]
      rules:
        - column_pattern: "*.email"
          technique: PSEUDONYMIZATION
        - column_pattern: "*.phone"
          technique: FORMAT_PRESERVING
        - classification: "FINANCIAL"
          technique: NOISE_ADDITION
          params: { noise_range: 10% }
        - classification: "PII"
          technique: SYNTHETIC_DATA
  ```
- **Acceptance Criteria**:
  - AC-1: Policy auto-applied khi clone database to non-prod
  - AC-2: Policy validation trước khi apply
  - AC-3: Dry-run mode để preview anonymized data

---

## 3. Yêu cầu kỹ thuật

| Component                  | File/Package                                | Thay đổi                          |
|----------------------------|---------------------------------------------|-----------------------------------|
| Anonymization Engine       | `backend/component/privacy/anonymizer.go`   | Core anonymization pipeline       |
| Pseudonymization Service   | `backend/component/privacy/pseudonym.go`    | HMAC-based token mapping          |
| FPE Module                 | `backend/component/privacy/fpe.go`          | FF1/FF3-1 implementation          |
| Policy Store               | `backend/store/anonymization_policy.go`     | Policy CRUD                       |
| Export Integration         | `backend/component/export/`                 | Apply anonymization on export     |
| Feature Gate               | `enterprise/feature.go`                     | `FeatureDataAnonymization`        |

---

## 4. Test Cases

| Test ID | Mô tả                                         | Expected Result                 |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | Pseudonymize email column                      | Deterministic token generated  |
| TC-002  | Same email → same token across tables          | Referential integrity kept     |
| TC-003  | FPE on phone number                            | Output is valid phone format   |
| TC-004  | Synthetic data generation for names            | Realistic fake names           |
| TC-005  | K-anonymity check on quasi-identifiers         | k ≥ 5 per group               |
| TC-006  | Key rotation for pseudonymization              | Old data still decodable       |
| TC-007  | Dry-run anonymization preview                  | Preview without modification   |

---

## 5. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Anonymization techniques (suppression, generalization) | Sprint 1 |
| Phase 2 | Pseudonymization engine + key management  | Sprint 2       |
| Phase 3 | Format-preserving encryption              | Sprint 3       |
| Phase 4 | Policy engine + environment integration   | Sprint 3       |
| Phase 5 | Synthetic data generation                 | Sprint 4       |

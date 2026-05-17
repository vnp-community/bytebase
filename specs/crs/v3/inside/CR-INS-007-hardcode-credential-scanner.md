# Change Request: Hardcode Credential Scanner

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-007                                               |
| **Gap ID**         | G7                                                       |
| **Title**          | Hardcode Credential Scanner                              |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Module quét SQL statements trong Bytebase pipeline để phát hiện hardcoded passwords, credentials, và secrets trước khi chúng được commit vào database changelog. Hoạt động như **pre-commit secret scanner** cho database changes.

### 1.2 Bối cảnh
Gap G7 đề xuất GitLeaks cho source code scanning. Tuy nhiên, Bytebase cũng cần scanner cho **SQL statements** trong change pipeline — nơi DBA có thể vô tình hardcode passwords trong CREATE USER, connection strings, hoặc stored procedures.

### 1.3 Mục tiêu
- Scan SQL trong pipeline cho hardcoded credentials
- Block deployment nếu phát hiện plaintext password
- Scan SQL Review sheets/templates
- Integration với SQL Review policy engine

---

## 2. Yêu cầu chức năng

### FR-001: SQL Credential Scanner
- Scan tất cả SQL statements trong Issue/Plan trước rollout
- Detection patterns:
  - `CREATE USER ... IDENTIFIED BY 'plaintext'`
  - `ALTER USER ... PASSWORD 'plaintext'` 
  - Connection strings: `postgresql://user:pass@host`
  - `DBMS_CRYPTO` calls với hardcoded keys
  - Stored procedures chứa credentials
  - Config tables với password values
- Custom pattern registry (regex-based)

### FR-002: SQL Review Integration
- Tích hợp dưới dạng SQL Review Rule mới:
  - Rule: `security.no-hardcoded-credential`
  - Severity: ERROR (block deployment)
  - Applicable engines: ALL
- DBA phải sử dụng template variables: `{{password}}` thay vì plaintext
- Credentials resolved từ External Secret Manager (SEC-18)

### FR-003: Sheet/Template Scanner
- Scan SQL Sheets (templates) cho hardcoded values
- Periodic scan existing sheets
- Flag violations → notify sheet owner

### FR-004: Audit Log Scanner
- Scan historical audit logs cho past credential exposures
- Generate remediation report
- Track credential rotation status

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Credential Scanner | `backend/component/security/credential_scanner.go` | Pattern matching |
| Pattern Registry | `backend/store/credential_pattern.go` | Custom patterns |
| SQL Review Rule | `backend/plugin/advisor/*/advisor_no_hardcoded_cred.go` | New review rule |
| Scanner API | `backend/api/v1/credential_scan_service.go` | API endpoints |
| Scanner UI | `frontend/src/views/Security/CredentialScan.vue` | Results view |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | SQL `CREATE USER x IDENTIFIED BY 'pass123'` | Blocked by scanner |
| TC-002 | SQL with `{{password}}` variable | Passes scan |
| TC-003 | Connection string in stored proc | Detected & flagged |
| TC-004 | Custom pattern added → new SQL matches | Detected |
| TC-005 | Historical sheet scan finds credential | Notification sent |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Core scanner + SQL Review rule | Sprint 1 |
| Phase 2 | Sheet scanner + custom patterns | Sprint 2 |
| Phase 3 | Historical scan + audit | Sprint 3 |

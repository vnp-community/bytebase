# Change Request: Password Restrictions

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-010                                               |
| **Feature ID**     | SEC-13                                                   |
| **Title**          | Password Restrictions Policy                             |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai chính sách mật khẩu nghiêm ngặt cho ENTERPRISE plan, bao gồm complexity requirements, rotation policy, password history, và account lockout.

### 1.2 Mục tiêu
- Enforce password complexity requirements
- Password rotation (expiry) policy
- Password history (prevent reuse)
- Account lockout after failed attempts
- Compliance với NIST SP 800-63B guidelines

---

## 2. Yêu cầu chức năng

### FR-001: Password Complexity Policy
- **Mô tả**: Configurable password complexity rules.
- **Configuration**:
  ```yaml
  password_policy:
    min_length: 12           # 8-128
    max_length: 128
    require_uppercase: true
    require_lowercase: true
    require_digits: true
    require_special: true
    special_characters: "!@#$%^&*()_+-=[]{}|;':\",./<>?"
    disallow_common: true    # Check against common password list
    disallow_username: true  # Password cannot contain username
  ```
- **Acceptance Criteria**:
  - AC-1: Password validation enforced on create/change password
  - AC-2: Real-time strength indicator trên UI
  - AC-3: Clear error messages cho mỗi violated rule
  - AC-4: Common password list (top 10K) checked server-side

### FR-002: Password Rotation Policy
- **Mô tả**: Force password change sau configurable period.
- **Configuration**:
  ```yaml
  rotation:
    max_age_days: 90        # 0 = disabled
    warning_days: 14        # Warn N days before expiry
    grace_period_days: 7    # Allow login N days after expiry
  ```
- **Acceptance Criteria**:
  - AC-1: Warning banner khi password sắp hết hạn
  - AC-2: Force redirect tới change password page khi expired
  - AC-3: Grace period cho phép login nhưng force change
  - AC-4: Service Accounts exempt from rotation

### FR-003: Password History
- **Mô tả**: Prevent password reuse.
- **Configuration**:
  ```yaml
  history:
    remember_count: 10     # Remember last N passwords
    min_age_hours: 24      # Minimum time between changes
  ```
- **Acceptance Criteria**:
  - AC-1: Reject password nếu matches last N passwords
  - AC-2: Password history stored as salted hashes
  - AC-3: Min age prevents rapid cycling through history

### FR-004: Account Lockout Policy
- **Mô tả**: Lock account after consecutive failed login attempts.
- **Configuration**:
  ```yaml
  lockout:
    max_attempts: 5          # Failed attempts before lockout
    lockout_duration_min: 30 # Auto-unlock after N minutes
    reset_attempts_min: 15   # Reset counter after N minutes of no failures
  ```
- **Acceptance Criteria**:
  - AC-1: Account locked after N consecutive failures
  - AC-2: Auto-unlock after configured duration
  - AC-3: Admin can manually unlock account
  - AC-4: Lockout event logged to audit log

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                         | Thay đổi                                          |
|------------------------------|--------------------------------------|----------------------------------------------------|
| Password Policy Service      | `backend/api/v1/password_policy.go`  | Policy CRUD + validation                           |
| Auth Service                 | `backend/api/auth/`                  | Enforce policy on login/change password            |
| Feature Gate                 | `enterprise/feature.go`              | Define `FeaturePasswordRestrictions`               |
| Store Layer                  | `backend/store/principal.go`         | Password history + lockout tracking                |
| Data Cleaner                 | `backend/runner/cleaner/`            | Expired password history cleanup                   |

### 3.2 Database Changes

```sql
ALTER TABLE principal ADD COLUMN password_changed_at TIMESTAMPTZ;
ALTER TABLE principal ADD COLUMN failed_login_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE principal ADD COLUMN locked_until TIMESTAMPTZ;

CREATE TABLE password_history (
    id BIGSERIAL PRIMARY KEY,
    user_uid BIGINT NOT NULL REFERENCES principal(id),
    password_hash TEXT NOT NULL,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_history_user ON password_history (user_uid, created_ts DESC);
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Password < min_length                                   | Rejected with specific error          |
| TC-002     | Password without uppercase                              | Rejected (if required)                |
| TC-003     | Password in common list                                 | Rejected                              |
| TC-004     | Password reuse (in last 10)                             | Rejected                              |
| TC-005     | Login after password expired                            | Forced to change password             |
| TC-006     | 6th failed login attempt                                | Account locked                        |
| TC-007     | Auto-unlock after lockout duration                      | Login allowed                         |
| TC-008     | Admin unlock locked account                             | Account unlocked immediately          |
| TC-009     | Non-ENTERPRISE: no password policy                      | Default simple validation only        |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Complexity rules backend             | Sprint 1       |
| Phase 2 | Frontend strength indicator          | Sprint 1       |
| Phase 3 | Rotation policy                      | Sprint 2       |
| Phase 4 | Password history                     | Sprint 2       |
| Phase 5 | Account lockout                      | Sprint 3       |

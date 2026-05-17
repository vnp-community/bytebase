# Solution: CR-ENT-010 — Password Restrictions

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-010                |
| **Solution**   | SOL-ENT-010               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai configurable password policy engine tại L4 (AuthService), bao gồm complexity rules, rotation policy, password history, và account lockout. Tuân thủ NIST SP 800-63B.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `password_policy.go` (NEW) | Policy CRUD + validation engine |
| **L4 — Service** | `auth/` | Enforce on login/change password |
| **L8 — Store** | `principal` table extensions | Password metadata + lockout |
| **L8 — Store** | `password_history` (NEW) | Prevent password reuse |
| **L6 — Runner** | `runner/cleaner/` | Expired history cleanup |
| **L9 — Enterprise** | `feature.go` | `FeaturePasswordRestrictions` gate |

---

## 3. Chi tiết Implementation

### 3.1 Password Validation Engine

```go
func (v *PasswordValidator) Validate(ctx context.Context, password string, userEmail string) []ValidationError {
    var errors []ValidationError
    policy := v.getPolicy(ctx)

    if len(password) < policy.MinLength { errors = append(errors, ErrTooShort) }
    if policy.RequireUppercase && !hasUppercase(password) { errors = append(errors, ErrNoUppercase) }
    if policy.RequireLowercase && !hasLowercase(password) { errors = append(errors, ErrNoLowercase) }
    if policy.RequireDigits && !hasDigit(password) { errors = append(errors, ErrNoDigit) }
    if policy.RequireSpecial && !hasSpecial(password) { errors = append(errors, ErrNoSpecial) }
    if policy.DisallowUsername && containsIgnoreCase(password, userEmail) { errors = append(errors, ErrContainsUsername) }
    if policy.DisallowCommon && isCommonPassword(password) { errors = append(errors, ErrCommonPassword) }

    return errors
}
```

### 3.2 Schema Migration

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

### 3.3 Account Lockout

```go
func (s *AuthService) handleFailedLogin(ctx context.Context, user *store.UserMessage) {
    user.FailedLoginAttempts++
    if user.FailedLoginAttempts >= policy.MaxAttempts {
        user.LockedUntil = time.Now().Add(policy.LockoutDuration)
        // Emit audit event: ACCOUNT_LOCKED
    }
    s.store.UpdatePrincipal(ctx, user)
}
```

### 3.4 Rotation Policy

- Warning banner khi password sắp hết hạn (N days trước)
- Force redirect tới change password page khi expired
- Service Accounts exempt from rotation

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-009 | 2FA + password restrictions = defense-in-depth |
| CR-ENT-003 | Lockout events logged to audit |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Complexity rules backend | Sprint 1 |
| 2 | Frontend strength indicator | Sprint 1 |
| 3 | Rotation policy | Sprint 2 |
| 4 | Password history | Sprint 2 |
| 5 | Account lockout | Sprint 3 |

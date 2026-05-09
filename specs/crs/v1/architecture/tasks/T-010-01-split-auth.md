# T-010-01: Split auth_service.go

| Field | Value |
|---|---|
| **Task ID** | T-010-01 |
| **Solution** | SOL-ARCH-010 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target Files** | `backend/api/v1/auth_service.go` → split into 4 files |
| **Type** | Refactor |

---

## Objective

Split `auth_service.go` (1,930 lines) into domain-focused files. Same package, same struct, zero behavioral change.

## Implementation

| New File | Domain | Est. Lines |
|----------|--------|------------|
| `auth_service.go` | Struct + constructor only | ~150 |
| `auth_service_login.go` | Login, Logout, CreateUser | ~450 |
| `auth_service_mfa.go` | MFA setup, verify, backup codes | ~380 |
| `auth_service_sso.go` | OIDC, SAML, SSO providers | ~500 |
| `auth_service_password.go` | Password reset, change, hash | ~450 |

**Process**: Copy file → remove irrelevant methods → commit with descriptive message.

## Acceptance Criteria

- [ ] Each resulting file < 800 lines
- [ ] `go build ./backend/api/v1/...` passes
- [ ] All existing tests pass unchanged
- [ ] No import changes in consumers
- [ ] `git log --follow` tracks method history

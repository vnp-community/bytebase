# TASK-WEAK-004-1: auth_service.go Split (→ 6 files)

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P1 |
| Depends On | — |
| Est. | M (move ~1600 LoC across files) |

## Objective

Split `auth_service.go` (1930 lines) into 6 domain files. Zero functional change — same package, same struct, method redistribution only.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/auth_service.go` — keep struct + constructor + Login |
| CREATE | `backend/api/v1/auth_signup.go` — Signup, CreateUser, email verification |
| CREATE | `backend/api/v1/auth_sso.go` — OAuth2, OIDC, SAML, LDAP flows |
| CREATE | `backend/api/v1/auth_mfa.go` — TOTP setup, MFA verify, recovery codes |
| CREATE | `backend/api/v1/auth_password.go` — Password reset, change, policy |
| CREATE | `backend/api/v1/auth_token.go` — JWT generate, refresh, revoke, cookie |

## Specification

### Method distribution

All new files: `package v1`, methods on `*AuthService`:
- **auth_service.go** (~300 lines): `AuthService` struct, `NewAuthService()`, `Login()`
- **auth_signup.go**: `Signup()`, `CreateUser()`, `VerifyEmail()`
- **auth_sso.go**: `ExchangeToken()`, `GetIdentityProviderLoginURL()`, SSO callbacks
- **auth_mfa.go**: `SetupMFA()`, `VerifyMFA()`, `GenerateRecoveryCodes()`
- **auth_password.go**: `ResetPassword()`, `ChangePassword()`, `validatePasswordPolicy()`
- **auth_token.go**: `RefreshToken()`, `RevokeToken()`, `generateJWT()`, cookie helpers

### Verification

```bash
go build ./backend/api/v1/...
go vet ./backend/api/v1/...
go test ./backend/tests/... -count=1
```

## Acceptance Criteria

- [ ] `auth_service.go` reduced to ≤400 lines
- [ ] `go build` passes — no compile errors
- [ ] All existing tests pass unchanged
- [ ] No new imports needed (same package)

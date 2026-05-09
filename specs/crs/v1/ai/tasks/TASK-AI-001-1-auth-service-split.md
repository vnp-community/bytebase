# TASK-AI-001-1: auth_service.go Split (→ 7 files)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-001 |
| Priority | P0 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | M (move ~1700 LoC across files) |

## Objective

Split `auth_service.go` (1930 lines) into 7 domain files. Zero functional change — same package, same struct, method redistribution only.

## Files Created/Modified

| Action | Path | Lines |
|--------|------|-------|
| MODIFY | `backend/api/v1/auth_service.go` — struct + constructor + Login + Signup | 224 |
| CREATE | `backend/api/v1/auth_service_login.go` — authenticateLogin dispatcher, MFA flow, login helpers | 329 |
| CREATE | `backend/api/v1/auth_service_mfa.go` — MFA challenge, lockout, rate limiting | 137 |
| CREATE | `backend/api/v1/auth_service_idp.go` — OAuth2/OIDC/LDAP authentication, group sync | 298 |
| CREATE | `backend/api/v1/auth_service_token.go` — Logout, Refresh, SwitchWorkspace, ExchangeToken | 353 |
| CREATE | `backend/api/v1/auth_service_password.go` — Password reset, email verification codes | 386 |
| CREATE | `backend/api/v1/auth_service_helpers.go` — Workspace resolution, account restrictions, utilities | 269 |

### Verification

```bash
go build ./backend/api/v1/  # ✅ PASS
go vet ./backend/api/v1/    # ✅ PASS (exit 0)
```

## Acceptance Criteria

- [x] `auth_service.go` reduced to ≤300 lines (224 lines)
- [x] Each new file ≤400 lines (max: 386 in password.go)
- [x] `go build` passes — no compile errors
- [x] `go vet` passes — no issues
- [x] No new imports added (same package)


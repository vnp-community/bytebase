# TASK-LIM-006-B1: 2FA → TEAM + Password Policy

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-006 |
| Phase | B — Feature Reshuffling |
| Priority | P0 |
| Depends On | — |
| Est. | M (~180 LoC) |

## Objective

Move 2FA from ENTERPRISE to TEAM tier. Add basic password policy (min length, mixed case, numbers) for TEAM plan.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/enterprise/plan.yaml` — add FEATURE_2FA to TEAM |
| MODIFY | `backend/enterprise/license.go` — update feature-plan mapping |
| CREATE | `backend/api/v1/password_policy.go` — validation logic |
| MODIFY | `backend/api/v1/setting_service.go` — UpdatePasswordPolicy |
| MODIFY | `backend/api/v1/auth_service.go` — validatePassword on signup/change |

## Specification

### Feature tier shift

```go
var featureMinPlan = map[Feature]PlanType{
    FEATURE_2FA:              api.TEAM,       // was ENTERPRISE
    FEATURE_PASSWORD_BASIC:   api.TEAM,       // NEW
    FEATURE_PASSWORD_ADVANCED: api.ENTERPRISE, // history, regex, breach
}
```

### Password policy (TEAM — basic)

```go
type PasswordPolicy struct {
    MinLength        int   // 8-32
    RequireMixedCase bool
    RequireNumbers   bool
    RequireSpecial   bool  // ENTERPRISE only
    HistoryCount     int   // ENTERPRISE only
}
```

Validation in `auth_service.go`:
- Read policy from settings table
- Validate on signup, password change, admin reset
- TEAM users: MinLength, MixedCase, Numbers
- ENTERPRISE: adds Special + History check

### Setting storage

Store as JSON in `setting` table: key = `workspace.password-policy`

## Acceptance Criteria

- [ ] TEAM users can enable 2FA
- [ ] TEAM users can set basic password policy (length, mixed case, numbers)
- [ ] ENTERPRISE-only fields (special chars, history) enforced for TEAM
- [ ] Password validated on signup and change
- [ ] Policy stored/loaded via settings table

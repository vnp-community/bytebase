# TASK-SEC-012 — Brute-Force Login Lockout

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-012                               |
| **Source**       | SOL-SEC-003 §3.2, §3.3                    |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement progressive account lockout trong AuthService (L4) với login attempt tracking (L8).

## Scope

1. **Migration**: `login_attempt` table (id, email, ip_address, user_agent, success, geo_country, geo_city, created_ts)
2. **Migration**: `account_lockout` table (email PK, failed_count, locked_until, last_attempt)
3. **Store**: `store/login_attempt.go` — RecordAttempt, IncrementFailedAttempts, ResetFailedAttempts, GetLockoutStatus
4. **AuthService**: Progressive lockout logic:
   - 3 fails → 2s delay
   - 5 fails → CAPTCHA required (return `captcha_required` status)
   - 10 fails → 15min lock
   - 20 fails → 1h lock
   - 50 fails → 24h lock + admin notification
5. **Anti-enumeration**: Same error message for invalid user và locked account

## Acceptance Criteria

- [ ] Progressive delay/lockout verified
- [ ] Lockout resets on successful login
- [ ] Same error message for invalid user vs locked
- [ ] Admin notified at 50 fails
- [ ] Unit tests cho lockout progression

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | login_attempt, account_lockout |
| `backend/store/login_attempt.go` | New file |
| `backend/api/v1/auth_service.go` | Lockout logic |

## Definition of Done

- Lockout progression verified, no user enumeration leak

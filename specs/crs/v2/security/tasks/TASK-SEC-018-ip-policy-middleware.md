# TASK-SEC-018 — IP Policy Middleware + Geo-Restriction

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-018                               |
| **Source**       | SOL-SEC-006 §3.1-§3.3                     |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement IP policy middleware (L2) trong Echo middleware stack. Support CIDR allowlist/denylist, geo country restriction, per-instance DB connection IP restriction.

## Scope

1. **Middleware**: `ipPolicyMiddleware()` — insert after `recoverMiddleware`, before `securityHeadersMiddleware`
2. **IP resolution**: `resolveClientIP()` — parse `X-Forwarded-For` with trusted proxy validation (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
3. **Policy storage**: Reuse `policy` table, `PolicyType_IP_POLICY`, JSONB payload (mode, rules[], allowedCountries, deniedCountries, onViolation)
4. **CIDR matching**: Support IPv4/IPv6 CIDR ranges
5. **Geo restriction**: Use GeoIP component (TASK-SEC-013) for country check
6. **onViolation modes**: "block" (403), "mfa_challenge", "log_only" (monitor mode)
7. **DB connection restriction**: `DBFactory.GetDriver()` — check per-instance `IPAllowlist`

## Acceptance Criteria

- [ ] IP allowlist blocks non-whitelisted IPs
- [ ] X-Forwarded-For parsed correctly with trusted proxies
- [ ] Country restriction works via GeoIP
- [ ] Monitor mode logs but doesn't block
- [ ] Per-instance DB IP restriction works

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/server/echo_routes.go` | IP middleware |
| `backend/store/policy.go` | IP_POLICY type |
| `backend/component/dbfactory/factory.go` | Per-instance IP check |

## Definition of Done

- Middleware integrated in correct position in stack

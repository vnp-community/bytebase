# TASK-SEC-013 — GeoIP Component + Suspicious Login Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-013                               |
| **Source**       | SOL-SEC-003 §3.4, §3.5                    |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement GeoIP component (L5) dùng MaxMind GeoLite2 và Suspicious Login Runner (L6) cho impossible travel detection.

## Scope

1. **GeoIP**: `component/geoip/geoip.go` — MaxMind GeoLite2-City reader, `Lookup(ip) → (country, city)`
2. **Login enrichment**: Record `geo_country`, `geo_city` trong `login_attempt` table
3. **Security Runner**: `runner/security/suspicious_login.go` — 30s interval:
   - Impossible travel: same user, different country, < 2h apart
   - New device detection: unknown User-Agent for this user
4. **Email notification**: New device login → email user
5. **GeoLite2 update**: Auto-download via scheduled task or manual config

## Acceptance Criteria

- [ ] GeoIP lookup returns country + city cho valid IPs
- [ ] Impossible travel detected (different country < 2h)
- [ ] New device email sent
- [ ] Graceful fallback when GeoIP unavailable

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/geoip/geoip.go` | New file |
| `backend/runner/security/suspicious_login.go` | New file |
| `backend/server/server.go` | Bootstrap |

## Definition of Done

- GeoIP tested with known IPs, runner lifecycle managed

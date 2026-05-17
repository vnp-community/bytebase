# TASK-SEC-032 — TLS Manager + Server Hardening

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-032                               |
| **Source**       | SOL-SEC-014 §3.1, §3.4                    |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Implement TLS Manager component (L5) và harden server TLS config (L2).

## Scope

1. **TLSManager**: `component/tls/manager.go` — `GetServerTLSConfig()` (TLS 1.2 minimum, preferred cipher suites, client cert optional)
2. **Cipher suites**: TLS_AES_128/256_GCM (TLS 1.3), ECDHE_RSA/ECDSA_AES_256_GCM (TLS 1.2)
3. **Server hardening**: `server.go` — apply TLSConfig, HSTS preload header
4. **Certificate monitoring**: `MonitorExpiry()` goroutine — alert 30 days before cert expiry
5. **Migration**: `certificate` table (id, name, type, fingerprint UNIQUE, not_before, not_after, subject_cn, issuer_cn, cert_pem, key_pem encrypted, instance_uid, created_ts)

## Acceptance Criteria

- [ ] TLS 1.2 minimum enforced
- [ ] Only strong cipher suites allowed
- [ ] Certificate expiry monitoring works
- [ ] HSTS preload header set

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/tls/manager.go` | New file |
| `backend/server/server.go` | TLS config |
| `backend/migrator/migration/` | certificate table |

## Definition of Done

- TLS config verified via SSL Labs scan

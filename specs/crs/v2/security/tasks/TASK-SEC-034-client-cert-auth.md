# TASK-SEC-034 — Client Certificate Authentication

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-034                               |
| **Source**       | SOL-SEC-014 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 4                                   |

---

## Mô tả

Add client certificate as authentication method trong Auth Interceptor (L3).

## Scope

1. **Auth Interceptor**: `authenticateClientCert()` — extract `TLS.PeerCertificates[0]`, SHA-256 fingerprint, map to service account
2. **Store**: `GetServiceAccountByCertFingerprint()` — lookup principal by cert fingerprint
3. **Cert registration**: API to associate cert fingerprint with service account

## Acceptance Criteria

- [ ] Client cert maps to service account
- [ ] Unknown cert → 401
- [ ] Cert fingerprint registered via API

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/auth/` | Client cert auth |
| `backend/store/principal.go` | Cert fingerprint lookup |

## Definition of Done

- Client cert auth flow verified end-to-end

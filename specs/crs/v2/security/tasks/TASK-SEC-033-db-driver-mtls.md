# TASK-SEC-033 — DB Driver mTLS Extension

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-033                               |
| **Source**       | SOL-SEC-014 §3.2                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Extend DB Driver plugin interface (L7) với mTLS connection config cho 22 drivers.

## Scope

1. **ConnectionConfig extension**: ADD `TLS *TLSConfig` — Mode (disable/require/verify-ca/verify-full), CACert, ClientCert, ClientKey, ServerName
2. **PostgreSQL driver**: `plugin/db/pg/` — build pgx TLSConfig with custom CA, client cert, server name verification
3. **MySQL driver**: `plugin/db/mysql/` — build mysql TLS config
4. **Instance UI**: Frontend form fields for TLS mode, CA cert upload, client cert/key upload
5. **Store**: `data_source` ADD TLS config columns (or JSONB)

## Acceptance Criteria

- [ ] PostgreSQL mTLS connection works
- [ ] MySQL mTLS connection works
- [ ] verify-full validates server CN
- [ ] Client cert uploaded via UI
- [ ] Fallback to non-TLS when mode=disable

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/plugin/db/pg/` | TLS config |
| `backend/plugin/db/mysql/` | TLS config |
| `backend/component/dbfactory/factory.go` | TLSConfig builder |
| `backend/store/instance.go` | TLS config storage |

## Definition of Done

- mTLS verified with PostgreSQL + MySQL test instances

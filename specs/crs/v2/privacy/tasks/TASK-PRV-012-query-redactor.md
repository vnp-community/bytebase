# TASK-PRV-012 — Query Redactor + Audit Privacy Pipeline

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-012                               |
| **Source**       | SOL-PRV-005 Phase 1–3 (CR-PRV-005)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng Query Redactor (L5) sử dụng ANTLR SQL Parser (L7) và integrate vào Audit Interceptor (L3) cho privacy-preserving audit logging.

## Scope

1. **L5 — `component/privacy/redactor.go`**: QueryRedactor — parse SQL AST via ANTLR, redact string/numeric literals in WHERE/SET/VALUES/INSERT
2. **Redaction levels**: OFF, LITERALS_ONLY, FULL
3. **Regex fallback**: Cho engines chưa có ANTLR parser
4. **Query hash**: SHA-256 hash of original query stored cho forensic correlation
5. **L5 — `component/privacy/param_filter.go`**: ParamFilter — strip passwords, tokens, secrets, API keys, connection string credentials from audit entries
6. **L3 — `api/v1/audit.go`** (modify): Integrate redactor + param filter vào existing `createAuditEntry()`
7. **L8 — Migration**: Add columns `detail_level`, `query_hash`, `encrypted_original` to `audit_log`
8. **L9 — `feature.go`**: `FeaturePrivacyAudit` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/redactor.go` | NEW — SQL literal redactor |
| `backend/component/privacy/param_filter.go` | NEW — Sensitive param filter |
| `backend/api/v1/audit.go` | MODIFY — Privacy pipeline |
| `backend/store/migration/` | NEW — ALTER TABLE audit_log |
| `backend/enterprise/feature.go` | ADD — `FeaturePrivacyAudit` |

## Acceptance Criteria

- [ ] String literals in WHERE/SET auto-redacted: `email = '[REDACTED]'`
- [ ] Table/column names preserved (NOT redacted)
- [ ] Redaction configurable: OFF, LITERALS_ONLY, FULL
- [ ] Query hash stored for forensic correlation
- [ ] Regex fallback for 13+ engines without ANTLR parser
- [ ] Sensitive parameters stripped: password, token, secret, API key
- [ ] Connection string passwords masked: `user:***@host`
- [ ] Audit detail levels: MINIMAL, STANDARD, DETAILED, FORENSIC

## Dependencies

- None (modifies existing audit interceptor)

## Definition of Done

- Redactor tested with PostgreSQL, MySQL, Oracle SQL
- Regex fallback tested with MongoDB, Redis
- Parameter filter edge cases validated

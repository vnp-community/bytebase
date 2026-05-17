# TASK-PRV-013 — Tiered Audit Access Control + FORENSIC Encryption

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-013                               |
| **Source**       | SOL-PRV-005 Phase 4–5 (CR-PRV-005)        |
| **Status**       | Pending                                    |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement tiered access control cho audit log queries và FORENSIC level encryption với break-glass access.

## Scope

1. **L4 — `api/v1/audit_log_service.go`** (modify): Tiered access control trên SearchAuditLogs
2. **Access tiers**: Viewer (MINIMAL+STANDARD), Auditor (+DETAILED), Forensic (+encrypted originals)
3. **Break-glass**: FORENSIC access requires justification + admin approval
4. **Meta-audit**: Access to audit logs itself audited (meta-audit log)
5. **FORENSIC encryption**: Original query encrypted with separate key via External Secret Manager, stored in `encrypted_original` column
6. **Break-glass decryption**: Forensic analyst can request decryption with justification

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/audit_log_service.go` | MODIFY — Tiered access |
| `backend/api/v1/audit.go` | MODIFY — FORENSIC encryption |

## Acceptance Criteria

- [ ] Viewer: only MINIMAL + STANDARD entries visible
- [ ] Auditor: + DETAILED entries
- [ ] Forensic: + encrypted originals (with break-glass)
- [ ] Break-glass requires justification string
- [ ] Meta-audit: forensic access logged
- [ ] FORENSIC encryption key in External Secret Manager
- [ ] Role-based: `bb.auditLogs.search`, `bb.auditLogs.forensic`

## Dependencies

- TASK-PRV-012 (Query redactor + detail levels)
- TASK-ENT-018 (External Secret Manager — for encryption key)

## Definition of Done

- Access tiers tested with different roles
- Break-glass workflow validated
- Meta-audit entries verified

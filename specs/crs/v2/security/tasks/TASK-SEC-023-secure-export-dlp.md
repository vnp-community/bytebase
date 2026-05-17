# TASK-SEC-023 — Secure Export Controls + DLP Scanner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-023                               |
| **Source**       | SOL-SEC-009 §3.1-§3.4                     |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Extend SQLService.Export (L4) với export policy, masking enforcement, DLP scanner component, encrypted export.

## Scope

1. **Export policy**: `ExportPolicy` struct (maxRows, maxSizeBytes, requireApproval, dlpEnabled, encryptExports, allowedFormats) — stored in `policy` table
2. **Row/size limits**: Check before export execution, return error if exceeded
3. **Masking enforcement**: Apply existing MaskingEvaluator (12KB) to export data
4. **DLP scanner**: `component/dlp/scanner.go` — regex patterns for SSN, credit card, email → block export if detected
5. **Encrypted export**: AES-256-GCM encryption with password-derived key (argon2)
6. **Approval workflow**: If `requireApproval=true` for PRODUCTION → create approval request

## Acceptance Criteria

- [ ] Row/size limits enforced
- [ ] Masking applied in exports
- [ ] DLP scanner blocks sensitive data export
- [ ] Encrypted export downloadable
- [ ] Approval required for production exports

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/sql_service.go` | Export interception |
| `backend/component/dlp/scanner.go` | New file |
| `backend/component/export/encrypted.go` | New file |

## Definition of Done

- DLP patterns detect SSN, credit card, email correctly

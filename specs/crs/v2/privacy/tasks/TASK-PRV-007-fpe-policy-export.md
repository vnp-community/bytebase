# TASK-PRV-007 — FPE Engine + Anonymization Policy + Export Integration

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-007                               |
| **Source**       | SOL-PRV-002 Phase 3–6 (CR-PRV-002)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 3–4                                 |

---

## Mô tả

Hoàn thiện anonymization stack: FPE engine (FF1), synthetic data generator, policy engine, export pipeline integration, dry-run mode.

## Scope

1. **L5 — `component/privacy/fpe.go`**: FF1/FF3-1 FPE — `EncryptEmail()`, `EncryptPhone()`, `EncryptCreditCard()`, `EncryptNationalID()`, `EncryptDate()`
2. **L5 — `component/privacy/synthetic.go`**: Synthetic data generator — realistic fake names, addresses, emails (locale-aware)
3. **Policy engine**: YAML-like policy config — per-environment, per-classification, per-column technique selection
4. **L5 — `component/export/privacy.go`**: Hook anonymization vào Export pipeline — auto-apply policy khi export to non-prod
5. **Dry-run mode**: Preview anonymized data trước khi apply

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/fpe.go` | NEW — FF1 FPE engine |
| `backend/component/privacy/synthetic.go` | NEW — Synthetic data gen |
| `backend/component/export/privacy.go` | NEW — Export anonymization |

## Acceptance Criteria

- [ ] FPE: output format identical to input (email→email, phone→phone)
- [ ] FPE algorithm: FF1 (NIST SP 800-38G), min input length ≥ 6
- [ ] Synthetic data: realistic names/addresses, locale-aware (Vietnamese + English)
- [ ] Policy auto-applied khi clone prod → non-prod
- [ ] Dry-run mode: preview without modification
- [ ] Export integration: anonymized export khi target env < source env
- [ ] Policy validation trước khi apply

## Dependencies

- TASK-PRV-005, TASK-PRV-006

## Definition of Done

- FPE tested: email, phone, credit card, national ID, date formats
- Synthetic data quality reviewed
- Export pipeline integration tested end-to-end

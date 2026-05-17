# TASK-SEC-030 — Compliance Engine + Evidence Collectors

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-030                               |
| **Source**       | SOL-SEC-012 §3.1-§3.2                     |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement Compliance Engine (L5) và evidence collectors thu thập data từ existing Store.

## Scope

1. **Migration**: `compliance_framework` (id PK, name, controls JSONB, is_active), `compliance_assessment` (id, framework_id FK, score, results JSONB, created_ts)
2. **ComplianceEngine**: `component/compliance/engine.go` — `Assess(frameworkID) → AssessmentResult`
3. **Evidence collectors**: audit_log (count entries), policy (check policy exists), setting (check value), role (list custom roles)
4. **Framework templates**: Pre-built SOC2, ISO 27001 controls as JSONB
5. **Score calculation**: (passing controls / total controls) * 100

## Acceptance Criteria

- [ ] SOC2 + ISO 27001 templates loaded
- [ ] Evidence collected from existing Store data
- [ ] Score calculated correctly
- [ ] Assessment results persisted

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/compliance/engine.go` | New file |
| `backend/migrator/migration/` | compliance tables |
| `backend/store/compliance.go` | New file |

## Definition of Done

- Assessment runs against real Store data

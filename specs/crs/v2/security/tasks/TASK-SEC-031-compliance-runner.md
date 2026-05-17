# TASK-SEC-031 — Compliance Runner + Report Generation

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-031                               |
| **Source**       | SOL-SEC-012 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement Compliance Runner (L6) cho scheduled assessments và report generation (PDF/HTML).

## Scope

1. **Runner**: `runner/compliance/runner.go` — weekly ticker, assess all active frameworks, save results
2. **Regression alert**: Compare score with previous → notify if decreased
3. **ComplianceService**: API endpoints — TriggerAssessment, GetAssessmentHistory, ExportReport
4. **Report generation**: Go templates → HTML → optional PDF (wkhtmltopdf)

## Acceptance Criteria

- [ ] Weekly assessment runs automatically
- [ ] Regression alerts on score decrease
- [ ] Report exportable as HTML
- [ ] API endpoints functional

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/compliance/runner.go` | New file |
| `backend/api/v1/compliance_service.go` | New service |

## Definition of Done

- Runner registered in bootstrap, report verified

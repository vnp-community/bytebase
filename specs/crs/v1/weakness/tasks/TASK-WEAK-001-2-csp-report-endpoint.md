# TASK-WEAK-001-2: CSP Violation Report Endpoint

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P1 |
| Depends On | TASK-WEAK-001-1 |
| Est. | S (~80 LoC) |

## Objective

Add `POST /api/csp-report` endpoint that logs CSP violations and exports Prometheus counter `bytebase_csp_violations_total{directive,blocked_uri}`.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/server/csp_report.go` |
| MODIFY | `backend/server/echo_routes.go` — register route |

## Specification

- Parse `csp-report` JSON body (RFC 7 CSP format)
- Log: `slog.Warn("CSP violation", document_uri, blocked_uri, violated_directive)`
- Metric: `cspViolationCounter.WithLabelValues(effective_directive, blocked_uri).Inc()`
- Return `204 No Content`
- Add `report-uri /api/csp-report` to CSP header in `buildCSP()`

## Acceptance Criteria

- [ ] Valid CSP report → 204 + counter increment
- [ ] Invalid JSON → 400
- [ ] Metric labels: `directive`, `blocked_uri`

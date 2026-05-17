# TASK-WEAK-001-2: CSP Violation Report Endpoint

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-001 |
| Priority | P1 |
| Depends On | TASK-WEAK-001-1 |
| Est. | S (~80 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-10 |

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

- [x] Valid CSP report → 204 + counter increment
- [x] Invalid JSON → 400
- [x] Metric labels: `directive`, `blocked_uri`

## Implementation Notes

- Created `backend/server/csp_report.go` with `handleCSPReport()` and `registerCSPReportRoute()`
- Prometheus counter registered via `promauto.NewCounterVec` with namespace `bytebase`, name `csp_violations_total`
- `blocked_uri` label truncated to 128 chars to prevent high-cardinality metric explosion
- Uses `effective-directive` (CSP Level 3) with fallback to `violated-directive` (Level 2)
- Route registered in `echo_routes.go` via `registerCSPReportRoute(e)` alongside `newSecurityHeadersMiddleware`
- `report-uri /api/csp-report` directive added to both `buildCSP()` and `buildCSPDev()` in `csp.go`

# TASK-LIM-003-A1: Production Readiness Checker + API

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-003 |
| Phase | A — Readiness Warning |
| Priority | P1 |
| Depends On | — |
| Est. | M (~200 LoC) |

## Objective

Tạo service phát hiện khi embedded PG usage vượt ngưỡng production (>5 instances, >10 users, >100 changes, >30d uptime, has prod env). Expose qua API cho frontend banner.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/readiness/checker.go` |
| MODIFY | `backend/api/v1/actuator_service.go` — add GetProductionReadiness, DismissWarning |

## Specification

### `checker.go`

- `ReadinessChecker` struct with `store`, `profile`
- `Check(ctx) → ReadinessReport` — evaluates 5 criteria
- `ReadinessReport`: `IsEmbedded`, `CriteriaMet`, `ShowWarning` (true if ≥2 criteria met), `Details[]`
- 5 criteria: instance count >5, user count >10, changelog >100, uptime >30d, has production env

### API endpoints

- `GetProductionReadiness` — returns ReadinessReport
- `DismissReadinessWarning` — stores dismiss until +30d in setting table

## Acceptance Criteria

- [x] Returns `IsEmbedded=false` for external PG (no warning) → **DONE**: Early return in `Check()` when `!profile.UseEmbedDB()`
- [x] Correctly evaluates all 5 criteria → **DONE**: instance_count >5, user_count >10, changelog_count >100, uptime >30d, production_env
- [x] Warning shows when ≥2 criteria met → **DONE**: `ShowWarning = CriteriaMet >= 2`
- [x] Dismiss persists for 30 days via settings → **DONE**: Stored in `server_config` table with RFC3339 expiry

## Implementation Notes

- Created `backend/component/readiness/checker.go` (~200 LoC)
  - Direct SQL queries for instance/changelog counts (avoids store method proliferation)
  - Production env detection via case-insensitive "prod" substring matching in environment settings
  - Uptime tracked from server start timestamp
- Added `GetReadinessReport()` and `DismissReadinessWarning()` to `ActuatorService`
- Dismiss uses `server_config(key, value)` table with `ON CONFLICT DO UPDATE` upsert

**Status: ✅ DONE**

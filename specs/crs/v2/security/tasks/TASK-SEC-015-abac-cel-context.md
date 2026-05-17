# TASK-SEC-015 — ABAC CEL Context Extension

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-015                               |
| **Source**       | SOL-SEC-004 §3.1, §3.2                    |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Extend IAM Manager CEL environment (L5) với ABAC context attributes: time, IP, environment tier, risk level. Inject context trong ACL Interceptor.

## Scope

1. **CEL variables**: Extend `cel.NewEnv()` trong `component/iam/manager.go`:
   - `request.time` (time.Time), `request.source_ip` (string)
   - `request.environment.tier` (PRODUCTION/STAGING/DEV), `request.environment.name`
   - `change.risk_level` (HIGH/MEDIUM/LOW), `change.statement_type` (DDL/DML/DQL)
2. **ACL Interceptor**: `buildRequestContext()` — resolve environment tier from method, extract IP
3. **ABAC Policy type**: Reuse `policy` table, `PolicyType_ABAC`, payload = ABACRule[] (name, condition CEL, effect DENY/ALLOW, priority)
4. **Feature gate**: `FeatureABAC` trong `enterprise/feature.go`

## Acceptance Criteria

- [ ] CEL expression `request.time.getHours() >= 9 && request.time.getHours() <= 18` works
- [ ] Environment tier resolved correctly
- [ ] ABAC policies evaluated in priority order
- [ ] Feature gated to ENTERPRISE plan

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/iam/manager.go` | CEL env extension |
| `backend/api/v1/acl.go` | Context injection |
| `backend/enterprise/feature.go` | FeatureABAC |

## Definition of Done

- CEL context attributes verified via unit tests

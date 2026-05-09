# TASK-WEAK-003-4: Blanket nolint Replacement + Error Metrics

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-003 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~30 LoC changes) |

## Objective

Replace blanket `// nolint` with specific lint rule + justification. Pure comment change — zero functional impact.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/masking_evaluator.go` — lines 52, 63, 78 |
| MODIFY | `backend/api/v1/user_service.go` — line 766 |
| MODIFY | `backend/api/v1/sql_service.go` — line 1857 |

## Specification

| File:Line | Before | After |
|-----------|--------|-------|
| `masking_evaluator.go:52` | `// nolint` | `//nolint:unused // evaluateMaskingLevelOfColumn called via reflection` |
| `masking_evaluator.go:63` | `// nolint` | `//nolint:unused // maskingPolicyEvaluator registered dynamically` |
| `masking_evaluator.go:78` | `// nolint` | `//nolint:unused // evaluateColumnMaskingWithPolicy called via interface` |
| `user_service.go:766` | `//nolint:nilerr` | `//nolint:nilerr // user lookup miss returns nil, not error` |
| `sql_service.go:1857` | `//nolint:nilerr` | `//nolint:nilerr // parse failure returns empty result, not error` |

## Acceptance Criteria

- [ ] No blanket `// nolint` comments remain in modified files
- [ ] Each `//nolint:` has specific lint rule name
- [ ] Each `//nolint:` has justification comment
- [ ] `go vet ./backend/api/v1/...` passes

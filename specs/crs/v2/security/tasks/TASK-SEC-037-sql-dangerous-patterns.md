# TASK-SEC-037 — SQL Dangerous Pattern Detection (AST)

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-037                               |
| **Source**       | SOL-SEC-016 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement dangerous SQL pattern detection using existing ANTLR parsers (L7). Block DELETE/UPDATE without WHERE, DROP, TRUNCATE, GRANT/REVOKE, UNION injection patterns.

## Scope

1. **SecurityAnalyzer**: `plugin/parser/security.go` — reuse existing ANTLR parser per engine
2. **Dangerous patterns**: DROP, TRUNCATE, DELETE without WHERE, UPDATE without WHERE, DCL (GRANT/REVOKE), UNION with subquery
3. **Integration**: `sql_service.go` — call `detectDangerousPatterns()` before execution
4. **Policy**: `AllowDangerousStatements` flag per role — if false, block; if true, emit security event
5. **CI integration**: `gosec` rules, grep check for SQL string concatenation in `backend/store/`

## Acceptance Criteria

- [ ] DROP/TRUNCATE detected and blocked
- [ ] DELETE without WHERE detected
- [ ] UNION injection pattern flagged
- [ ] CI gosec + string concat check passes
- [ ] No `fmt.Sprintf` with SQL in store layer

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/plugin/parser/security.go` | New file |
| `backend/api/v1/sql_service.go` | Pattern detection call |
| `.github/workflows/` | CI security scan job |

## Definition of Done

- AST patterns tested per engine (PG, MySQL)

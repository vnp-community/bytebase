# Tasks — Weakness Solutions

> Tác vụ được chia nhỏ từ 8 Solution proposals (SOL-WEAK-001→008), tối ưu cho token cost khi thực thi bởi AI agent.

## Task Naming Convention

```
TASK-WEAK-{SOL}-{SEQ} — {short-title}
  SOL: 001-008 (maps to SOL-WEAK-XXX)
  SEQ: 1-9
```

## Master Task Index

### SOL-WEAK-001 — CSP Security Hardening (4 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-001-1 | CSP nonce middleware | P0 | — | M |
| TASK-WEAK-001-2 | CSP violation report endpoint | P1 | 001-1 | S |
| TASK-WEAK-001-3 | Frontend nonce injection (HTML + Naive UI) | P0 | 001-1 | M |
| TASK-WEAK-001-4 | connect-src tightening (ws→wss, remove data:) | P1 | 001-1 | S |

### SOL-WEAK-002 — CORS Safety Guard (2 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-002-1 | CORS refactor + configurable origins | P0 | — | S |
| TASK-WEAK-002-2 | CORS audit middleware + dev mode warning | P1 | 002-1 | S |

### SOL-WEAK-003 — Error Handling Hardening (4 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-003-1 | IAM error propagation (closures → error-returning) | P0 | — | M |
| TASK-WEAK-003-2 | ACL 503 for store errors | P0 | 003-1 | S |
| TASK-WEAK-003-3 | Migration executor warning propagation | P1 | — | M |
| TASK-WEAK-003-4 | Blanket nolint replacement + error metrics | P1 | — | S |

### SOL-WEAK-004 — Service Modularization (4 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-004-1 | auth_service.go split (→ 6 files) | P1 | — | M |
| TASK-WEAK-004-2 | sql_service.go split (→ 5 files) | P1 | — | M |
| TASK-WEAK-004-3 | rollout_service.go split (→ 4 files) | P1 | — | M |
| TASK-WEAK-004-4 | CI file size lint enforcement | P2 | 004-1 | S |

### SOL-WEAK-005 — Composite PK Guardrails (2 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-005-1 | Unique ID index migration | P0 | — | S |
| TASK-WEAK-005-2 | Store composite PK validation helper | P1 | 005-1 | S |

### SOL-WEAK-006 — JSONB Query Optimization (2 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-006-1 | GIN index migration (CONCURRENTLY) | P1 | — | S |
| TASK-WEAK-006-2 | Generated column + store dual read | P2 | 006-1 | M |

### SOL-WEAK-007 — Test Coverage Hardening (5 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-007-1 | Store interface extraction | P0 | — | M |
| TASK-WEAK-007-2 | Component + enterprise interface extraction | P0 | — | S |
| TASK-WEAK-007-3 | Mock generation setup (mockgen) | P0 | 007-1, 007-2 | S |
| TASK-WEAK-007-4 | Auth + changelog unit tests | P1 | 007-3 | M |
| TASK-WEAK-007-5 | CI coverage gate workflow | P1 | — | S |

### SOL-WEAK-008 — Connection Pool Optimization (4 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-WEAK-008-1 | PoolManager + dual pool architecture | P0 | — | L |
| TASK-WEAK-008-2 | Pool Prometheus metrics | P1 | 008-1 | S |
| TASK-WEAK-008-3 | Robust reconnection (replace time.Sleep) | P1 | 008-1 | M |
| TASK-WEAK-008-4 | Runner pool integration + Store wiring | P0 | 008-1 | M |

---

## Execution Priority Order

```
━━━ P0 — Security & Reliability (Sprint 1-2) ━━━
TASK-WEAK-003-1  →  IAM error propagation (silent failures = security risk)
TASK-WEAK-003-2  →  ACL 503 (depends 003-1)
TASK-WEAK-001-1  →  CSP nonce middleware (XSS mitigation)
TASK-WEAK-001-3  →  Frontend nonce injection (depends 001-1)
TASK-WEAK-002-1  →  CORS refactor (quick, high impact)
TASK-WEAK-005-1  →  Unique ID indexes (quick, protects queries)
TASK-WEAK-007-1  →  Store interfaces (enables all testing)
TASK-WEAK-007-2  →  Component interfaces
TASK-WEAK-007-3  →  Mock generation
TASK-WEAK-008-1  →  Dual pool (runner isolation)
TASK-WEAK-008-4  →  Runner pool wiring

━━━ P1 — Quality & Observability (Sprint 3-4) ━━━
TASK-WEAK-003-3  →  Migration warning propagation
TASK-WEAK-003-4  →  nolint + error metrics
TASK-WEAK-001-2  →  CSP violation reporting
TASK-WEAK-001-4  →  connect-src tightening
TASK-WEAK-002-2  →  CORS audit
TASK-WEAK-004-1  →  auth_service split
TASK-WEAK-004-2  →  sql_service split
TASK-WEAK-004-3  →  rollout_service split
TASK-WEAK-005-2  →  Composite PK guard
TASK-WEAK-006-1  →  GIN indexes
TASK-WEAK-007-4  →  Auth unit tests
TASK-WEAK-007-5  →  CI coverage gate
TASK-WEAK-008-2  →  Pool metrics
TASK-WEAK-008-3  →  Robust reconnect

━━━ P2 — Polish (Sprint 5+) ━━━
TASK-WEAK-004-4  →  CI file size lint
TASK-WEAK-006-2  →  Generated columns
```

## Estimation Key

| Size | Lines of Code | Token Budget | Duration |
|------|--------------|--------------|----------|
| **S** | < 100 LoC | < 2K tokens | < 1h |
| **M** | 100-300 LoC | 2K-5K tokens | 1-3h |
| **L** | 300+ LoC | 5K-10K tokens | 3-8h |

# Tasks — Limitation Solutions

> Tác vụ được chia nhỏ từ 6 Solution proposals, tối ưu cho token cost khi thực thi bởi AI agent.

## Task Naming Convention

```
TASK-LIM-{SOL}-{PHASE}{SEQ} — {short-title}
  SOL:   001-006 (maps to SOL-LIM-XXX)
  PHASE: A/B/C/D
  SEQ:   1-9
```

## Master Task Index

### SOL-LIM-001 — Distributed Cache & HA Scaling (7 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-001-A1 | Cache interface + LRU adapter | P0 | — | M |
| TASK-LIM-001-A2 | Redis cache adapter | P0 | A1 | M |
| TASK-LIM-001-A3 | Store cache factory + wiring | P0 | A1, A2 | M |
| TASK-LIM-001-A4 | PG NOTIFY cache invalidator | P1 | A3 | S |
| TASK-LIM-001-A5 | Cache Prometheus metrics | P1 | A1 | S |
| TASK-LIM-001-B1 | Leader election + runner wrapper | P0 | — | M |
| TASK-LIM-001-B2 | Server HA wiring | P0 | B1 | S |
| TASK-LIM-001-C1 | Read replica pool manager | P2 | A3 | M |

### SOL-LIM-002 — Persistent Message Bus (6 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-002-A1 | MessageBus interface + Channel adapter | P0 | — | M |
| TASK-LIM-002-A2 | PG outbox migration + PGBus | P0 | A1 | L |
| TASK-LIM-002-A3 | Bus factory + runner adaptation | P0 | A1, A2 | M |
| TASK-LIM-002-B1 | Bus Prometheus metrics | P1 | A1 | S |
| TASK-LIM-002-B2 | DLQ admin API | P1 | A2 | M |
| TASK-LIM-002-B3 | Outbox cleanup extension | P1 | A2 | S |

### SOL-LIM-003 — Embedded PG Migration Toolkit (5 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-003-A1 | Production readiness checker | P1 | — | M |
| TASK-LIM-003-A2 | Readiness API endpoint | P1 | A1 | S |
| TASK-LIM-003-B1 | Migration engine + CLI | P0 | — | L |
| TASK-LIM-003-C1 | PG health monitor | P1 | — | M |
| TASK-LIM-003-C2 | Backup scheduler | P2 | — | M |

### SOL-LIM-004 — Frontend Framework Unification (7 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-004-A1 | Runtime env config + token manager | P0 | — | M |
| TASK-LIM-004-A2 | Backend CORS + auth mode extension | P0 | — | M |
| TASK-LIM-004-A3 | Nginx config + Docker | P0 | A1, A2 | S |
| TASK-LIM-004-B1 | React Router shell + AuthGuard | P0 | A1 | M |
| TASK-LIM-004-B2 | Vue bridge component | P0 | B1 | M |
| TASK-LIM-004-C1 | Route migration pattern (template) | P1 | B1, B2 | S |
| TASK-LIM-004-D1 | Vue removal checklist | P2 | C1 | L |

### SOL-LIM-005 — Driver Feature Parity (6 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-005-A1 | Capability types + registry | P0 | — | M |
| TASK-LIM-005-A2 | Driver registration (all engines) | P0 | A1 | M |
| TASK-LIM-005-A3 | Refactor scattered checks + API | P1 | A1, A2 | M |
| TASK-LIM-005-B1 | ClickHouse SQL Advisor | P1 | A1 | L |
| TASK-LIM-005-B2 | BigQuery SQL Advisor | P2 | A1 | L |
| TASK-LIM-005-C1 | pgroll integration | P2 | A1 | L |

### SOL-LIM-006 — Feature Gate Rebalancing (5 tasks)

| Task ID | Title | Priority | Depends On | Est. |
|---------|-------|----------|------------|------|
| TASK-LIM-006-A1 | FREE instance limit + smart counting | P0 | — | S |
| TASK-LIM-006-A2 | TEAM audit log retention 90d | P0 | — | S |
| TASK-LIM-006-B1 | 2FA → TEAM tier | P0 | — | S |
| TASK-LIM-006-B2 | Basic password policy for TEAM | P1 | B1 | M |
| TASK-LIM-006-C1 | Dynamic Plan Policy Engine | P1 | A1, A2, B1 | L |

---

## Execution Priority Order

```
━━━ P0 — Critical Path (Sprint 1-2) ━━━
TASK-LIM-006-A1  →  FREE instance limit (trivial, high impact)
TASK-LIM-006-A2  →  Audit retention (trivial, compliance)
TASK-LIM-006-B1  →  2FA tier shift (trivial, user value)
TASK-LIM-001-A1  →  Cache interface (foundation)
TASK-LIM-001-A2  →  Redis adapter (depends A1)
TASK-LIM-001-A3  →  Store wiring (depends A1+A2)
TASK-LIM-001-B1  →  Leader election (independent)
TASK-LIM-001-B2  →  Server HA wiring (depends B1)
TASK-LIM-002-A1  →  Bus interface (foundation)
TASK-LIM-002-A2  →  PG outbox bus (depends A1)
TASK-LIM-002-A3  →  Runner adaptation (depends A1+A2)

━━━ P1 — High Value (Sprint 3-4) ━━━
TASK-LIM-005-A1  →  Capability registry
TASK-LIM-005-A2  →  Driver registration
TASK-LIM-004-A1  →  Frontend env config
TASK-LIM-004-A2  →  Backend CORS
TASK-LIM-004-B1  →  React Router shell
TASK-LIM-004-B2  →  Vue bridge
TASK-LIM-003-A1  →  Readiness checker
TASK-LIM-003-B1  →  Migration engine

━━━ P2 — Deferred (Sprint 5+) ━━━
TASK-LIM-001-C1  →  Read replica (if needed)
TASK-LIM-004-D1  →  Vue removal (after all routes migrated)
TASK-LIM-005-C1  →  pgroll (optional)
```

## Estimation Key

| Size | Lines of Code | Token Budget | Duration |
|------|--------------|--------------|----------|
| **S** | < 100 LoC | < 2K tokens | < 1h |
| **M** | 100-300 LoC | 2K-5K tokens | 1-3h |
| **L** | 300+ LoC | 5K-10K tokens | 3-8h |

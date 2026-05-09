# Tasks — AI Readiness Solutions

> Tác vụ được chia nhỏ từ 5 Solution proposals (SOL-AI-001→005), tối ưu cho token cost khi thực thi bởi AI agent.

## Task Naming Convention

```
TASK-AI-{SOL}-{SEQ} — {short-title}
  SOL: 001-005 (maps to SOL-AI-XXX)
  SEQ: 1-9
```

## Master Task Index

### SOL-AI-001 — Service & Model Decomposition (7 tasks)

| Task ID | Title | Priority | Depends On | Est. | Status |
|---------|-------|----------|------------|------|--------|
| TASK-AI-001-1 | auth_service.go split (→ 6 files) | P0 | — | M | ⏳ PENDING |
| TASK-AI-001-2 | sql_service.go split (→ 5 files) | P0 | — | M | ⏳ PENDING |
| TASK-AI-001-3 | rollout_service.go split (→ 4 files) | P1 | — | M | ⏳ PENDING |
| TASK-AI-001-4 | DCM services split (plan, issue, project) | P1 | — | M | ⏳ PENDING |
| TASK-AI-001-5 | Infrastructure services split (database, instance) | P1 | — | M | ⏳ PENDING |
| TASK-AI-001-6 | Store model split (database.go → 4 files) | P2 | — | M | ⏳ PENDING |
| TASK-AI-001-7 | CI file size lint enforcement | P2 | 001-1 | S | ⏳ PENDING |

### SOL-AI-002 — Interface-Based DI & Mocks (6 tasks)

| Task ID | Title | Priority | Depends On | Est. | Status |
|---------|-------|----------|------------|------|--------|
| TASK-AI-002-1 | Mock generation from store/interfaces.go | P0 | — | S | ⏳ PENDING |
| TASK-AI-002-2 | AuthService DI migration (3 interfaces) | P0 | 002-1, 001-1 | M | ⏳ PENDING |
| TASK-AI-002-3 | DatabaseService DI migration (3 interfaces) | P1 | 002-1 | M | ⏳ PENDING |
| TASK-AI-002-4 | SQLService + RolloutService DI (DataStore) | P1 | 002-1 | M | ⏳ PENDING |
| TASK-AI-002-5 | ACL interceptor DI (DataStore aggregate) | P1 | 002-1 | M | ⏳ PENDING |
| TASK-AI-002-6 | Server wiring update + compile-time checks | P0 | 002-2 | S | ⏳ PENDING |

### SOL-AI-003 — Build Profiles & Engine Matrix (5 tasks)

| Task ID | Title | Priority | Depends On | Est. | Status |
|---------|-------|----------|------------|------|--------|
| TASK-AI-003-1 | AI-CONTEXT comments in build-tagged files | P0 | — | S | ✅ DONE |
| TASK-AI-003-2 | BUILD_PROFILES.md documentation | P0 | — | S | ✅ DONE |
| TASK-AI-003-3 | Engine capability snapshot test | P0 | — | S | ✅ DONE |
| TASK-AI-003-4 | engine.go map refactor (11 switches → map) | P0 | 003-3 | M | ✅ DONE |
| TASK-AI-003-5 | DriverRegistry interface | P2 | 003-4 | S | ⏳ PENDING |

### SOL-AI-004 — Resource Name Simplification (4 tasks)

| Task ID | Title | Priority | Depends On | Est. | Status |
|---------|-------|----------|------------|------|--------|
| TASK-AI-004-1 | Typed resource ref structs | P1 | — | S | ✅ DONE |
| TASK-AI-004-2 | Parse + format functions | P1 | 004-1 | M | ✅ DONE |
| TASK-AI-004-3 | Backward-compatible deprecated wrappers | P1 | 004-2 | S | ⏳ PENDING |
| TASK-AI-004-4 | RESOURCE_NAMES.md documentation | P2 | 004-2 | S | ⏳ PENDING |

### SOL-AI-005 — EventBus Interface & ACL Contract (7 tasks)

| Task ID | Title | Priority | Depends On | Est. | Status |
|---------|-------|----------|------------|------|--------|
| TASK-AI-005-1 | EventBus interface + typed event structs | P1 | — | S | ✅ DONE |
| TASK-AI-005-2 | Bus concrete implementation refactor | P1 | 005-1 | M | ⏳ PENDING |
| TASK-AI-005-3 | Runner migration to EventBus interface | P1 | 005-2 | M | ⏳ PENDING |
| TASK-AI-005-4 | EVENT_FLOWS.md documentation | P2 | 005-2 | S | ⏳ PENDING |
| TASK-AI-005-5 | ACL static resource extractor map | P0 | — | L | ⏳ PENDING |
| TASK-AI-005-6 | ACL interceptor integration + fail-closed | P0 | 005-5 | M | ⏳ PENDING |
| TASK-AI-005-7 | ACL coverage test + CONTRACT.md | P0 | 005-6 | S | ⏳ PENDING |

---

## Progress Summary

| Metric | Value |
|--------|-------|
| Total tasks | 29 |
| ✅ Completed | 7 |
| ⏳ Pending | 22 |
| Completion | 24% |
| Last updated | 2026-05-09T19:35+07:00 |

## Completed Tasks Log

| Task | Date | Verification |
|------|------|-------------|
| TASK-AI-003-1 | 2026-05-09 | 5 files modified, `go build` passes |
| TASK-AI-003-2 | 2026-05-09 | `BUILD_PROFILES.md` created |
| TASK-AI-003-3 | 2026-05-09 | 25 engines × 10 capabilities tested, all PASS |
| TASK-AI-003-4 | 2026-05-09 | 493→210 LOC, `init()` exhaustiveness, all tests PASS |
| TASK-AI-004-1 | 2026-05-09 | 12 ref structs with String() methods, `go build` passes |
| TASK-AI-004-2 | 2026-05-09 | 8 parse functions, round-trip tests PASS |
| TASK-AI-005-1 | 2026-05-09 | EventBus interface defined, `go build` passes |

---

## Execution Priority Order

```
━━━ P0 — Foundation & Security (Sprint 1-2) ━━━
✅ TASK-AI-003-1  →  AI-CONTEXT comments (zero risk, instant context gain)
✅ TASK-AI-003-2  →  BUILD_PROFILES.md (zero risk, documentation)
✅ TASK-AI-003-3  →  Engine capability snapshot test (safety net for 003-4)
✅ TASK-AI-003-4  →  engine.go map refactor (493→210 LOC, biggest win/LOC)
⏳ TASK-AI-002-1  →  Mock generation (enables all DI tasks)
⏳ TASK-AI-001-1  →  auth_service split (highest LOC service)
⏳ TASK-AI-001-2  →  sql_service split (second highest LOC service)
⏳ TASK-AI-002-2  →  AuthService DI migration (depends 001-1, 002-1)
⏳ TASK-AI-002-6  →  Server wiring update (depends 002-2)
⏳ TASK-AI-005-5  →  ACL static extractor map (security-critical)
⏳ TASK-AI-005-6  →  ACL interceptor integration (security-critical)
⏳ TASK-AI-005-7  →  ACL coverage test (security verification)

━━━ P1 — Decomposition & DI Rollout (Sprint 3-4) ━━━
⏳ TASK-AI-001-3  →  rollout_service split
⏳ TASK-AI-001-4  →  DCM services split (plan, issue, project)
⏳ TASK-AI-001-5  →  Infrastructure services split (database, instance)
⏳ TASK-AI-002-3  →  DatabaseService DI migration
⏳ TASK-AI-002-4  →  SQLService + RolloutService DI
⏳ TASK-AI-002-5  →  ACL interceptor DI
✅ TASK-AI-004-1  →  Resource ref structs
✅ TASK-AI-004-2  →  Parse + format functions
⏳ TASK-AI-004-3  →  Deprecated wrappers
✅ TASK-AI-005-1  →  EventBus interface
⏳ TASK-AI-005-2  →  Bus concrete refactor
⏳ TASK-AI-005-3  →  Runner EventBus migration

━━━ P2 — Polish & Documentation (Sprint 5+) ━━━
⏳ TASK-AI-001-6  →  Store model split
⏳ TASK-AI-001-7  →  CI file size lint
⏳ TASK-AI-003-5  →  DriverRegistry interface
⏳ TASK-AI-004-4  →  RESOURCE_NAMES.md
⏳ TASK-AI-005-4  →  EVENT_FLOWS.md
```

## Estimation Key

| Size | Lines of Code | Token Budget | Duration |
|------|--------------|--------------|----------|
| **S** | < 100 LoC | < 2K tokens | < 1h |
| **M** | 100-300 LoC | 2K-5K tokens | 1-3h |
| **L** | 300+ LoC | 5K-10K tokens | 3-8h |

# Weakness Solutions Registry

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| Version        | v1                                         |
| Document Date  | 2026-05-09                                 |
| Source         | architecture.md + TDD.md → Solution specs  |

---

## Overview

Implementation solutions cho 8 weakness Change Requests. Mỗi solution bao gồm: architecture context analysis, root cause diagnosis, concrete code-level design, file change manifest, test strategy, và rollback plan.

**Key architectural proposals**: SOL-WEAK-007 và SOL-WEAK-008 đề xuất thay đổi kiến trúc significant — interface extraction cho testability và dual connection pool cho performance isolation.

---

## Solution Registry

| SOL ID | CR | Title | Arch Change | Files |
|--------|-----|-------|-------------|-------|
| [SOL-WEAK-001](SOL-WEAK-001-csp-hardening.md) | CR-WEAK-001 | CSP Nonce Middleware + Style Extraction | No | 6 |
| [SOL-WEAK-002](SOL-WEAK-002-cors-safety-guard.md) | CR-WEAK-002 | CORS Safety Guard + Configurable Origins | No | 4 |
| [SOL-WEAK-003](SOL-WEAK-003-error-handling-hardening.md) | CR-WEAK-003 | Error Propagation + Warning Pipeline | Minor (proto) | 6 |
| [SOL-WEAK-004](SOL-WEAK-004-service-modularization.md) | CR-WEAK-004 | Service File Decomposition | No (refactor) | 12+ |
| [SOL-WEAK-005](SOL-WEAK-005-composite-pk-guardrails.md) | CR-WEAK-005 | Unique ID Index + Query Validation | No | 6 |
| [SOL-WEAK-006](SOL-WEAK-006-jsonb-optimization.md) | CR-WEAK-006 | GIN Indexes + Generated Columns | No | 3 |
| [SOL-WEAK-007](SOL-WEAK-007-test-coverage-hardening.md) | CR-WEAK-007 | **Interface Extraction + Mock Infra** | **Yes** | 8+ |
| [SOL-WEAK-008](SOL-WEAK-008-connection-pool-optimization.md) | CR-WEAK-008 | **Dual Pool Architecture** | **Yes** | 7 |

---

## Architectural Changes Summary

### SOL-WEAK-007: Store Interface Extraction (Dependency Inversion)

```
BEFORE: L4 (Service) → *store.Store (concrete)
AFTER:  L4 (Service) → store.UserReader (interface)
                      → iam.PermissionChecker (interface)
        Wiring: L2 (server.go) injects concrete implementations
```

**Impact**: Enables unit testing without database. Services declare minimal interface dependencies.

### SOL-WEAK-008: Dual Connection Pool

```
BEFORE: All traffic → single *sql.DB (50 conn hard cap)
AFTER:  API requests → apiPool (70% of configurable max)
        Runners      → runnerPool (30% of configurable max)
        Both pools   → Prometheus metrics + alerts
```

**Impact**: API requests never blocked by heavy runner operations. Pool size auto-detected from PG max_connections.

---

## Layer Impact Matrix

| Solution | L1 | L2 | L3 | L4 | L5 | L6 | L7 | L8 | L9 | L10 |
|----------|----|----|----|----|----|----|----|----|----|----|
| SOL-001  | ● | ●  |    |    |    |    |    |    |    |     |
| SOL-002  |    | ●  |    |    |    |    |    |    |    | ●   |
| SOL-003  |    |    | ●  | ●  | ●  | ●  |    |    |    | ●   |
| SOL-004  |    |    |    | ●  |    |    |    |    |    |     |
| SOL-005  |    |    |    |    |    |    |    | ●  |    | ●   |
| SOL-006  |    |    |    |    |    |    |    | ●  |    | ●   |
| SOL-007  |    |    |    | ●  | ●  |    |    | ●  | ●  |     |
| SOL-008  |    | ●  |    |    |    | ●  |    | ●  |    | ●   |

---

## Execution Priority

### Wave 1 — P0 Critical
1. **SOL-WEAK-003** — Error handling (Sprint 1-3): Start immediately, highest impact on reliability

### Wave 2 — P1 High (parallel tracks)
2. **SOL-WEAK-002** — CORS guard (Sprint 1-2): Small scope, quick win
3. **SOL-WEAK-008** — Dual pool (Sprint 1-3): Architecture change, needs careful testing
4. **SOL-WEAK-007** — Test infra (Sprint 1-4): Foundation for all future quality
5. **SOL-WEAK-001** — CSP hardening (Sprint 1-4): Incremental, feature-flagged

### Wave 3 — P2-P3
6. **SOL-WEAK-004** — Service split (Sprint 1-4): One service per PR
7. **SOL-WEAK-005** — Composite PK (Sprint 1-3): Migration + validation
8. **SOL-WEAK-006** — JSONB optimization (Sprint 1-2): Low risk, low priority

---

> **Generated**: 2026-05-09 — Based on architecture.md, TDD.md, and source code analysis.

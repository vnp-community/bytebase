# Architecture Solutions Registry

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| Version        | v1                                         |
| Document Date  | 2026-05-09                                 |
| Source         | CR-ARCH-001 → CR-ARCH-011                 |
| Architecture   | `specs/architecture.md` (2026-05-08)       |
| TDD            | `specs/TDD.md` (2026-05-08)               |

---

## Overview

11 Solution specifications providing detailed implementation blueprints for each architecture Change Request. Solutions include Go code implementations, migration strategies, file manifests, and rollback plans — all grounded in current source code analysis.

---

## Solution Index

### P0 — Critical Solutions

| SOL ID | CR | Title | Affected Layers | New Deps |
|--------|-----|-------|----------------|----------|
| [SOL-ARCH-001](SOL-ARCH-001-store-interface-extraction.md) | CR-001 | Role-Based Interface Extraction + Mock Infra | L4, L5, L8 | `go.uber.org/mock` |
| [SOL-ARCH-007](SOL-ARCH-007-connection-pool-isolation.md) | CR-007 | Dual Pool Manager (API/Runner) | L2, L6, L8 | None |
| [SOL-ARCH-009](SOL-ARCH-009-resilience-patterns.md) | CR-009 | Circuit Breaker + Bulkhead + Retry + Rate Limiter | L4, L5, L6 | None |

### P1 — High Priority Solutions

| SOL ID | CR | Title | Affected Layers | New Deps |
|--------|-----|-------|----------------|----------|
| [SOL-ARCH-002](SOL-ARCH-002-durable-message-bus.md) | CR-002 | PG-Backed Queue + Channel Bridge | L5, L8 | None |
| [SOL-ARCH-004](SOL-ARCH-004-distributed-cache-layer.md) | CR-004 | Cache Abstraction + Redis/LRU/Noop | L8 | `github.com/redis/go-redis/v9` |
| [SOL-ARCH-008](SOL-ARCH-008-deep-health-check.md) | CR-008 | Component-Level Health + K8s Probes | L2 | None |

### P2 — Medium Priority Solutions

| SOL ID | CR | Title | Affected Layers | New Deps |
|--------|-----|-------|----------------|----------|
| [SOL-ARCH-003](SOL-ARCH-003-rest-gateway-direct-dispatch.md) | CR-003 | ConnectRPC bufconn (Eliminate TCP Loopback) | L2 | None (bufconn is in grpc) |
| [SOL-ARCH-005](SOL-ARCH-005-graceful-bootstrap.md) | CR-005 | Component Registry + Parallel Init | L2, L5, L10 | None |
| [SOL-ARCH-006](SOL-ARCH-006-frontend-migration.md) | CR-006 | Incremental Vue→React Page Migration | L1 | None |
| [SOL-ARCH-010](SOL-ARCH-010-service-decomposition.md) | CR-010 | Domain-Based File Split + CI Lint | L4 | None |

### P3 — Low Priority Solutions

| SOL ID | CR | Title | Affected Layers | New Deps |
|--------|-----|-------|----------------|----------|
| [SOL-ARCH-011](SOL-ARCH-011-plugin-build-tags.md) | CR-011 | Go Build Tags per Driver | L7 | None |

---

## Implementation Strategy Summary

### New Files Created

| Solution | New Files |
|----------|-----------|
| SOL-001 | `store/interfaces.go`, `store/mock/`, `iam/interfaces.go`, `enterprise/interfaces.go` |
| SOL-002 | `bus/durable_bus.go`, `bus/metrics.go`, migration SQL |
| SOL-003 | None (modify only) |
| SOL-004 | `store/cache/cache.go`, `store/cache/lru.go`, `store/cache/redis.go`, `store/cache/noop.go` |
| SOL-005 | `server/components.go`, `mcp/noop.go`, `lsp/noop.go` |
| SOL-006 | React shell, Zustand stores, Base UI components |
| SOL-007 | `store/pool_manager.go`, `store/pool_metrics.go` |
| SOL-008 | `server/health.go` |
| SOL-009 | `common/resilience/circuit_breaker.go`, `bulkhead.go`, `retry.go`, `rate_limiter.go` |
| SOL-010 | Split files `*_login.go`, `*_mfa.go`, etc. + CI script |
| SOL-011 | `plugin/db/all/all.go`, Makefile profiles |

### External Dependencies

| Dependency | Solutions | Status |
|-----------|----------|--------|
| `go.uber.org/mock` | SOL-001 | New (dev only) |
| `github.com/redis/go-redis/v9` | SOL-004 | New (optional, HA only) |
| `google.golang.org/grpc/test/bufconn` | SOL-003 | Existing (grpc module) |

### Feature Flags

| Flag | Solution | Default |
|------|----------|---------|
| `BUS_PERSISTENT_ENABLED` | SOL-002 | `false` |
| `CACHE_BACKEND` | SOL-004 | `lru` |
| `PG_POOL_ISOLATION` | SOL-007 | `false` |
| `CSP_NONCE_ENABLED` | (from SOL-WEAK-001) | `false` |

---

## Execution Dependency Graph

```
SOL-001 (Interfaces) ─── foundation for all services
    ↓
SOL-009 (Resilience) ─── independent utility library
    ↓
SOL-007 (Pool Isolation) ─── independent infra
    ↓
SOL-008 (Health Check) ← uses SOL-005 ComponentRegistry
    ↓
SOL-002 (Durable Bus) ─── independent infra
SOL-004 (Distributed Cache) ─── independent infra
SOL-005 (Graceful Bootstrap) ← depends on SOL-008 health
SOL-003 (REST Gateway) ─── independent (single file)
SOL-010 (Service Split) ─── independent (code movement)
SOL-006 (Frontend) ─── independent (long-term, L1 only)
SOL-011 (Build Tags) ─── independent (L7 only)
```

---

> **Generated**: 2026-05-09 — Based on architecture.md + TDD.md analysis aligned with source code.

# Task Registry — Gateway + Services Migration (Production-Grade)

| Metadata | Value |
|----------|-------|
| Total Tasks | 22 |
| Total Phases | 5 (Phase 0-4) |
| Estimated | 11-16 working days |
| Protocols | RESTful/ConnectRPC, NATS, gRPC |
| Grade | **Production / Enterprise** |

---

## Task Overview

### Phase 0: Pre-Migration (0.5 day)
| Task | Title | Status | Risk |
|------|-------|--------|------|
| [TASK-000](./TASK-000-pre-migration.md) | Pre-Migration Baseline | ✅ DONE | Low |

### Phase 1: NATS Bus + Transport + Production Infra (3-4 days)
| Task | Title | Status | Risk |
|------|-------|--------|------|
| [TASK-101](./TASK-101-nats-embedded.md) | Embed NATS Server + NATSBus | ✅ DONE | Medium |
| [TASK-102](./TASK-102-transport-layer.md) | Transport Abstraction (bufconn) | ✅ DONE | Low |
| [TASK-103](./TASK-103-service-interfaces.md) | Define Service Interfaces | ✅ DONE | Low |
| [TASK-104](./TASK-104-phase1-verify.md) | Phase 1 Verification | ✅ DONE | Low |
| [TASK-106](./TASK-106-observability.md) | Observability Infrastructure (OTel, Metrics, Middleware) | ✅ DONE | Low |
| [TASK-107](./TASK-107-resilience.md) | Resilience Infrastructure (Circuit Breaker, Retry, Config) | ✅ DONE | Low |

### Phase 2: Service Layer Extraction (3-4 days)
| Task | Title | Status | Risk |
|------|-------|--------|------|
| [TASK-201](./TASK-201-dcm-service.md) | Create DCM Service | ✅ DONE | Low |
| [TASK-202](./TASK-202-sql-service.md) | Create SQL Service | ✅ DONE | Low |
| [TASK-203](./TASK-203-admin-service.md) | Create Admin Service | ✅ DONE | Low |
| [TASK-204](./TASK-204-runner-service.md) | Create Runner Service | ✅ DONE | Low |
| [TASK-205](./TASK-205-service-registry.md) | Create ServiceRouter | ✅ DONE | Low |
| [TASK-206](./TASK-206-phase2-verify.md) | Phase 2 Verification | ✅ DONE | Low |

### Phase 3: Gateway + Server Refactor (2-3 days)
| Task | Title | Status | Risk |
|------|-------|--------|------|
| [TASK-301](./TASK-301-gateway-proxy.md) | Create Gateway HTTP Reverse Proxy | ✅ DONE | Medium |
| [TASK-302](./TASK-302-gateway-interceptors.md) | Extract Interceptor Chain | ✅ DONE | Low |
| [TASK-303](./TASK-303-refactor-grpc-routes.md) | Refactor grpc_routes.go | ✅ DONE | **Medium** |
| [TASK-304](./TASK-304-refactor-server.md) | Refactor server.go | ✅ DONE | **Medium** |
| [TASK-305](./TASK-305-phase3-verify.md) | Phase 3 Full Verification | ✅ DONE | Medium |

### Phase 4: Cleanup & Docs (1-2 days)
| Task | Title | Status | Risk |
|------|-------|--------|------|
| [TASK-401](./TASK-401-arch-test.md) | Architecture Boundary Tests | ✅ DONE | Low |
| [TASK-402](./TASK-402-update-docs.md) | Update architecture.md + TDD.md | ✅ DONE | Low |
| [TASK-403](./TASK-403-service-readme.md) | Developer Guide | ✅ DONE | Low |
| [TASK-404](./TASK-404-final-audit.md) | Final Audit & Tag | ✅ DONE | Low |

---

## Dependency Graph

```
TASK-000 (Baseline)
    │
    ├───────────────┬───────────────┬───────────────┐
    ▼               ▼               ▼               ▼
TASK-101 (NATS)  TASK-102 (Transport) TASK-103 (Interfaces) TASK-106 (Observability)
    │               │               │               │
    └───────────────┴───────────────┘               │
                    │                                ▼
              TASK-104 (Phase1 Verify)         TASK-107 (Resilience)
                    │                                │
    ┌───────────────┼───────────────┬────────────────┘
    ▼               ▼               ▼
TASK-201 (DCM)  TASK-202 (SQL)  TASK-203 (Admin)  ← parallel, include middleware
    │               │               │
    └───────────────┴───────────────┘
                    │
              TASK-204 (Runner) ── depends on TASK-101 (NATSBus)
                    │
              TASK-205 (ServiceRouter)
                    │
              TASK-206 (Phase2 Verify)
                    │
    ┌───────────────┼───────────────┐
    ▼               ▼               ▼
TASK-301 (Gateway+CB) TASK-302 (Interceptors)  ← includes circuit breaker
    │               │
    └───────────────┘
                    │
              TASK-303 (grpc_routes refactor)
                    │
              TASK-304 (server.go refactor)
                    │
              TASK-305 (Phase3 Verify — includes health aggregation)
                    │
    ┌───────────────┼───────────────┐
    ▼               ▼               ▼
TASK-401 (Test)  TASK-402 (Docs) TASK-403 (README)
    │               │               │
    └───────────────┴───────────────┘
                    │
              TASK-404 (Final Audit — SLO validation)
```

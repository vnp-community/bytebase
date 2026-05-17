# Migration Plan — Gateway + Services (Production-Grade)

## Overview

Migration từ monolith sang Gateway + Services, đạt chuẩn **Production-Grade & Enterprise-Level**:

- **RESTful/ConnectRPC** (via bufconn HTTP) — Services giữ nguyên existing handlers
- **NATS** (embedded) — Async events replace Go channels
- **Observability** — OpenTelemetry tracing, Prometheus metrics, structured logging
- **Resilience** — Circuit breaker, retry, timeout, graceful degradation
- **Security** — Internal HMAC auth, audit trail, secret management
- **Health** — Deep health checks, K8s-compatible probes, NATS monitoring

```
Phase 0         Phase 1              Phase 2          Phase 3          Phase 4
─────────       ─────────────        ─────────        ─────────        ─────────
Pre-Migration   NATS + Transport     Service Layer    Gateway Layer    Cleanup &
Baseline        + Production Infra   Extraction       Refactor         Documentation
(0.5 day)       (3-4 days)           (3-4 days)       (3-4 days)       (1-2 days)
```

**Total estimated time: 11-16 working days**

---

## Phase 0: Pre-Migration (0.5 day)

Thiết lập baseline — [TASK-000](./tasks/TASK-000-pre-migration.md)

---

## Phase 1: NATS Bus + Transport + Production Infra (3-4 days)

### Goal
Tạo NATSBus, Transport abstraction, Observability stack, Resilience infra. **Zero behavior change**.

### Tasks
| Task | Title |
|------|-------|
| [TASK-101](./tasks/TASK-101-nats-embedded.md) | Embed NATS Server + NATSBus |
| [TASK-102](./tasks/TASK-102-transport-layer.md) | Transport Abstraction Layer |
| [TASK-103](./tasks/TASK-103-service-interfaces.md) | Define Service Interfaces |
| [TASK-106](./tasks/TASK-106-observability.md) | Observability (OTel, Metrics, Middleware) |
| [TASK-107](./tasks/TASK-107-resilience.md) | Resilience (Circuit Breaker, Retry, Config) |
| [TASK-104](./tasks/TASK-104-phase1-verify.md) | Phase 1 Verification |

### Deliverables
- `backend/component/bus/nats_bus.go` — NATSBus implements EventBus
- `backend/transport/` — BufconnTransport
- `backend/service/service.go` — DomainService interface
- `backend/component/otel/` — OTel tracer
- `backend/component/metrics/` — ServiceMetrics (Prometheus)
- `backend/component/middleware/` — Production middleware stack
- `backend/component/errors/` — Standardized error codes
- `backend/gateway/circuitbreaker.go` — Circuit breakers
- `backend/component/config/` — ServiceConfig + FeatureFlags

---

## Phase 2: Service Layer Extraction (3-4 days)

### Goal
Tạo 3 domain services + Runner Service. Mỗi service chạy HTTP server trên bufconn với **production middleware stack**. **Code api/v1/ không thay đổi**.

### Tasks
| Task | Title |
|------|-------|
| [TASK-201](./tasks/TASK-201-dcm-service.md) | Create DCM Service |
| [TASK-202](./tasks/TASK-202-sql-service.md) | Create SQL Service |
| [TASK-203](./tasks/TASK-203-admin-service.md) | Create Admin Service |
| [TASK-204](./tasks/TASK-204-runner-service.md) | Create Runner Service |
| [TASK-205](./tasks/TASK-205-service-registry.md) | Create ServiceRouter / Registry |
| [TASK-206](./tasks/TASK-206-phase2-verify.md) | Phase 2 Verification |

### Deliverables
- `backend/service/dcm/` — 8 handlers + middleware + /internal/healthz
- `backend/service/sqlsvc/` — 8 handlers + middleware + /internal/healthz
- `backend/service/admin/` — 15 handlers + middleware + /internal/healthz
- `backend/service/runner/` — Background runners with NATSBus + panic recovery

---

## Phase 3: Gateway Layer Refactor (3-4 days)

### Goal
Gateway with HTTP reverse proxy, circuit breakers, health aggregation, OTel tracing.

### Tasks
| Task | Title |
|------|-------|
| [TASK-301](./tasks/TASK-301-gateway-proxy.md) | Create Gateway HTTP Reverse Proxy + Circuit Breaker |
| [TASK-302](./tasks/TASK-302-gateway-interceptors.md) | Extract Interceptor Chain |
| [TASK-303](./tasks/TASK-303-refactor-grpc-routes.md) | Refactor grpc_routes.go |
| [TASK-304](./tasks/TASK-304-refactor-server.md) | Refactor server.go |
| [TASK-305](./tasks/TASK-305-phase3-verify.md) | Phase 3 Full Verification + Health Aggregation |

### Deliverables
- `backend/gateway/` — HTTP reverse proxy + circuit breakers + OTel + health aggregation
- `backend/server/server.go` — Simplified (~50% fewer lines)
- `backend/server/grpc_routes.go` — Simplified (~80% fewer lines)
- `/healthz`, `/readyz`, `/livez` endpoints

---

## Phase 4: Cleanup & Documentation (1-2 days)

### Tasks
| Task | Title |
|------|-------|
| [TASK-401](./tasks/TASK-401-arch-test.md) | Architecture Boundary Tests |
| [TASK-402](./tasks/TASK-402-update-docs.md) | Update architecture.md + TDD.md |
| [TASK-403](./tasks/TASK-403-service-readme.md) | Developer Guide |
| [TASK-404](./tasks/TASK-404-final-audit.md) | Final Audit + SLO Validation |

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| ConnectRPC handler behind reverse proxy | High | Test early with 1 service |
| NATS binary size (~15MB) | Low | Acceptable trade-off |
| Circuit breaker false positive | Medium | Tune thresholds with real traffic |
| OTel tracing overhead | Low | Configurable sample rate |
| bufconn latency higher than direct | Low | ~10-50μs, negligible |

## Rollback

Each phase independently reversible. Feature flags allow gradual enable/disable.

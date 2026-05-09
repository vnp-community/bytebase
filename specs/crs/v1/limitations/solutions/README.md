# Solutions — Limitations Change Requests

| Metadata       | Value                                    |
|----------------|------------------------------------------|
| Category       | Limitation Solutions                     |
| Total Solutions| 6                                        |
| Document Date  | 2026-05-09                               |
| Status         | Proposed — All solutions pending review  |
| Arch Source    | `specs/architecture.md` (L1-L10)         |
| TDD Source     | `specs/TDD.md` (§1-§14)                 |

---

## Tổng quan

Thư mục này chứa **Solution proposals** cho các CR-LIM Change Requests. Mỗi solution được thiết kế dựa trên phân tích kiến trúc 10-layer (Architecture) và Technical Design Document (TDD), với các đề xuất thay đổi kiến trúc khi cần thiết.

---

## Solution Index

| Solution ID   | CR Ref      | Title                                    | Arch Changes | Key Layers |
|---------------|-------------|------------------------------------------|-------------|------------|
| SOL-LIM-001   | CR-LIM-001  | Distributed Cache & HA Scaling           | No          | L5, L6, L8 |
| SOL-LIM-002   | CR-LIM-002  | Persistent Message Bus (PG Outbox)       | Minor       | L5, L6, L8 |
| SOL-LIM-003   | CR-LIM-003  | Embedded PG Migration Toolkit            | No          | L4, L10    |
| SOL-LIM-004   | CR-LIM-004  | Frontend Framework Unification (CSR)     | **Major**   | L1, L2, L3 |
| SOL-LIM-005   | CR-LIM-005  | Driver Feature Parity (Capability Reg)   | **Major**   | L7, L4     |
| SOL-LIM-006   | CR-LIM-006  | Feature Gate Rebalancing (Policy Engine) | **Major**   | L9, L4     |

---

## Proposed Architecture Changes

Ba solutions đề xuất thay đổi kiến trúc đáng kể:

### 1. SOL-LIM-004 — L1/L2 Boundary: Standalone CSR Frontend

```
BEFORE: Go binary embeds SPA (server_frontend_embed.go)
AFTER:  Frontend deployed independently via Nginx/CDN
        Backend is API-only (CORS + Bearer token auth)
```

**Rationale**: Independent deployment cycles, CDN caching, consistent with VNP platform (Dify/Flowise CSR migration).

### 2. SOL-LIM-005 — L7: Driver Capability Registry

```
BEFORE: Scattered EngineSupportX() functions in backend/common/
AFTER:  Each driver declares DriverCapabilities at init() time
        API exposes capabilities per engine for feature matrix UI
```

**Rationale**: Single source of truth, self-documenting drivers, runtime-queryable capabilities.

### 3. SOL-LIM-006 — L9: Dynamic Plan Policy Engine

```
BEFORE: Static plan.yaml → hardcoded LicenseService checks
AFTER:  plan.yaml (defaults) → PlanPolicyEngine → DB settings (overrides)
        Self-hosted operators can customize feature gates without code changes
```

**Rationale**: Runtime configurability, A/B testing for SaaS, no binary rebuild for plan changes.

---

## Design Decisions Summary

| Decision | SOL | Rationale |
|----------|-----|-----------|
| PG Outbox over NATS for message bus | SOL-LIM-002 | Zero new dependency, PG transaction guarantee, sufficient throughput (<100 msg/s) |
| PG Advisory Locks over etcd for leader election | SOL-LIM-001 | Already in codebase, no new dependency, session-level auto-release |
| Standalone CSR over embedded SPA | SOL-LIM-004 | Independent deploy, CDN, consistent with VNP platform pattern |
| Capability Registry over scattered checks | SOL-LIM-005 | Centralized, self-documenting, API-queryable |
| pgroll over custom PG OSC | SOL-LIM-005 | Mature tool, expand-contract pattern, community maintained |
| Dynamic Policy Engine over static YAML | SOL-LIM-006 | Runtime configurable, no redeploy for plan changes |

---

## External Dependencies Introduced

| Dependency | Solution | Mode | Purpose |
|------------|----------|------|---------|
| Redis/Valkey 7+ | SOL-LIM-001 | HA only (optional) | Distributed cache |
| pgroll | SOL-LIM-005 | Optional | PG online schema change |

**Note**: SOL-LIM-002 intentionally avoids NATS dependency by using PG outbox pattern.

---

## Estimated Timeline

| Quarter   | Solutions                              | Focus                                    |
|-----------|----------------------------------------|------------------------------------------|
| Q3 2026   | SOL-LIM-001 (A,B), SOL-LIM-006 (A,B) | HA scaling + pricing quick wins          |
| Q4 2026   | SOL-LIM-002, SOL-LIM-003              | Reliability + deployment tooling         |
| Q1 2027   | SOL-LIM-005 (A,B), SOL-LIM-004 (A)   | Driver parity + frontend decoupling      |
| Q2 2027   | SOL-LIM-004 (B,C), SOL-LIM-005 (C)   | React migration + PG OSC                 |
| Q3 2027   | SOL-LIM-004 (D), SOL-LIM-006 (C)     | Vue removal + dynamic policy engine      |
| Q4 2027   | SOL-LIM-001 (C)                       | Read replica routing (if needed)         |

---

## File Structure

```
specs/crs/v1/limitations/solutions/
├── README.md                                        ← This file
├── SOL-LIM-001-distributed-cache-ha-scaling.md      ← Redis cache, leader election
├── SOL-LIM-002-persistent-message-bus.md            ← PG outbox, DLQ, metrics
├── SOL-LIM-003-embedded-pg-migration-toolkit.md     ← Migration CLI, health monitor
├── SOL-LIM-004-frontend-framework-unification.md    ← CSR decoupling, Vue→React
├── SOL-LIM-005-driver-feature-parity.md             ← Capability registry, advisors
└── SOL-LIM-006-feature-gate-rebalancing.md          ← Dynamic plan policy engine
```

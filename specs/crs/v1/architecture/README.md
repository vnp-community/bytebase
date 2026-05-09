# Architecture Change Request Registry

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| Version        | v1                                         |
| Document Date  | 2026-05-09                                 |
| Source         | Architecture Limitations & Weaknesses Analysis |
| PRD Reference  | `/docs/PRD.md` (2026-05-08)               |

---

## Overview

11 Change Requests to remediate 6 architectural limitations và 6 architectural weaknesses identified in the Bytebase modular monolith. CRs are mapped to PRD feature requirements and prioritized by enterprise impact.

---

## Change Request Index

### P0 — Critical (Must Fix)

| CR ID | Title | Source | Layer | PRD Refs |
|-------|-------|--------|-------|----------|
| [CR-ARCH-001](CR-ARCH-001-store-interface-extraction.md) | Store Interface Extraction | LIM-001 + WEAK-001 | L4-L8 | SEC-01, DCM-01 |
| [CR-ARCH-007](CR-ARCH-007-connection-pool-isolation.md) | Connection Pool Isolation | WEAK-002 | L6, L8 | DCM-01, SQL-01 |
| [CR-ARCH-009](CR-ARCH-009-resilience-patterns.md) | Resilience Patterns Infrastructure | WEAK-004 | L4-L6 | DCM-01, ADM-02 |

### P1 — High Priority

| CR ID | Title | Source | Layer | PRD Refs |
|-------|-------|--------|-------|----------|
| [CR-ARCH-002](CR-ARCH-002-durable-message-bus.md) | Durable Message Bus | LIM-002 | L5 | DCM-01, SEC-09 |
| [CR-ARCH-004](CR-ARCH-004-distributed-cache-layer.md) | Distributed Cache Layer | LIM-004 | L8 | SEC-01, DCM-01 |
| [CR-ARCH-008](CR-ARCH-008-deep-health-check.md) | Deep Health Check | WEAK-003 | L2 | ADM-08, SEC-10 |

### P2 — Medium Priority

| CR ID | Title | Source | Layer | PRD Refs |
|-------|-------|--------|-------|----------|
| [CR-ARCH-003](CR-ARCH-003-rest-gateway-direct-dispatch.md) | REST Gateway Direct Dispatch | LIM-003 | L2 | ADM-08 |
| [CR-ARCH-005](CR-ARCH-005-graceful-bootstrap.md) | Graceful Bootstrap | LIM-005 | L2→L10 | ADM-09, ADM-10 |
| [CR-ARCH-006](CR-ARCH-006-frontend-migration.md) | Frontend Migration (Vue→React) | LIM-006 | L1 | SQL-01, DCM-01 |
| [CR-ARCH-010](CR-ARCH-010-service-decomposition.md) | Service Layer Decomposition | WEAK-005 | L4 | DCM-01, SEC-01 |

### P3 — Low Priority

| CR ID | Title | Source | Layer | PRD Refs |
|-------|-------|--------|-------|----------|
| [CR-ARCH-011](CR-ARCH-011-plugin-build-tags.md) | Plugin Build Tag Isolation | WEAK-006 | L7 | ADM-08 |

---

## Traceability Matrix

| Limitation/Weakness | CR(s) | Priority |
|---------------------|-------|----------|
| ARCH-LIM-001 (God Store) | CR-ARCH-001 | P0 |
| ARCH-LIM-002 (Volatile Bus) | CR-ARCH-002 | P1 |
| ARCH-LIM-003 (Loopback Proxy) | CR-ARCH-003 | P2 |
| ARCH-LIM-004 (Cache-HA) | CR-ARCH-004 | P1 |
| ARCH-LIM-005 (Bootstrap) | CR-ARCH-005 | P2 |
| ARCH-LIM-006 (Dual Frontend) | CR-ARCH-006 | P2 |
| ARCH-WEAK-001 (No Interfaces) | CR-ARCH-001 | P0 |
| ARCH-WEAK-002 (Pool Contention) | CR-ARCH-007 | P0 |
| ARCH-WEAK-003 (Shallow Health) | CR-ARCH-008 | P1 |
| ARCH-WEAK-004 (No Resilience) | CR-ARCH-009 | P0 |
| ARCH-WEAK-005 (Service Bloat) | CR-ARCH-010 | P2 |
| ARCH-WEAK-006 (Binary Inflation) | CR-ARCH-011 | P3 |

---

## Execution Dependency Graph

```
CR-ARCH-001 (Store Interfaces) ──────────────────────┐
  ↓ enables mock testing                             │
CR-ARCH-009 (Resilience Patterns) ─── independent    │
  ↓ shared resilience library                        │
CR-ARCH-007 (Pool Isolation) ────── independent      │
  ↓ pool metrics feed into                           │
CR-ARCH-008 (Deep Health Check) ←────────────────────┘
  ↓ health check uses interfaces
CR-ARCH-002 (Durable Bus) ─── independent
CR-ARCH-004 (Distributed Cache) ─── independent
CR-ARCH-005 (Graceful Bootstrap) ← depends on CR-ARCH-008
CR-ARCH-003 (REST Gateway) ─── independent
CR-ARCH-010 (Service Decomposition) ─── independent
CR-ARCH-006 (Frontend Migration) ─── independent (long-term)
CR-ARCH-011 (Build Tags) ─── independent
```

---

## Recommended Execution Order

| Sprint | CRs | Focus |
|--------|-----|-------|
| Sprint 1-2 | CR-ARCH-001, CR-ARCH-009, CR-ARCH-007 | Foundation: interfaces + resilience + pool |
| Sprint 2-3 | CR-ARCH-008, CR-ARCH-010 | Observability + maintainability |
| Sprint 3-4 | CR-ARCH-002, CR-ARCH-004 | HA infrastructure |
| Sprint 4-5 | CR-ARCH-003, CR-ARCH-005 | Performance + reliability |
| Sprint 5-9 | CR-ARCH-006 | Frontend migration (long-term) |
| Sprint 5-6 | CR-ARCH-011 | Build optimization |

---

## PRD Feature Coverage

| PRD Feature | CRs that improve it |
|-------------|---------------------|
| DCM-01 (Change Workflow) | CR-001, 002, 007, 009, 010 |
| DCM-09 (Batch Changes) | CR-002, 007, 009 |
| SEC-01 (IAM) | CR-001, 004, 010 |
| SEC-09 (Approval Workflow) | CR-002 |
| SQL-01 (SQL Editor) | CR-006, 007 |
| ADM-08 (API Integration) | CR-001, 003, 004, 008, 011 |
| ADM-09 (MCP Server) | CR-005 |
| ADM-10 (LSP Server) | CR-005 |
| ADM-02 (IM Notifications) | CR-009 |
| SEC-10 (Audit Log) | CR-008 |

---

> **Generated**: 2026-05-09 — Based on architecture limitation analysis aligned with PRD v2026-05-08.

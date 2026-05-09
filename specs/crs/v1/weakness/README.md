# Bytebase — Weakness Change Requests Registry

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| Version        | v1                                         |
| Document Date  | 2026-05-08                                 |
| Source         | Weakness analysis → Change Request specs   |

---

## Overview

Change Requests để khắc phục 8 weaknesses đã được xác định trong [weakness analysis](weakness/). Mỗi CR bao gồm: functional requirements, technical design, test cases, rollout plan, và risk assessment.

---

## Change Request Registry

| CR ID        | Weakness    | Title                                       | Priority | Category          | Sprints |
|--------------|-------------|---------------------------------------------|----------|-------------------|---------|
| [CR-WEAK-001](CR-WEAK-001-csp-hardening.md)          | WEAK-001 | CSP Security Hardening                     | P1 High  | Security          | 4       |
| [CR-WEAK-002](CR-WEAK-002-cors-safety-guard.md)      | WEAK-002 | CORS Safety Guard & Production Protection  | P1 High  | Security          | 2       |
| [CR-WEAK-003](CR-WEAK-003-error-handling-hardening.md)| WEAK-003 | Error Handling Hardening                   | P0 Crit  | Reliability       | 3       |
| [CR-WEAK-004](CR-WEAK-004-service-modularization.md)  | WEAK-004 | Service Layer Modularization               | P2 Med   | Maintainability   | 4       |
| [CR-WEAK-005](CR-WEAK-005-composite-pk-guardrails.md) | WEAK-005 | Composite PK Guardrails                    | P2 Med   | Database Design   | 3       |
| [CR-WEAK-006](CR-WEAK-006-jsonb-optimization.md)      | WEAK-006 | JSONB Query Optimization                   | P3 Low   | Data Access       | 2       |
| [CR-WEAK-007](CR-WEAK-007-test-coverage-hardening.md) | WEAK-007 | Test Coverage Hardening                    | P1 High  | Quality Assurance | 4       |
| [CR-WEAK-008](CR-WEAK-008-connection-pool-optimization.md)| WEAK-008 | Connection Pool Optimization          | P1 High  | Performance       | 3       |

---

## Priority Execution Order

### Wave 1 — P0 Critical (Sprint 1-3)
1. **CR-WEAK-003** — Error Handling Hardening
   - IAM silent failures → 503 instead of false 403
   - Migration executor warning propagation
   - Blanket nolint replacement

### Wave 2 — P1 High (Sprint 1-4)
2. **CR-WEAK-002** — CORS Safety Guard (Sprint 1-2)
3. **CR-WEAK-008** — Connection Pool Optimization (Sprint 1-3)
4. **CR-WEAK-007** — Test Coverage Hardening (Sprint 1-4)
5. **CR-WEAK-001** — CSP Security Hardening (Sprint 1-4)

### Wave 3 — P2-P3 Medium/Low (Sprint 2-4)
6. **CR-WEAK-004** — Service Modularization (Sprint 1-4)
7. **CR-WEAK-005** — Composite PK Guardrails (Sprint 1-3)
8. **CR-WEAK-006** — JSONB Query Optimization (Sprint 1-2)

---

## PRD Feature Cross-Reference

| CR           | PRD Features Affected                                          |
|--------------|----------------------------------------------------------------|
| CR-WEAK-001  | SEC-01 (IAM), SEC-15 (Data Masking), SQL-01 (SQL Editor)      |
| CR-WEAK-002  | SEC-01 (IAM), ADM-08 (API Integration)                        |
| CR-WEAK-003  | SEC-01 (IAM), SEC-10 (Audit Log), DCM-12 (Changelog)          |
| CR-WEAK-004  | All services — cross-cutting refactor                          |
| CR-WEAK-005  | DCM-01 (Issue/Plan/Rollout), DCM-12 (Changelog)               |
| CR-WEAK-006  | DCM-01 (Plan/Issue), SEC-10 (Audit Log)                        |
| CR-WEAK-007  | All — cross-cutting quality requirement                        |
| CR-WEAK-008  | ADM-08 (API Integration), DCM-01 (Change Management)           |

---

## Impact Summary

| Dimension            | Before                           | After                              |
|----------------------|----------------------------------|-------------------------------------|
| **Security**         | CSP unsafe-inline, CORS wildcard | Nonce-based CSP, configurable CORS |
| **Reliability**      | Silent IAM failures, error swallowing | Proper error propagation, 503 errors |
| **Maintainability**  | 78KB service files               | ≤40KB per file, SRP compliant      |
| **Quality**          | <30% test coverage               | ≥60% service, ≥70% store          |
| **Performance**      | 50 conn hard cap, no metrics     | Auto-scaled pool, full observability|
| **Data Integrity**   | No PK guardrails                 | Unique ID index, query validation  |

---

> **Generated**: 2026-05-08 — Based on weakness analysis of vnp-bytebase repository and PRD v1 feature matrix.

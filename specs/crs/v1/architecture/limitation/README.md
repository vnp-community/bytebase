# Architecture Limitations & Weaknesses Registry

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| Version        | v1                                         |
| Document Date  | 2026-05-09                                 |
| Source         | architecture.md + TDD.md + Source code analysis |

---

## Overview

Phân tích kiến trúc modular monolith của Bytebase dựa trên mô hình 10 layers (L1-L10), xác định 12 điểm giới hạn và yếu kém cấu trúc ảnh hưởng đến khả năng mở rộng, bảo trì, và vận hành enterprise.

---

## Classification

### Limitations (Giới hạn cấu trúc — trade-off có chủ đích)

| ID | Title | Layer | Impact |
|----|-------|-------|--------|
| [ARCH-LIM-001](ARCH-LIM-001-god-store-coupling.md) | God Object Store — Central Coupling | L8 | Testability, Modularity |
| [ARCH-LIM-002](ARCH-LIM-002-volatile-message-bus.md) | Volatile In-Process Message Bus | L5 | Reliability, HA |
| [ARCH-LIM-003](ARCH-LIM-003-rest-gateway-loopback.md) | REST Gateway Loopback Proxy | L2 | Performance, Complexity |
| [ARCH-LIM-004](ARCH-LIM-004-cache-ha-incompatibility.md) | Cache-HA Mutual Exclusion | L8 | Performance, Scaling |
| [ARCH-LIM-005](ARCH-LIM-005-monolith-bootstrap.md) | Monolithic Bootstrap Coupling | L2→L10 | Startup, Resilience |
| [ARCH-LIM-006](ARCH-LIM-006-dual-frontend-framework.md) | Dual Frontend Framework (Vue+React) | L1 | Complexity, Bundle Size |

### Weaknesses (Điểm yếu kiến trúc — cần sửa)

| ID | Title | Layer | Severity |
|----|-------|-------|----------|
| [ARCH-WEAK-001](ARCH-WEAK-001-no-interface-contracts.md) | No Interface Contracts in Core Layers | L4-L8 | High |
| [ARCH-WEAK-002](ARCH-WEAK-002-runner-resource-contention.md) | Runner/API Resource Contention | L6, L8 | High |
| [ARCH-WEAK-003](ARCH-WEAK-003-shallow-health-check.md) | Shallow Health Check | L2 | Medium |
| [ARCH-WEAK-004](ARCH-WEAK-004-no-resilience-patterns.md) | Missing Resilience Patterns | L4-L6 | High |
| [ARCH-WEAK-005](ARCH-WEAK-005-service-layer-bloat.md) | Service Layer Bloat | L4 | Medium |
| [ARCH-WEAK-006](ARCH-WEAK-006-plugin-binary-inflation.md) | Plugin Binary Inflation | L7 | Medium |

---

## Layer Distribution

```
         Limitations    Weaknesses
L1          1              —
L2          2              1
L4          —              2
L5          1              —
L6          —              1
L7          —              1
L8          2              1
Cross       —              1
```

---

## Impact Matrix

| Concern         | LIM-001 | LIM-002 | LIM-003 | LIM-004 | LIM-005 | LIM-006 | WEAK-001 | WEAK-002 | WEAK-003 | WEAK-004 | WEAK-005 | WEAK-006 |
|----------------|:-------:|:-------:|:-------:|:-------:|:-------:|:-------:|:--------:|:--------:|:--------:|:--------:|:--------:|:--------:|
| Scalability    |    ●    |    ●    |    ●    |    ●    |         |         |          |    ●     |          |          |          |    ●     |
| Testability    |    ●    |         |         |         |         |         |    ●     |          |          |          |    ●     |          |
| Reliability    |         |    ●    |         |         |    ●    |         |          |          |    ●     |    ●     |          |          |
| Maintainability|    ●    |         |    ●    |         |    ●    |    ●    |    ●     |          |          |          |    ●     |    ●     |
| Performance    |         |         |    ●    |    ●    |         |    ●    |          |    ●     |          |          |          |          |

---

> **Generated**: 2026-05-09 — Based on L1-L10 layered architecture analysis.

# Bytebase Frontend — AI Development Issues Index

> **Version**: 1.0.0  
> **Date**: 2026-05-13  
> **Scope**: Điểm yếu và giới hạn khi sử dụng AI phục vụ phát triển frontend

---

## Tổng Quan

Phân tích codebase frontend Bytebase (307K LOC, 1465 files, hybrid Vue+React) đã xác định **10 issues** ảnh hưởng đến hiệu quả sử dụng AI cho phát triển.

## Severity Matrix

| ID | Issue | Severity | Category |
|---|---|---|---|
| [ISS-AI-001](./ISS-AI-001-hybrid-framework-complexity.md) | Hybrid Vue + React Framework | 🔴 Critical | Architecture |
| [ISS-AI-002](./ISS-AI-002-protobuf-generated-code-volume.md) | Proto-ES Generated Code Volume (~38K LOC) | 🟠 High | Context Window |
| [ISS-AI-003](./ISS-AI-003-massive-god-components.md) | God Components (18 files >1000 LOC) | 🟠 High | Code Complexity |
| [ISS-AI-004](./ISS-AI-004-complex-state-topology.md) | Multi-layer State Topology (4 layers) | 🟠 High | State Management |
| [ISS-AI-005](./ISS-AI-005-codebase-scale-context-limit.md) | Codebase Scale ~307K LOC | 🔴 Critical | Scale |
| [ISS-AI-006](./ISS-AI-006-non-standard-patterns.md) | Non-standard Patterns | 🟠 High | Conventions |
| [ISS-AI-007](./ISS-AI-007-connectrpc-interceptor-complexity.md) | ConnectRPC + Interceptor Chain | 🟡 Medium | API Layer |
| [ISS-AI-008](./ISS-AI-008-dual-i18n-systems.md) | Dual i18n Systems | 🟡 Medium | i18n |
| [ISS-AI-009](./ISS-AI-009-routing-complexity.md) | Routing Complexity (100+ routes) | 🟡 Medium | Routing |
| [ISS-AI-010](./ISS-AI-010-domain-knowledge-gap.md) | Domain Knowledge Gap | 🟡 Medium | Domain |

## Impact Summary

```
Code Generation Accuracy:  ██████░░░░  ~60% (do hybrid framework + non-standard patterns)
Refactoring Safety:        ████░░░░░░  ~40% (do codebase scale + cross-file dependencies)
Bug Fix Precision:         █████░░░░░  ~50% (do god components + state complexity)
Feature Development:       ██████░░░░  ~60% (do domain knowledge gap + routing)
```

## Top Recommendations

1. **AI Context Files**: Tạo `.ai-context/` per domain với module maps, API cheat sheets, type summaries.
2. **Component Decomposition**: Tách 18 god components thành smaller units (<500 LOC).
3. **State Simplification**: Reduce `useVueState` calls bằng cách migrate top-usage stores sang Zustand.
4. **Generated Code Tagging**: Mark proto-es + openapi-index (~68K LOC) as AI-excludable.
5. **Scaffold Templates**: Provide "new page", "new sheet", "new API call" templates cho AI agents.

# Frontend Limitations — Task Registry

> **Version**: 1.0.0 | **Date**: 2026-05-14  
> **Source**: [Solutions Registry](../solutions/README.md)

---

## Sprint 1 — Critical/Security (P1)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-L-001 | SOL-003 §2.1 | [Resilient Lock Pattern](./TASK-L-001.md) | 1 file | 3h | — |
| TASK-L-002 | SOL-003 §2.2 | [Idempotency-Aware Auth Retry](./TASK-L-002.md) | 1 file | 2h | — |
| TASK-L-003 | SOL-003 §2.3 | [Encrypted Refresh Token](./TASK-L-003.md) | 1 file | 3h | — |
| TASK-L-004 | SOL-004 §2.1 | [React ErrorBoundary Component](./TASK-L-004.md) | 1 file (new) | 1.5h | — |
| TASK-L-005 | SOL-004 §2.2 | [Integrate ErrorBoundary in mount.ts](./TASK-L-005.md) | 1 file | 1h | L-004 |
| TASK-L-006 | SOL-004 §2.3 | [Refined ConnectError Filter](./TASK-L-006.md) | 1 file | 1h | — |
| TASK-L-007 | SOL-004 §2.4 | [OAuth Listener Fix + Unhandled Rejection](./TASK-L-007.md) | 1 file | 1h | — |
| TASK-L-008 | SOL-006 §2.2 | [Redirect URL Validator](./TASK-L-008.md) | 2 files | 1.5h | — |
| TASK-L-009 | SOL-006 §2.3 | [Sync Store Reset on Auth Routes](./TASK-L-009.md) | 1 file | 0.5h | — |

## Sprint 2 — High Impact (P2)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-L-010 | SOL-001 §2.1 | [Render Versioning in ReactPageMount](./TASK-L-010.md) | 1 file | 3h | — |
| TASK-L-011 | SOL-001 §2.2 | [Cleanup Lifecycle Hardening](./TASK-L-011.md) | 1 file | 0.5h | L-010 |
| TASK-L-012 | SOL-001 §2.3 | [Defensive buildTree in mount.ts](./TASK-L-012.md) | 1 file | 0.5h | — |
| TASK-L-013 | SOL-002 §2.2 | [LRUEntityCache Class](./TASK-L-013.md) | 1 file | 3h | — |
| TASK-L-014 | SOL-002 §2.1+2.3 | [Namespace Tier Map + useCache Update](./TASK-L-014.md) | 1 file | 2h | L-013 |
| TASK-L-015 | SOL-002 §2.4 | [Dev-Mode Cache Monitor](./TASK-L-015.md) | 1 file (new) | 1h | L-013 |
| TASK-L-016 | SOL-006 §2.1 | [Route-Scoped Query Preservation](./TASK-L-016.md) | 2 files | 2h | — |

## Sprint 3 — Medium Impact (P3)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-L-017 | SOL-005 §2.1 | [Browser Baseline Upgrade](./TASK-L-017.md) | 1 file | 0.5h | — |
| TASK-L-018 | SOL-005 §2.2 | [Simplify WeakRef Polyfill](./TASK-L-018.md) | 1 file | 0.5h | L-017 |
| TASK-L-019 | SOL-005 §2.3 | [Legacy Plugin Target Update](./TASK-L-019.md) | 1 file | 0.5h | L-017 |
| TASK-L-020 | SOL-007 §1.1 | [LocaleManager Singleton](./TASK-L-020.md) | 1 file (new) | 2h | — |
| TASK-L-021 | SOL-007 §1.1 | [Wire Vue i18n + React i18next](./TASK-L-021.md) | 2 files | 1.5h | L-020 |
| TASK-L-022 | SOL-007 §1.2 | [i18n Key Sync CI Script](./TASK-L-022.md) | 1 file (new) | 2h | — |
| TASK-L-023 | SOL-008 §1.1 | [Bundle Size Budget CI Gate](./TASK-L-023.md) | 1 file (new) | 2h | — |
| TASK-L-024 | SOL-008 §1.3 | [Dynamic Chunk Strategy](./TASK-L-024.md) | 1 file | 1h | — |
| TASK-L-025 | SOL-008 §1.5 | [React TSX Transform tsconfig](./TASK-L-025.md) | 1 file | 0.5h | — |

---

## Summary

```
Total tasks:       25
Total effort:      ~36h (~4.5 developer-days)
Sprint 1 (P1):      9 tasks — 14.5h
Sprint 2 (P2):      7 tasks — 12h
Sprint 3 (P3):      9 tasks — 10.5h
```

## Design Principles

1. **Self-contained** — Mỗi task liệt kê chính xác files và lines cần thay đổi
2. **Atomic** — 1-2 files per task, giảm context window
3. **Precise diffs** — Code mẫu sẵn sàng apply
4. **Explicit AC** — Acceptance Criteria rõ ràng, testable
5. **Dependency chain** — Tasks có deps phải thực thi theo thứ tự

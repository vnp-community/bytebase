# Frontend Weakness Tasks — Registry

> **Version**: 1.0.0 | **Date**: 2026-05-14  
> **Source**: [Solutions Registry](../solutions/README.md)

---

## Task Registry

### Sprint 1 — Security & Stability (P1)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-W-001 | SOL-003 §3.1 | [Open Redirect Validation](./TASK-W-001.md) | 2 files | 1.5h | — |
| TASK-W-002 | SOL-003 §3.2-3.3 | [Auth Error Transparency & Logout Retry](./TASK-W-002.md) | 1 file | 1.5h | — |
| TASK-W-003 | SOL-003 §3.4-3.5 | [Token Refresh Resilience](./TASK-W-003.md) | 1 file | 2.5h | — |
| TASK-W-004 | SOL-003 §3.6 | [OAuth Listener Lifecycle](./TASK-W-004.md) | 1 file | 0.5h | — |
| TASK-W-005 | SOL-001 §3.3 | [React Error Boundary](./TASK-W-005.md) | 1 file (new) | 1.5h | — |
| TASK-W-006 | SOL-001 §3.4 | [Bridge Type Definitions](./TASK-W-006.md) | 1 file (new) | 1.5h | — |
| TASK-W-007 | SOL-001 §3.1 | [BridgeLifecycleManager](./TASK-W-007.md) | 1 file (new) | 3h | W-005, W-006 |
| TASK-W-008 | SOL-001 §3.2 | [Refactor ReactPageMount.vue](./TASK-W-008.md) | 1 file | 3h | W-007 |
| TASK-W-009 | SOL-001 §3.5 | [Page Cache Cleanup on Logout](./TASK-W-009.md) | 2 files | 1h | W-007 |
| TASK-W-010 | SOL-001 §Phase6 | [Sidebar Bridge Migration](./TASK-W-010.md) | 2 files | 2h | W-007 |

### Sprint 2 — State & Data Integrity (P2)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-W-011 | SOL-002 §3.1-3.2 | [Cache Entry Timestamps](./TASK-W-011.md) | 1 file | 2h | — |
| TASK-W-012 | SOL-002 §3.3 | [Cache Eviction Engine](./TASK-W-012.md) | 1 file (new) | 2.5h | W-011 |
| TASK-W-013 | SOL-002 §3.4 | [Enhanced useCache Hook](./TASK-W-013.md) | 1 file | 3h | W-011, W-012 |
| TASK-W-014 | SOL-002 §3.5 | [Consolidate Database Triple Cache](./TASK-W-014.md) | 1 file | 2.5h | W-013 |
| TASK-W-015 | SOL-004 §3.1 | [useVueState Deep Default](./TASK-W-015.md) | 1 file | 0.5h | — |
| TASK-W-016 | SOL-004 §3.2-3.3 | [Refactor useAppState to Pinia Proxy](./TASK-W-016.md) | 1 file | 2.5h | W-015 |
| TASK-W-017 | SOL-004 §3.4 | [Typed Shell Bridge](./TASK-W-017.md) | 1 file | 2h | — |

### Sprint 3 — Quality & Reliability (P3)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-W-018 | SOL-005 §2.1 | [Scoped ConnectError Suppression](./TASK-W-018.md) | 1 file | 1.5h | — |
| TASK-W-019 | SOL-005 §2.2 | [NotFound Default Notification](./TASK-W-019.md) | 1 file | 1h | — |
| TASK-W-020 | SOL-005 §2.4 | [Replace Empty Catches](./TASK-W-020.md) | 6 files | 2h | — |
| TASK-W-021 | SOL-008 §2.1 | [Guard Pipeline Architecture](./TASK-W-021.md) | 1 file (new) | 3h | — |
| TASK-W-022 | SOL-008 §2.2 | [Route Registry Whitelist](./TASK-W-022.md) | 2 files | 2.5h | W-021 |
| TASK-W-023 | SOL-008 §2.3 | [Lazy Store Reset](./TASK-W-023.md) | 2 files | 1.5h | W-021, W-002 |
| TASK-W-024 | SOL-008 §2.4 | [OAuth Consent Guard](./TASK-W-024.md) | 1 file (new) | 1h | W-021 |
| TASK-W-025 | SOL-008 §2.5 | [Unified Title Manager](./TASK-W-025.md) | 2 files | 1.5h | — |

### Sprint 4 — Optimization (P3)

| ID | Source | Tên | Scope | Effort | Deps |
|---|---|---|---|---|---|
| TASK-W-026 | SOL-006 §2.1 | [Progressive Bootstrap](./TASK-W-026.md) | 3 files | 3.5h | — |
| TASK-W-027 | SOL-006 §2.3 | [Strip Console.debug in Build](./TASK-W-027.md) | 1 file | 0.5h | — |
| TASK-W-028 | SOL-006 §2.4 | [Optimize Route Watcher](./TASK-W-028.md) | 1 file | 1h | — |
| TASK-W-029 | SOL-006 §2.2 | [Merge i18n Shared Namespace](./TASK-W-029.md) | 3+ files | 5h | — |
| TASK-W-030 | SOL-006 §2.5 | [Lazy Monaco Loading](./TASK-W-030.md) | 2 files | 1.5h | — |
| TASK-W-031 | SOL-007 §2.1 | [StorageService Core](./TASK-W-031.md) | 1 file (new) | 3.5h | — |
| TASK-W-032 | SOL-007 §2.2 | [PII-Free Storage Keys](./TASK-W-032.md) | 1 file | 1.5h | W-031 |
| TASK-W-033 | SOL-007 §2.3 | [Encrypted Token Storage](./TASK-W-033.md) | 1 file | 2h | W-031 |
| TASK-W-034 | SOL-007 §2.4 | [Migrate High-Traffic localStorage](./TASK-W-034.md) | 10+ files | 5h | W-031 |

---

## Summary

```
Total tasks:       34
Total effort:      ~65h (~8.5 developer-days)
Sprint 1 (P1):     10 tasks — 18h
Sprint 2 (P2):      7 tasks — 15h  
Sprint 3 (P3):      8 tasks — 14h
Sprint 4 (P3):      9 tasks — 23.5h
```

## Token Optimization Strategy

Mỗi task được thiết kế để **tối thiểu token**:
1. **Self-contained context** — chỉ liệt kê files và lines cần thay đổi
2. **Precise diffs** — code mẫu sẵn sàng apply, không cần đọc thêm context
3. **No cross-references** — mỗi task có đủ thông tin, không cần đọc solution docs
4. **Atomic scope** — 1-2 files per task, giảm context window
5. **Explicit AC** — Acceptance Criteria rõ ràng, không ambiguity

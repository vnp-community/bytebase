# Frontend Weakness & Bug Registry — Index

> **Version**: 1.0.0 | **Date**: 2026-05-13  
> **Scope**: Bytebase Frontend (`vnp-bytebase/frontend/`)

---

## Tổng quan

Tài liệu này liệt kê các điểm yếu (weaknesses) và lỗi tiềm ẩn (latent bugs) được phát hiện qua phân tích static code của frontend Bytebase. Các issues được phân loại theo severity và category.

---

## Bug Registry

| ID | Severity | Category | Tên | File |
|---|---|---|---|---|
| **BUG-WEAK-001** | 🔴 HIGH | Architecture | [Vue ↔ React Bridge Race Conditions & Memory Leaks](./BUG-WEAK-001-bridge-race-conditions.md) | `mount.ts`, `ReactPageMount.vue` |
| **BUG-WEAK-002** | 🟠 MEDIUM-HIGH | State Mgmt | [Entity Cache Unbounded Growth & Stale Data](./BUG-WEAK-002-cache-unbounded-growth.md) | `cache.ts`, `database.ts` |
| **BUG-WEAK-003** | 🔴 HIGH | Security | [Authentication & Security Vulnerabilities](./BUG-WEAK-003-auth-security-gaps.md) | `auth.ts`, `refreshToken.ts`, `router/index.ts` |
| **BUG-WEAK-004** | 🟠 MEDIUM-HIGH | State Mgmt | [Dual Framework State Synchronization Issues](./BUG-WEAK-004-state-sync-dual-framework.md) | `useVueState.ts`, `useAppState.ts` |
| **BUG-WEAK-005** | 🟡 MEDIUM | Error Handling | [Error Handling Anti-Patterns](./BUG-WEAK-005-error-handling-antipatterns.md) | `App.vue`, `errorNotificationMiddleware.ts` |
| **BUG-WEAK-006** | 🟡 MEDIUM | Performance | [Performance & Bundle Weaknesses](./BUG-WEAK-006-performance-bundle.md) | `main.ts`, `App.vue`, `locales/` |
| **BUG-WEAK-007** | 🟡 MEDIUM | Security/Data | [localStorage Security & Fragmentation](./BUG-WEAK-007-localstorage-security.md) | `web-storage.ts`, `token-manager.ts` |
| **BUG-WEAK-008** | 🟡 MEDIUM | Navigation | [Router Guard Complexity & Edge Cases](./BUG-WEAK-008-router-guard-complexity.md) | `router/index.ts` |

---

## Severity Distribution

```
🔴 HIGH          : 2 (BUG-WEAK-001, BUG-WEAK-003)
🟠 MEDIUM-HIGH   : 2 (BUG-WEAK-002, BUG-WEAK-004)
🟡 MEDIUM        : 4 (BUG-WEAK-005, BUG-WEAK-006, BUG-WEAK-007, BUG-WEAK-008)
```

## Category Distribution

```
Security         : 2 (BUG-WEAK-003, BUG-WEAK-007)
State Management : 2 (BUG-WEAK-002, BUG-WEAK-004)
Architecture     : 1 (BUG-WEAK-001)
Error Handling   : 1 (BUG-WEAK-005)
Performance      : 1 (BUG-WEAK-006)
Navigation       : 1 (BUG-WEAK-008)
```

---

## Priority Remediation Order

1. **BUG-WEAK-003** — Security: Open redirect + auth error swallowing (highest impact)
2. **BUG-WEAK-001** — Memory leaks in bridge layer (user-facing performance degradation)
3. **BUG-WEAK-002** — Cache unbounded growth (long-session stability)
4. **BUG-WEAK-004** — State sync inconsistency (data correctness)
5. **BUG-WEAK-005** — Error handling (diagnostics & debugging)
6. **BUG-WEAK-008** — Router guard brittleness (feature additions)
7. **BUG-WEAK-006** — Performance optimization (load time)
8. **BUG-WEAK-007** — localStorage cleanup (maintenance debt)

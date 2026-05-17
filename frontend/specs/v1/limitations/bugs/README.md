# Frontend Limitations & Bugs — Index

> **Version**: 1.0.0  
> **Date**: 2026-05-13  
> **Scope**: Toàn bộ frontend tại `vnp-bytebase/frontend/`

---

## Tổng Quan

Tài liệu này tổng hợp các giới hạn, rủi ro và bugs được phát hiện qua phân tích mã nguồn frontend Bytebase. Mỗi issue được phân loại theo severity và có file chi tiết riêng.

## Severity Summary

| Severity | Count | Issues |
|---|---|---|
| **Critical** | 1 | BUG-LIM-003 |
| **High** | 3 | BUG-LIM-001, BUG-LIM-002, BUG-LIM-004 |
| **Medium** | 4 | BUG-LIM-005, BUG-LIM-006, BUG-LIM-007, BUG-LIM-008 |

## Issue Registry

| ID | Title | Category | Severity | File |
|---|---|---|---|---|
| [BUG-LIM-001](./BUG-LIM-001-react-mount-race-condition.md) | Race Condition Trong Vue→React Bridge Mount | Runtime Bug | High | `ReactPageMount.vue`, `mount.ts` |
| [BUG-LIM-002](./BUG-LIM-002-cache-memory-leak.md) | Entity Cache Tăng Trưởng Không Giới Hạn | Memory Management | High | `store/cache.ts` |
| [BUG-LIM-003](./BUG-LIM-003-token-refresh-edge-cases.md) | Token Refresh Cross-Tab Edge Cases | Auth / Security | Critical | `refreshToken.ts`, `token-manager.ts` |
| [BUG-LIM-004](./BUG-LIM-004-error-boundary-gaps.md) | React Exceptions Không Được Catch | Error Handling | High | `App.vue`, `mount.ts` |
| [BUG-LIM-005](./BUG-LIM-005-weakref-polyfill-memory-leak.md) | WeakRef Polyfill Gây Memory Leak | Browser Compat | Medium | `polyfill.ts` |
| [BUG-LIM-006](./BUG-LIM-006-query-preservation-navigation.md) | Query Preservation Loop & Open Redirect | Routing | Medium | `App.vue`, `router/index.ts` |
| [BUG-LIM-007](./BUG-LIM-007-dual-i18n-desync.md) | Dual i18n System Desync | i18n | Medium | `plugins/i18n.ts`, `react/i18n.ts` |
| [BUG-LIM-008](./BUG-LIM-008-build-system-fragility.md) | Build System Fragility | Build / DevOps | Medium | `vite.config.ts`, `package.json` |

## Risk Matrix

```
                    ┌─────────────────────────────────┐
     High Impact    │  BUG-003   │  BUG-001  BUG-002  │
                    │ (Critical) │  BUG-004   (High)   │
                    ├────────────┼─────────────────────┤
     Med Impact     │  BUG-006   │  BUG-007  BUG-008  │
                    │            │  BUG-005            │
                    ├────────────┼─────────────────────┤
                    │ Low Prob   │  Med-High Prob      │
                    └─────────────────────────────────┘
```

## Top Priorities

1. **BUG-LIM-003** (Critical) — Token refresh race conditions có thể gây session loss cho tất cả browser tabs hoặc bị exploit qua XSS.
2. **BUG-LIM-004** (High) — Thiếu React Error Boundary gây blank page khi React component crash.
3. **BUG-LIM-002** (High) — Entity cache unbounded growth gây degradation trong long sessions.
4. **BUG-LIM-001** (High) — Vue→React bridge race conditions gây rendering bugs.

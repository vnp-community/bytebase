# Frontend Limitations — Solutions Index

> **Version**: 1.0.0  
> **Date**: 2026-05-13  
> **Source**: Bugs tại `specs/v1/limitations/bugs/`

---

## Tổng Quan

Mỗi solution document đề xuất giải pháp cụ thể cho bug/limitation tương ứng, bao gồm code changes, architecture document updates, và acceptance criteria.

## Solution Registry

| ID | Title | Resolves | Type | Priority | Effort |
|---|---|---|---|---|---|
| [SOL-LIM-001](./SOL-LIM-001-safe-react-bridge-mount.md) | Safe React Bridge Mount | BUG-LIM-001 | Code Fix | High | 2 ngày |
| [SOL-LIM-002](./SOL-LIM-002-bounded-lru-cache.md) | Bounded LRU Cache With TTL | BUG-LIM-002 | Architecture | High | 1 tuần |
| [SOL-LIM-003](./SOL-LIM-003-resilient-token-refresh.md) | Resilient Cross-Tab Token Refresh | BUG-LIM-003 | Security Fix | Critical | 1 tuần |
| [SOL-LIM-004](./SOL-LIM-004-unified-error-boundary.md) | Unified Error Boundary Layer | BUG-LIM-004 | Architecture | High | 3 ngày |
| [SOL-LIM-005](./SOL-LIM-005-browser-baseline-upgrade.md) | Browser Baseline Upgrade | BUG-LIM-005 | Config | Medium | 1 ngày |
| [SOL-LIM-006](./SOL-LIM-006-hardened-navigation.md) | Hardened Navigation & Redirects | BUG-LIM-006 | Security Fix | Medium | 2 ngày |
| [SOL-LIM-007](./SOL-LIM-007-unified-locale-manager.md) | Unified Locale Manager | BUG-LIM-007 | Architecture | Medium | 1 tuần |
| [SOL-LIM-008](./SOL-LIM-008-build-pipeline-hardening.md) | Build Pipeline Hardening | BUG-LIM-008 | DevOps | Medium | 2 ngày |

## Architecture & TDD Changes Summary

Các solutions yêu cầu cập nhật tại `specs/architecture.md` và `specs/technical-design-document.md`:

| Document Section | Changed By | Type |
|---|---|---|
| §4.1 Bridge Pattern | SOL-001 | Add render versioning description |
| §6.4 Cross-Tab Token Refresh | SOL-003 | Rewrite with resilient lock pattern |
| §7.2 Cache Strategy | SOL-002 | Replace with bounded LRU + tiers |
| §8.1 Component Architecture | SOL-004 | Add ErrorBoundary to primitives |
| §9.2 Dual Auth Modes | SOL-003 | Add encrypted storage for token mode |
| §9.3 Security Features | SOL-006 | Add redirect validation description |
| §10.2 Chunk Strategy | SOL-008 | Add react-core chunk + budgets |
| §11.2 i18n Architecture | SOL-007 | Replace with LocaleManager pattern |
| §13.2 Known Constraints | SOL-005, SOL-008 | Update browser baseline, add bundle budget |
| TDD §3.1 Bridge | SOL-001 | Add race condition prevention docs |
| TDD §3.2 Transport | SOL-003 | Add retry safety section |
| TDD §3.3.2 Cache | SOL-002 | Add memory budgeting section |
| TDD §3.6.3 Query | SOL-006 | Replace with route-scoped preservation |
| TDD §3.8 i18n | SOL-007 | Replace with LocaleManager diagram |
| TDD §7 Error Handling | SOL-004 | Update boundaries table + filter logic |

## Implementation Priority

```
Phase 1 — Critical/Security (Week 1-2):
  ├── SOL-LIM-003: Token refresh resilience + XSS mitigation
  ├── SOL-LIM-004: React ErrorBoundary
  └── SOL-LIM-006: Open redirect fix

Phase 2 — High Impact (Week 3-4):
  ├── SOL-LIM-001: React bridge race conditions
  └── SOL-LIM-002: Bounded LRU cache

Phase 3 — Medium Impact (Week 5-6):
  ├── SOL-LIM-005: Browser baseline upgrade
  ├── SOL-LIM-007: Unified locale manager
  └── SOL-LIM-008: Build pipeline hardening
```

## New Files Created By Solutions

| File | Solution | Purpose |
|---|---|---|
| `src/react/components/ErrorBoundary.tsx` | SOL-004 | React error boundary with fallback UI |
| `src/utils/redirect-validator.ts` | SOL-006 | Unified redirect URL validation |
| `src/localeManager.ts` | SOL-007 | Framework-agnostic locale pub/sub |
| `src/store/cache-monitor.ts` | SOL-002 | Dev-mode cache size monitoring |
| `scripts/check-bundle-size.mjs` | SOL-008 | CI bundle size budget gate |
| `scripts/sync-i18n-keys.mjs` | SOL-007 | CI i18n key consistency check |

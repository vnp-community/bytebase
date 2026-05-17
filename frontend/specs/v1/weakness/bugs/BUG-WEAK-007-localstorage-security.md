# BUG-WEAK-007: localStorage Security & Fragmentation

> **Severity**: MEDIUM  
> **Category**: Data Management / Security Bug  
> **Status**: OPEN | **Created**: 2026-05-13

## 1. Mô tả

Hệ thống dùng `localStorage` rộng rãi (200+ call sites) mà không có centralized management, gây ra fragmentation, security risks, và quota issues.

## 2. Chi tiết lỗi

### 2.1 Sensitive Data in localStorage
**File**: `src/auth/token-manager.ts`
- Refresh token stored trong `localStorage` (token mode).
- XSS attack → attacker đọc được refresh token → session hijacking.
- Nên dùng `httpOnly` cookie hoặc encrypted storage.

### 2.2 No Storage Quota Management
- 200+ localStorage operations across codebase — no central tracking.
- SQL Editor tabs, query history, worksheet state, UI preferences → tích lũy data.
- Khi quota đầy (5MB default) → `setItem` throw `QuotaExceededError` → bị swallow bởi empty catches.

### 2.3 Fragmented Storage Keys
Multiple key patterns coexist:
- `bb.{feature}.{email}` — e.g., `bb.recent-visit.user@example.com`
- `bb.project-switch.page-size` — no email scoping
- `bytebase_options` — legacy format
- `bb.sql-editor.tab-state.{project}.{email}` — deeply nested
- `migrateStorageKeys()` exists but only covers known renames.

### 2.4 User Email in Storage Keys — PII Exposure
- Storage keys contain user email: `bb.recent-visit.user@example.com`
- `cleanupUserStorage()` iterates all keys matching `*.{email}` — O(n) scan.
- Email change → old keys orphaned if migration fails.

### 2.5 WebStorageHelper Silent Failures
**File**: `src/utils/web-storage.ts` (L33-51)

```typescript
save<T>(key: string, value: T) {
  try { localStorage.setItem(...); }
  catch { /* nothing */ }  // Quota exceeded? Silently lost.
}

load<T>(key: string, fallbackValue: T) {
  try { return JSON.parse(json) as T; }
  catch { return fallbackValue; }  // Corrupt data? Silent fallback.
}
```

- `save` fail → data loss without notification.
- `load` corrupt data → silent fallback → user preferences reset without explanation.

## 3. Đề xuất Fix
1. **Centralized storage service** — single abstraction with quota monitoring
2. **Encrypt sensitive data** — dùng `Web Crypto API` cho token storage
3. **Namespace cleanup** — auto-cleanup orphaned keys on login
4. **Storage health check** — warn user when approaching quota limit
5. **Hash email in keys** — replace PII with opaque identifier

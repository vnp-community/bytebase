# TASK-W-031: StorageService Core

> **Source**: SOL-WEAK-007 §2.1 | **Priority**: P3 | **Effort**: 3.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/utils/storage-service.ts`

## What
Create centralized `StorageService` class with: namespaced keys, optional AES-GCM encryption, TTL support, quota monitoring, graceful degradation.

## Implementation — see SOL-WEAK-007 §2.1 for full class code:
- `save<T>(key, value, options)` — with QuotaExceeded retry
- `load<T>(key, options, fallback)` — with TTL check, corrupt data cleanup
- `remove(key, namespace)`, `clearNamespace(namespace)`
- `evictExpired()` — sweep all namespaces
- `getStats()` — usage percentage
- `isQuotaWarning()` — >80% threshold
- Private `encrypt()`/`decrypt()` via Web Crypto API

## AC
- [x] File created at `src/utils/storage-service.ts`
- [x] Singleton `storageService` exported
- [x] Encryption works with AES-GCM
- [x] QuotaExceeded triggers auto-eviction + retry
- [x] Corrupt data cleaned up with console.warn

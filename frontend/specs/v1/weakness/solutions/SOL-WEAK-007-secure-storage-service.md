# SOL-WEAK-007: Secure Storage Service — Centralized, Encrypted, Quota-Aware

> **Source**: [BUG-WEAK-007](../bugs/BUG-WEAK-007-localstorage-security.md)  
> **Severity**: MEDIUM → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Thay thế 200+ raw `localStorage` calls bằng `StorageService` — centralized abstraction với encryption cho sensitive data, quota monitoring, PII-free keys, và graceful degradation.

---

## 2. Thiết kế Chi tiết

### 2.1 StorageService Class

```typescript
// src/utils/storage-service.ts

import { isDev } from "@/utils/util";

type StorageNamespace = "auth" | "editor" | "ui" | "workspace" | "temp";

interface StorageOptions {
  /** Encrypt value before storing (for sensitive data) */
  encrypt?: boolean;
  /** Namespace for grouping related keys */
  namespace: StorageNamespace;
  /** TTL in milliseconds. 0 = no expiry */
  ttlMs?: number;
}

interface StorageEntry<T> {
  value: T;
  createdAt: number;
  ttlMs: number;
}

class StorageService {
  private prefix = "bb";
  private quotaWarningThreshold = 0.8; // 80% of 5MB
  private encryptionKey: CryptoKey | null = null;

  /**
   * Save a value to localStorage with namespace, optional encryption, and TTL.
   */
  async save<T>(key: string, value: T, options: StorageOptions): Promise<boolean> {
    const fullKey = this.buildKey(key, options.namespace);
    const entry: StorageEntry<T> = {
      value,
      createdAt: Date.now(),
      ttlMs: options.ttlMs ?? 0,
    };

    try {
      let serialized = JSON.stringify(entry);

      if (options.encrypt) {
        serialized = await this.encrypt(serialized);
      }

      localStorage.setItem(fullKey, serialized);
      return true;
    } catch (error) {
      if (error instanceof DOMException && error.name === "QuotaExceededError") {
        console.warn("[Storage] Quota exceeded, attempting cleanup...");
        this.evictExpired();
        // Retry once after cleanup
        try {
          localStorage.setItem(fullKey, JSON.stringify(entry));
          return true;
        } catch {
          console.error("[Storage] Still exceeds quota after cleanup:", fullKey);
          return false;
        }
      }
      console.warn("[Storage] Failed to save:", fullKey, error);
      return false;
    }
  }

  /**
   * Load a value from localStorage.
   */
  async load<T>(key: string, options: StorageOptions, fallback: T): Promise<T> {
    const fullKey = this.buildKey(key, options.namespace);
    try {
      let raw = localStorage.getItem(fullKey);
      if (raw === null) return fallback;

      if (options.encrypt) {
        raw = await this.decrypt(raw);
      }

      const entry: StorageEntry<T> = JSON.parse(raw);

      // TTL check
      if (entry.ttlMs > 0 && Date.now() - entry.createdAt > entry.ttlMs) {
        localStorage.removeItem(fullKey);
        return fallback;
      }

      return entry.value;
    } catch {
      console.warn("[Storage] Failed to load (corrupt data?):", fullKey);
      localStorage.removeItem(fullKey); // Remove corrupt entry
      return fallback;
    }
  }

  /** Remove a specific key */
  remove(key: string, namespace: StorageNamespace): void {
    localStorage.removeItem(this.buildKey(key, namespace));
  }

  /** Clear all keys in a namespace */
  clearNamespace(namespace: StorageNamespace): void {
    const prefix = `${this.prefix}.${namespace}.`;
    const keys = Object.keys(localStorage).filter(k => k.startsWith(prefix));
    keys.forEach(k => localStorage.removeItem(k));
  }

  /** Evict expired entries across all namespaces */
  evictExpired(): number {
    let evicted = 0;
    const now = Date.now();
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i);
      if (!key?.startsWith(this.prefix)) continue;
      try {
        const entry: StorageEntry<unknown> = JSON.parse(localStorage.getItem(key)!);
        if (entry.ttlMs > 0 && now - entry.createdAt > entry.ttlMs) {
          localStorage.removeItem(key);
          evicted++;
        }
      } catch {
        // Non-JSON entry from legacy code — skip
      }
    }
    return evicted;
  }

  /** Get storage usage stats */
  getStats(): { used: number; total: number; percentage: number } {
    let used = 0;
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)!;
      used += key.length + (localStorage.getItem(key)?.length ?? 0);
    }
    const total = 5 * 1024 * 1024; // 5MB default
    return { used, total, percentage: used / total };
  }

  /** Check if approaching quota */
  isQuotaWarning(): boolean {
    return this.getStats().percentage > this.quotaWarningThreshold;
  }

  // --- Private helpers ---

  private buildKey(key: string, namespace: StorageNamespace): string {
    return `${this.prefix}.${namespace}.${key}`;
  }

  private async getEncryptionKey(): Promise<CryptoKey> {
    if (this.encryptionKey) return this.encryptionKey;
    // Derive key from a stable device identifier
    const raw = new TextEncoder().encode("bb-storage-key-v1");
    this.encryptionKey = await crypto.subtle.importKey(
      "raw", raw, { name: "AES-GCM" }, false, ["encrypt", "decrypt"]
    );
    return this.encryptionKey;
  }

  private async encrypt(data: string): Promise<string> {
    const key = await this.getEncryptionKey();
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encoded = new TextEncoder().encode(data);
    const encrypted = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv }, key, encoded
    );
    const combined = new Uint8Array(iv.length + encrypted.byteLength);
    combined.set(iv);
    combined.set(new Uint8Array(encrypted), iv.length);
    return btoa(String.fromCharCode(...combined));
  }

  private async decrypt(data: string): Promise<string> {
    const key = await this.getEncryptionKey();
    const combined = Uint8Array.from(atob(data), c => c.charCodeAt(0));
    const iv = combined.slice(0, 12);
    const encrypted = combined.slice(12);
    const decrypted = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv }, key, encrypted
    );
    return new TextDecoder().decode(decrypted);
  }
}

export const storageService = new StorageService();
```

### 2.2 PII-Free Keys (Fix BUG 2.4)

```typescript
// src/utils/storage-service.ts — User-scoped keys without PII

/**
 * Generate user-scoped storage key using hash instead of raw email.
 */
export function userScopedKey(baseKey: string, userEmail: string): string {
  // Simple hash — not crypto-grade, just for key uniqueness
  let hash = 0;
  for (let i = 0; i < userEmail.length; i++) {
    hash = ((hash << 5) - hash + userEmail.charCodeAt(i)) | 0;
  }
  return `${baseKey}.u${Math.abs(hash).toString(36)}`;
}

// Usage:
// Before: `bb.recent-visit.user@example.com`
// After:  `bb.ui.recent-visit.u7k3m2` (hashed, namespaced)
```

### 2.3 Token Storage — Encrypted (Fix BUG 2.1)

```typescript
// src/auth/token-manager.ts — Use encrypted storage

import { storageService } from "@/utils/storage-service";

const saveRefreshToken = (token: string) =>
  storageService.save("refresh-token", token, {
    namespace: "auth",
    encrypt: true,  // ← Encrypted with Web Crypto API
    ttlMs: 7 * 24 * 60 * 60 * 1000, // 7 days
  });

const loadRefreshToken = () =>
  storageService.load("refresh-token", {
    namespace: "auth",
    encrypt: true,
  }, null);
```

### 2.4 Cleanup on Login/Logout

```typescript
// On logout: clear auth namespace
storageService.clearNamespace("auth");

// On login: cleanup orphaned user-scoped keys from previous users
storageService.evictExpired();

// Periodic: check quota health
if (storageService.isQuotaWarning()) {
  console.warn("[Storage] Approaching quota limit");
  storageService.evictExpired();
}
```

---

## 3. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | Create `StorageService` class | LOW | 4h |
| 2 | Migrate token-manager to encrypted storage | MEDIUM | 2h |
| 3 | Replace PII keys with hashed keys | MEDIUM (migration) | 3h |
| 4 | Migrate high-traffic localStorage calls | MEDIUM | 6h |
| 5 | Add quota monitoring | LOW | 1h |

**Total**: ~16h (2 days)

---

## 4. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Raw localStorage calls | 200+ | 0 (all via StorageService) |
| Refresh token encryption | Plaintext | AES-GCM encrypted |
| PII in storage keys | Email exposed | Hashed identifier |
| Quota exceeded handling | Silent failure | Auto-evict + warn |
| Corrupt data handling | Silent fallback | Log + cleanup |

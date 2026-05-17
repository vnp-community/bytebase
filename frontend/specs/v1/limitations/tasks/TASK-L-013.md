# TASK-L-013: LRUEntityCache Class

> **Source**: SOL-LIM-002 §2.2 | **Priority**: P2 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/store/cache.ts` (thêm LRUEntityCache class)

## What
Implement `LRUEntityCache<T>` class có bounded size, TTL-based expiry, và LRU eviction. Dùng `shallowReactive(new Map())` để giữ Vue reactivity.

## Implementation

```typescript
// Thêm vào src/store/cache.ts

interface CacheConfig {
  maxEntries: number;
  ttlMs: number;
  tier: "heavy" | "standard" | "light" | "session";
}

type TimestampedEntity<T> = {
  entity: T;
  accessedAt: number;
  createdAt: number;
};

class LRUEntityCache<T> {
  private map: Map<string, TimestampedEntity<T>>;
  private config: CacheConfig;

  constructor(config: CacheConfig) {
    this.config = config;
    this.map = shallowReactive(new Map());
  }

  get(key: string): T | undefined {
    const entry = this.map.get(key);
    if (!entry) return undefined;
    // TTL check
    if (Date.now() - entry.createdAt > this.config.ttlMs) {
      this.map.delete(key);
      return undefined;
    }
    // LRU: move to end
    entry.accessedAt = Date.now();
    this.map.delete(key);
    this.map.set(key, entry);
    return entry.entity;
  }

  set(key: string, entity: T): void {
    if (this.map.size >= this.config.maxEntries && !this.map.has(key)) {
      const oldest = this.map.keys().next().value;
      if (oldest !== undefined) this.map.delete(oldest);
    }
    this.map.set(key, {
      entity,
      accessedAt: Date.now(),
      createdAt: Date.now(),
    });
  }

  delete(key: string): void { this.map.delete(key); }
  clear(): void { this.map.clear(); }
  get size(): number { return this.map.size; }
}
```

## AC
- [ ] `set()` evicts oldest entry when at capacity
- [ ] `get()` returns `undefined` for expired entries (TTL)
- [ ] `get()` moves entry to end (LRU promotion)
- [ ] `shallowReactive` maintains Vue reactivity for computed properties
- [ ] Unit test: 30-entry cache → 31st insert evicts 1st entry

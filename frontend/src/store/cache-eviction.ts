import type { KeyType } from "./cache";

// ─── Configuration ───────────────────────────────────────────

export interface CacheConfig {
  maxSize: number;
  ttlMs: number;
  trace: boolean;
}

const DEFAULT_CONFIG: CacheConfig = {
  maxSize: 500,
  ttlMs: 5 * 60_000, // 5 minutes
  trace: false,
};

const NAMESPACE_CONFIGS: Record<string, Partial<CacheConfig>> = {
  database: { maxSize: 1000, ttlMs: 10 * 60_000 },
  instance: { maxSize: 200, ttlMs: 10 * 60_000 },
  project: { maxSize: 200, ttlMs: 10 * 60_000 },
  issue: { maxSize: 300, ttlMs: 5 * 60_000 },
  plan: { maxSize: 300, ttlMs: 5 * 60_000 },
  worksheet: { maxSize: 200, ttlMs: 5 * 60_000 },
  dbSchema: { maxSize: 50, ttlMs: 5 * 60_000 },
  databaseCatalog: { maxSize: 50, ttlMs: 5 * 60_000 },
  environment: { maxSize: 100, ttlMs: 30 * 60_000 },
  role: { maxSize: 100, ttlMs: 30 * 60_000 },
  group: { maxSize: 100, ttlMs: 30 * 60_000 },
  policy: { maxSize: 200, ttlMs: 30 * 60_000 },
  setting: { maxSize: 100, ttlMs: 30 * 60_000 },
  auth: { maxSize: 10, ttlMs: Infinity }, // never expires
  subscription: { maxSize: 10, ttlMs: Infinity },
  workspace: { maxSize: 10, ttlMs: Infinity },
};

export function getConfigForNamespace(namespace: string): CacheConfig {
  const overrides = NAMESPACE_CONFIGS[namespace];
  return overrides ? { ...DEFAULT_CONFIG, ...overrides } : { ...DEFAULT_CONFIG };
}

// ─── Eviction Engine ─────────────────────────────────────────

interface TimestampedEntry {
  createdAt: number;
  lastAccessedAt: number;
}

/**
 * CacheEvictionEngine provides periodic TTL sweep and LRU size enforcement
 * for entity caches. It runs via requestIdleCallback (or setInterval fallback)
 * every 60 seconds.
 */
export class CacheEvictionEngine {
  private intervalId: ReturnType<typeof setInterval> | null = null;
  private readonly sweepIntervalMs = 60_000;
  private registeredCaches = new Map<
    string,
    {
      map: Map<string, TimestampedEntry>;
      config: CacheConfig;
    }
  >();

  /**
   * Register a namespace's entity cache map for periodic eviction.
   */
  register(
    namespace: string,
    map: Map<string, TimestampedEntry>,
    config: CacheConfig
  ): void {
    this.registeredCaches.set(namespace, { map, config });
  }

  /**
   * Unregister a namespace.
   */
  unregister(namespace: string): void {
    this.registeredCaches.delete(namespace);
  }

  /**
   * Sweep all registered caches: remove entries that exceed TTL.
   */
  sweepAll(): void {
    const now = Date.now();
    for (const [namespace, { map, config }] of this.registeredCaches) {
      if (config.ttlMs === Infinity) continue;
      let evicted = 0;
      for (const [key, entry] of map) {
        if (now - entry.createdAt > config.ttlMs) {
          map.delete(key);
          evicted++;
        }
      }
      if (evicted > 0 && config.trace) {
        console.debug(`[CacheEviction] ${namespace}: swept ${evicted} expired entries`);
      }
    }
  }

  /**
   * Enforce size limit on a specific namespace using LRU eviction.
   * Removes least-recently-accessed entries until size <= maxSize.
   */
  enforceSizeLimit(namespace: string): void {
    const cache = this.registeredCaches.get(namespace);
    if (!cache) return;
    const { map, config } = cache;
    if (map.size <= config.maxSize) return;

    // Sort entries by lastAccessedAt ascending (oldest first)
    const entries = Array.from(map.entries()).sort(
      (a, b) => a[1].lastAccessedAt - b[1].lastAccessedAt
    );

    const toRemove = map.size - config.maxSize;
    for (let i = 0; i < toRemove && i < entries.length; i++) {
      map.delete(entries[i][0]);
    }

    if (config.trace) {
      console.debug(
        `[CacheEviction] ${namespace}: evicted ${toRemove} entries (LRU), size now ${map.size}`
      );
    }
  }

  /**
   * Start periodic sweeps.
   */
  start(): void {
    if (this.intervalId) return;
    this.intervalId = setInterval(() => {
      if (typeof requestIdleCallback === "function") {
        requestIdleCallback(() => this.sweepAll(), { timeout: 5000 });
      } else {
        this.sweepAll();
      }
    }, this.sweepIntervalMs);
  }

  /**
   * Stop periodic sweeps.
   */
  stop(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  /**
   * Get statistics for all registered caches.
   */
  getStats(): Array<{ namespace: string; size: number; maxSize: number; ttlMs: number }> {
    return Array.from(this.registeredCaches.entries()).map(([namespace, { map, config }]) => ({
      namespace,
      size: map.size,
      maxSize: config.maxSize,
      ttlMs: config.ttlMs,
    }));
  }
}

/** Singleton eviction engine. */
export const evictionEngine = new CacheEvictionEngine();

// Auto-start in browser environment
if (typeof window !== "undefined") {
  evictionEngine.start();
}

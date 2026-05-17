import { shallowReactive } from "vue";
import { isDev } from "@/utils";
import { evictionEngine, getConfigForNamespace } from "./cache-eviction";

export type KeyType = string | number | boolean;

type RequestCacheEntry<K extends KeyType[], T> = {
  keys: K;
  promise: Promise<T>;
  abortController: AbortController;
};
type EntityCacheEntry<K extends KeyType[], T> = {
  keys: K;
  entity: T;
  createdAt: number;
  lastAccessedAt: number;
};

// ─── LRU Entity Cache ────────────────────────────────────────

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

export class LRUEntityCache<T> {
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

// ─── Tier Configuration ──────────────────────────────────────

const TIER_DEFAULTS: Record<string, CacheConfig> = {
  heavy:    { maxEntries: 30,  ttlMs: 5 * 60_000,  tier: "heavy" },
  standard: { maxEntries: 200, ttlMs: 10 * 60_000, tier: "standard" },
  light:    { maxEntries: 500, ttlMs: 30 * 60_000, tier: "light" },
  session:  { maxEntries: Infinity, ttlMs: Infinity, tier: "session" },
};

const NAMESPACE_TIER_MAP: Record<string, string> = {
  dbSchema: "heavy", databaseCatalog: "heavy",
  database: "standard", instance: "standard", project: "standard",
  issue: "standard", plan: "standard", worksheet: "standard",
  environment: "light", role: "light", group: "light",
  policy: "light", setting: "light",
  auth: "session", subscription: "session", workspace: "session",
};

// ─── Cache Registries ────────────────────────────────────────

const REQUEST_CACHE = new Map<
  string,
  Map<string, RequestCacheEntry<KeyType[], unknown>>
>();
const ENTITY_CACHE = new Map<
  string,
  Map<string, EntityCacheEntry<KeyType[], unknown>>
>();
const LRU_CACHE_REGISTRY = new Map<string, LRUEntityCache<unknown>>();

/** Returns all registered LRU caches. Used by cache-monitor. */
export function getEntityCacheRegistry(): Map<string, { size: number }> {
  return LRU_CACHE_REGISTRY as Map<string, { size: number }>;
}

function getOrCreateLRUCache<T>(namespace: string, config: CacheConfig): LRUEntityCache<T> {
  const existing = LRU_CACHE_REGISTRY.get(namespace);
  if (existing) return existing as LRUEntityCache<T>;
  const created = new LRUEntityCache<T>(config);
  LRU_CACHE_REGISTRY.set(namespace, created as LRUEntityCache<unknown>);
  return created;
}

export const useCache = <K extends KeyType[], T>(namespace: string) => {
  const tier = NAMESPACE_TIER_MAP[namespace] || "standard";
  const config = TIER_DEFAULTS[tier];
  const requestCacheMap = getRequestCacheMap<K, T>(namespace);
  const entityCacheMap = getEntityCacheMap<K, T>(namespace);
  const entityCache = getOrCreateLRUCache<T>(namespace, config);

  // Register with eviction engine for periodic sweeps
  const nsConfig = getConfigForNamespace(namespace);
  evictionEngine.register(
    namespace,
    entityCacheMap as unknown as Map<string, { createdAt: number; lastAccessedAt: number }>,
    nsConfig
  );

  const trace = isDev()
    ? (title: string, keys: KeyType[], ...args: unknown[]) =>
        console.debug("cache", namespace, title, JSON.stringify(keys), ...args)
    : () => {};

  const invalidateRequest = (keys: K) => {
    const key = getKey(keys);
    const request = requestCacheMap.get(key);
    if (!request) return;
    if (!request.abortController.signal.aborted) {
      request.abortController.abort();
    }
    requestCacheMap.delete(key);
  };

  const getRequest = (keys: K) => {
    const key = getKey(keys);
    trace("getRequest", keys, requestCacheMap.has(key));
    const request = requestCacheMap.get(key);
    if (!request) {
      return undefined;
    }
    if (request.abortController.signal.aborted) {
      invalidateRequest(keys);
      return undefined;
    }
    return request.promise;
  };

  const setRequest = (keys: K, promise: Promise<T>) => {
    invalidateRequest(keys);

    const key = getKey(keys);
    const abortController = new AbortController();
    promise
      .then((entity: T) => {
        if (!abortController.signal.aborted) {
          setEntity(keys, entity);
        }
      })
      .catch(() => {
        invalidateRequest(keys);
      })
      .finally(() => {
        invalidateRequest(keys);
      });
    requestCacheMap.set(key, {
      keys,
      promise,
      abortController,
    });
    // trace("setRequest", keys);
  };

  const getEntity = (keys: K) => {
    const key = getKey(keys);
    // Try LRU cache first, fallback to legacy map
    const lruResult = entityCache.get(key);
    if (lruResult !== undefined) return lruResult;
    // trace("getEntity", keys, entityCacheMap.has(key));
    // TTL check on legacy map entries
    const entry = entityCacheMap.get(key);
    if (entry) {
      if (nsConfig.ttlMs !== Infinity && Date.now() - entry.createdAt > nsConfig.ttlMs) {
        entityCacheMap.delete(key);
        return undefined;
      }
      entry.lastAccessedAt = Date.now();
      return entry.entity;
    }
    return undefined;
  };

  const setEntity = (keys: K, entity: T) => {
    const key = getKey(keys);
    entityCache.set(key, entity);
    const now = Date.now();
    entityCacheMap.set(key, {
      keys,
      entity,
      createdAt: now,
      lastAccessedAt: now,
    });
    // Trigger LRU eviction after insert
    evictionEngine.enforceSizeLimit(namespace);
    // trace("setEntity", keys);
  };

  const invalidateEntity = (keys: K) => {
    invalidateRequest(keys);
    const key = getKey(keys);
    entityCache.delete(key);
    entityCacheMap.delete(key);
  };

  const clear = () => {
    // abort and invalidate all flying requests
    for (const request of requestCacheMap.values()) {
      if (!request.abortController.signal.aborted) {
        request.abortController.abort();
      }
    }
    // clear all cache entries
    requestCacheMap.clear();
    entityCacheMap.clear();
    entityCache.clear();
  };

  const getStats = () => ({
    namespace,
    entityCount: entityCacheMap.size,
    requestCount: requestCacheMap.size,
    maxSize: nsConfig.maxSize,
    ttlMs: nsConfig.ttlMs,
  });

  return {
    requestCacheMap,
    entityCacheMap,
    getRequest,
    getEntity,
    setRequest,
    setEntity,
    invalidateRequest,
    invalidateEntity,
    clear,
    getStats,
  };
};

const getRequestCacheMap = <K extends KeyType[], T>(namespace: string) => {
  const existed = REQUEST_CACHE.get(namespace) as Map<
    string,
    RequestCacheEntry<K, T>
  >;
  if (existed) {
    return existed;
  }
  const created = new Map<string, RequestCacheEntry<K, T>>();
  REQUEST_CACHE.set(namespace, created);
  return created;
};

const getEntityCacheMap = <K extends KeyType[], T>(namespace: string) => {
  const existed = ENTITY_CACHE.get(namespace) as Map<
    string,
    EntityCacheEntry<K, T>
  >;

  if (existed) {
    return existed;
  }
  const created = shallowReactive(new Map<string, EntityCacheEntry<K, T>>());
  ENTITY_CACHE.set(namespace, created);
  return created;
};

const getKey = (keys: KeyType[]) => {
  return JSON.stringify(keys);
};

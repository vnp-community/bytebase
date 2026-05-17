import { isDev } from "@/utils/util";

export function startCacheMonitor(getCaches: () => Map<string, { size: number }>) {
  if (!isDev()) return;

  setInterval(() => {
    const caches = getCaches();
    const stats: Record<string, number> = {};
    for (const [ns, cache] of caches.entries()) {
      stats[ns] = cache.size;
    }
    const total = Object.values(stats).reduce((a, b) => a + b, 0);
    if (total > 500) {
      console.warn("[Cache Monitor] High entity count:", total, stats);
    }
  }, 30_000);
}

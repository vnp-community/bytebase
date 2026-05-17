# TASK-L-015: Dev-Mode Cache Monitor

> **Source**: SOL-LIM-002 §2.4 | **Priority**: P2 | **Effort**: 1h  
> **Status**: DONE | **Deps**: L-013

## Scope
- **NEW** `src/store/cache-monitor.ts`
- **EDIT** `src/main.ts` hoặc `src/App.vue` (call startCacheMonitor)

## What
Tạo dev-only cache monitor, log warning khi total entity count > 500.

## Implementation

### File 1: `src/store/cache-monitor.ts` (NEW)
```typescript
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
```

### File 2: Wire in bootstrap
```diff
+import { startCacheMonitor } from "@/store/cache-monitor";
+import { getEntityCacheRegistry } from "@/store/cache";
+
+// After Pinia setup
+startCacheMonitor(getEntityCacheRegistry);
```

## AC
- [ ] Monitor only active in dev mode
- [ ] Warning logged when total entities > 500
- [ ] No production bundle impact (tree-shaken by isDev check)

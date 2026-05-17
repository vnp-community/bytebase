# TASK-W-016: Refactor useAppState to Pinia Proxy

> **Source**: SOL-WEAK-004 §3.2-3.3 | **Priority**: P2 | **Effort**: 2.5h  
> **Status**: DONE | **Deps**: W-015

## Scope
- **EDIT** `src/react/hooks/useAppState.ts`

## What
Replace Zustand-based domain hooks with Pinia proxy via `useVueState`. Add `isLoaded` guard to skip redundant fetches.

## Implementation — see SOL-WEAK-004 §3.2-3.3
```diff
-export function useCurrentUser() {
-  return useAppStore(s => s.currentUser);
-}
+export function useCurrentUser() {
+  return useVueState(() => useAuthStore().currentUser);
+}

-export function useSubscription() {
-  useEffect(() => { loadSubscription(); }, []);
-  return useAppStore(s => s.subscription);
-}
+export function useSubscription() {
+  return useVueState(() => useSubscriptionV1Store().subscription);
+}
```

Add initialization guard:
```typescript
let appStateInitialized = false;
export function useAppStateInit() {
  useEffect(() => {
    if (appStateInitialized) return;
    appStateInitialized = true;
  }, []);
}
export function resetAppStateInit() { appStateInitialized = false; }
```

## AC
- [x] `useCurrentUser` proxies Pinia auth store
- [x] `useSubscription` proxies Pinia subscription store (no duplicate fetch)
- [x] `useAppStateInit` prevents redundant initialization
- [x] No duplicate API calls for user/subscription data

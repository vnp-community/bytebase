# TASK-W-015: useVueState Deep Default

> **Source**: SOL-WEAK-004 §3.1 | **Priority**: P2 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/react/hooks/useVueState.ts`

## What
Change `deep` default from `false` to `true` so React re-renders on in-place Pinia mutations.

## Implementation
```diff
 export function useVueState<T>(
   getter: () => T,
   options?: { deep?: boolean }
 ): T {
-  const { deep = false } = options ?? {};
+  const { deep = true } = options ?? {};
   // ... rest unchanged
 }
```

## AC
- [ ] Default `deep` is `true`
- [ ] Existing `{ deep: false }` call sites still work (opt-out preserved)
- [ ] React components re-render on nested Pinia state changes

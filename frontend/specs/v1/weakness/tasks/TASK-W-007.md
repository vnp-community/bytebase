# TASK-W-007: BridgeLifecycleManager

> **Source**: SOL-WEAK-001 §3.1 | **Priority**: P1 | **Effort**: 3h  
> **Status**: DONE | **Deps**: W-005, W-006

## Scope
- **NEW** `src/react/BridgeLifecycleManager.ts`

## What
Implement `BridgeLifecycleManager` class with cancellable mount, abort-on-remount, error propagation, and destroy (cache clear).

## Implementation
Create `src/react/BridgeLifecycleManager.ts` — full code in SOL-WEAK-001 §3.1:
- `mount(container, page, signal?)` — async mount with AbortController checks after each await
- `update(props)` — re-render without remount
- `unmountCurrent()` — unmount React root
- `destroy()` — abort pending + unmount + `clearPageCache()`
- Singleton export: `bridgeManager`
- Uses `ReactErrorBoundary` from W-005
- Uses types from `bridge-types.ts` (W-006)

## Key Design
1. Cancel previous pending mount when new mount requested
2. Check `signal.aborted` after every `await` (loadCoreDeps, loadPage)
3. Unmount previous root before creating new
4. Wrap page in `ReactErrorBoundary`
5. Re-throw errors (not swallow)

## AC
- [ ] File created at `src/react/BridgeLifecycleManager.ts`
- [ ] Fast navigation: new mount cancels previous pending mount
- [ ] Unmounted root before creating new root
- [ ] Errors propagated to caller (not swallowed)
- [ ] `destroy()` clears page cache and unmounts

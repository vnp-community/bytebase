# TASK-W-006: Bridge Type Definitions

> **Source**: SOL-WEAK-001 §3.4 | **Priority**: P1 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/react/bridge-types.ts`

## What
Create typed interfaces to replace `any` in bridge layer: `BridgePageBaseProps`, `ReactPageComponent`, `ReactCoreDeps`, `MountFunction`.

## Implementation
Create `src/react/bridge-types.ts` — full code in SOL-WEAK-001 §3.4:
- `BridgePageBaseProps` — base interface for page props
- `ReactPageComponent<P>` — replaces `(props: any) => any`
- `ReactCoreDeps` — replaces `ReactDeps = any`
- `MountFunction<P>` — typed mount signature

## AC
- [ ] File created at `src/react/bridge-types.ts`
- [ ] No `any` types in new file
- [ ] Types are generic and reusable
- [ ] Exported for use by `BridgeLifecycleManager` and `mount.ts`

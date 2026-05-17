# TASK-W-017: Typed Shell Bridge

> **Source**: SOL-WEAK-004 §3.4 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/react/shell-bridge.ts`

## What
Replace raw `CustomEvent<unknown>` with typed `ShellBridge` class supporting: typed event map, replay for late subscribers, proper cleanup.

## Implementation — see SOL-WEAK-004 §3.4
- `ShellBridgeEventMap`: typed interface for all bridge events
- `ShellBridge` class: `dispatch<K>()`, `on<K>(callback, { replay? })`
- Replay: last event per type stored, replayed to late subscribers
- Singleton: `export const shellBridge = new ShellBridge()`

## AC
- [x] All bridge events typed (localeChange, themeChange, routeChange, authStateChange)
- [x] `dispatch` and `on` are type-safe (no `unknown`)
- [x] Late subscribers can replay last event
- [x] `on` returns unsubscribe function

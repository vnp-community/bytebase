# SOL-WEAK-004: Unified State Bridge — Single Source of Truth for Dual Framework

> **Source**: [BUG-WEAK-004](../bugs/BUG-WEAK-004-state-sync-dual-framework.md)  
> **Severity**: MEDIUM-HIGH → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Thiết lập Pinia là **single source of truth** cho tất cả domain data. React sử dụng `useVueState` (deep by default) thay vì duplicate fetching. Typed shell bridge thay thế raw `CustomEvent<unknown>`.

---

## 2. Thay đổi Kiến trúc

### Architecture Doc Section 4.2 State Sharing
- Pinia = sole domain state owner
- Zustand = React-only UI state (panels, selections, local form state)
- Shell bridge typed + replay-capable

### TDD Section 3.3
- `useVueState` deep by default
- `useAppState` refactored to proxy Pinia only

---

## 3. Thiết kế Chi tiết

### 3.1 `useVueState` — Deep by Default (Fix BUG 2.1)

```typescript
// src/react/hooks/useVueState.ts — Updated

export function useVueState<T>(
  getter: () => T,
  options?: { deep?: boolean }
): T {
  const { deep = true } = options ?? {};  // ← DEFAULT: true (was false)
  // ... rest of implementation
}
```

**Rationale**: Pinia stores mutate entities in-place via `shallowReactive`. Shallow tracking misses nested property changes. Making `deep: true` default ensures React re-renders on all Pinia mutations. Performance-sensitive consumers can opt out with `{ deep: false }`.

### 3.2 Eliminate Duplicate Fetching (Fix BUG 2.2, 2.4)

```typescript
// src/react/hooks/useAppState.ts — BEFORE (duplicate fetching):

export function useCurrentUser() {
  // Zustand store — fetches from API independently
  return useAppStore(s => s.currentUser);
}

export function useSubscription() {
  // Triggers loadSubscription() on every mount
  useEffect(() => { loadSubscription(); }, []);
  return useAppStore(s => s.subscription);
}

// AFTER (proxy to Pinia — single source of truth):

export function useCurrentUser() {
  return useVueState(() => useAuthStore().currentUser);
}

export function useSubscription() {
  return useVueState(() => useSubscriptionV1Store().subscription);
}

export function useServerInfo() {
  return useVueState(() => useActuatorV1Store().serverInfo);
}

// Zustand useAppStore is DEPRECATED for domain data.
// Only use for React-specific UI state.
```

### 3.3 `isLoaded` Guard — Skip Redundant Fetches (Fix BUG 2.4)

```typescript
// src/react/hooks/useAppState.ts — Add initialization guard

let appStateInitialized = false;

export function useAppStateInit() {
  useEffect(() => {
    if (appStateInitialized) return;
    appStateInitialized = true;
    
    // One-time initialization — Pinia stores already loaded by Vue bootstrap
    // No duplicate API calls needed
  }, []);
}

// Reset on logout
export function resetAppStateInit() {
  appStateInitialized = false;
}
```

### 3.4 Typed Shell Bridge (Fix BUG 2.3)

```typescript
// src/react/shell-bridge.ts — BEFORE:
// window.dispatchEvent(new CustomEvent<unknown>("bb.localeChange", ...));

// AFTER:

/** All bridge event types */
interface ShellBridgeEventMap {
  "bb.localeChange": { locale: string };
  "bb.themeChange": { theme: "light" | "dark" };
  "bb.routeChange": { path: string; params: Record<string, string> };
  "bb.authStateChange": { isLoggedIn: boolean };
}

type ShellBridgeEventName = keyof ShellBridgeEventMap;

/**
 * Typed, replay-capable shell bridge.
 * Last event per type is stored for late subscribers.
 */
class ShellBridge {
  private lastEvents = new Map<string, unknown>();

  dispatch<K extends ShellBridgeEventName>(
    event: K,
    detail: ShellBridgeEventMap[K]
  ): void {
    this.lastEvents.set(event, detail);
    window.dispatchEvent(new CustomEvent(event, { detail }));
  }

  /**
   * Subscribe to bridge events.
   * If `replay: true`, immediately fires callback with last stored event.
   */
  on<K extends ShellBridgeEventName>(
    event: K,
    callback: (detail: ShellBridgeEventMap[K]) => void,
    options?: { replay?: boolean }
  ): () => void {
    // Replay last event if available and requested
    if (options?.replay) {
      const last = this.lastEvents.get(event);
      if (last) callback(last as ShellBridgeEventMap[K]);
    }

    const handler = (e: Event) => {
      callback((e as CustomEvent).detail as ShellBridgeEventMap[K]);
    };
    window.addEventListener(event, handler);
    return () => window.removeEventListener(event, handler);
  }
}

export const shellBridge = new ShellBridge();
```

### 3.5 State Ownership Matrix (Updated)

| Data Domain | Owner | Vue Access | React Access |
|---|---|---|---|
| Auth/User | Pinia `auth.ts` | Direct | `useVueState(() => useAuthStore().*)` |
| Database | Pinia `database.ts` | Direct | `useVueState(() => useDatabaseV1Store().*)` |
| Project | Pinia `project.ts` | Direct | `useVueState(() => useProjectV1Store().*)` |
| Subscription | Pinia `subscription.ts` | Direct | `useVueState(() => useSubscriptionV1Store().*)` |
| SQL Editor tabs | Pinia `sqlEditor/tab.ts` | Direct | `useVueState(() => useSQLEditorTabStore().*)` |
| React UI state | Zustand `app/` | N/A | Direct Zustand |
| Agent window | Zustand `agent/` | N/A | Direct Zustand |

---

## 4. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | `useVueState` deep=true default | LOW | 0.5h |
| 2 | Refactor `useAppState.ts` to proxy Pinia | MEDIUM | 3h |
| 3 | Implement typed `ShellBridge` | LOW | 2h |
| 4 | Add `isLoaded` guards | LOW | 1h |
| 5 | Deprecate Zustand domain stores | LOW | 1h |

**Total**: ~7.5h (1 day)

---

## 5. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Duplicate API calls (user fetch) | 2 (Vue + React) | 1 (Pinia only) |
| Shell bridge type safety | `unknown` | Fully typed |
| Late subscriber event loss | Yes | No (replay support) |
| Re-renders from shallow tracking miss | Frequent | None (deep default) |

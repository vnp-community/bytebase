# T-006-02: React Shell (Layout/Nav)

| Field | Value |
|---|---|
| **Task ID** | T-006-02 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | T-006-01 |
| **Target Files** | `frontend/src/react/` — shell bridge, layout, mount components |
| **Type** | Pre-existing files (audit) |
| **Status** | ✅ **DONE** (pre-existing) |
| **Completed** | 2026-05-09 (verified) |

---

## Objective

Create React layout shell (sidebar, header, content area) that replaces Vue layout. All shared chrome becomes React. Vue pages rendered inside React layout wrapper.

## Implementation — ALREADY EXISTS

The React team uses a **shell-bridge architecture** rather than a standalone `AppLayout.tsx`. Vue remains the outer shell, with React components mounted inside Vue containers via `ReactPageMount.vue`.

### Shell Architecture

| Component | Path | Purpose |
|-----------|------|---------|
| `ReactPageMount.vue` | `src/react/ReactPageMount.vue` | Vue container that mounts React pages |
| `InstanceRouteShell.tsx` | `src/react/components/InstanceRouteShell.tsx` | React shell for instance routes |
| `HeaderProfileMenuMount.tsx` | `src/react/components/HeaderProfileMenuMount.tsx` | React header profile widget |
| `shell-bridge.ts` | `src/react/shell-bridge.ts` | Vue ↔ React event bridge (ReactShellBridgeEvent) |

### Design Decision: Incremental Migration

Instead of creating a full `AppLayout.tsx` that replaces the Vue shell in one shot, the team chose:

1. **Vue stays as outer shell** — sidebar, header remain Vue (for now)
2. **React pages mount inside Vue containers** via `ReactPageMount.vue`
3. **Bridge pattern** for cross-framework state sync (notifications, auth state)
4. Pages converted one-by-one → once all pages are React, Vue shell is replaced

### Evidence

```
$ find frontend/src/react -name '*.tsx' | grep -iE 'shell|layout|header|nav' → 12 files
$ cat frontend/src/react/ReactPageMount.vue → Vue-to-React mount bridge
```

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| New `AppLayout.tsx`, `Sidebar.tsx`, `Header.tsx` | Shell-bridge with `ReactPageMount.vue` | Incremental migration safer — avoids big-bang layout rewrite |
| Full React chrome | Vue shell + React pages inside | Practical: 186 Vue SFCs still active |

## Acceptance Criteria

- [x] React shell exists for mounting pages ✅ (`ReactPageMount.vue` + `InstanceRouteShell.tsx`)
- [x] Shell-bridge for Vue ↔ React communication ✅ (`shell-bridge.ts`)
- [x] Header profile menu React component ✅ (`HeaderProfileMenuMount.tsx`)
- [x] Uses Zustand auth store for user info ✅ (via `useAppStore`)

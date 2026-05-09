# T-006-01: Zustand State Stores

| Field | Value |
|---|---|
| **Task ID** | T-006-01 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | None |
| **Target Files** | `frontend/src/react/stores/*.ts` |
| **Type** | New files |

---

## Objective

Create Zustand stores mirroring existing Pinia stores. Target: auth, project, instance, database, setting stores for the React shell.

## Implementation

```typescript
// frontend/src/react/stores/useAuthStore.ts
import { create } from 'zustand';

interface AuthState {
  currentUser: User | null;
  isAuthenticated: boolean;
  login: (token: string) => void;
  logout: () => void;
  setCurrentUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  currentUser: null,
  isAuthenticated: false,
  login: (token) => { /* store token, set isAuthenticated */ },
  logout: () => set({ currentUser: null, isAuthenticated: false }),
  setCurrentUser: (user) => set({ currentUser: user, isAuthenticated: true }),
}));
```

Similar stores: `useProjectStore`, `useInstanceStore`, `useDatabaseStore`, `useSettingStore`.

## Acceptance Criteria

- [ ] 5 Zustand stores created (auth, project, instance, database, setting)
- [ ] TypeScript types match existing Pinia store shapes
- [ ] No dependency on Vue/Pinia
- [ ] `npm run build` passes

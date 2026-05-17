import { create } from "zustand";
import { devtools } from "zustand/middleware";
import type { User } from "@/types/proto-es/v1/user_service_pb";

/**
 * Auth store — client-state for current user session.
 *
 * During the Vue→React transition, use `syncAuthFromVue()` at boot
 * to keep this store in sync with the Pinia auth store.
 */
interface AuthState {
  currentUser: User | null;
  isLoggedIn: boolean;
  requireResetPassword: boolean;
  requireMfa: boolean;
}

interface AuthActions {
  setCurrentUser: (user: User) => void;
  clearAuth: () => void;
  setRequireResetPassword: (v: boolean) => void;
  setRequireMfa: (v: boolean) => void;
}

export type AuthStore = AuthState & AuthActions;

export const useAuthStore = create<AuthStore>()(
  devtools(
    (set) => ({
      // State
      currentUser: null,
      isLoggedIn: false,
      requireResetPassword: false,
      requireMfa: false,

      // Actions
      setCurrentUser: (user) =>
        set({ currentUser: user, isLoggedIn: true }, false, "setCurrentUser"),
      clearAuth: () =>
        set(
          {
            currentUser: null,
            isLoggedIn: false,
            requireResetPassword: false,
            requireMfa: false,
          },
          false,
          "clearAuth"
        ),
      setRequireResetPassword: (v) =>
        set({ requireResetPassword: v }, false, "setRequireResetPassword"),
      setRequireMfa: (v) =>
        set({ requireMfa: v }, false, "setRequireMfa"),
    }),
    { name: "bb-auth" }
  )
);

/**
 * Sync auth state FROM Pinia (Vue) to Zustand (React).
 * Call once at app bootstrap during the transition period.
 *
 * Usage:
 *   import { useAuthV1Store as usePiniaAuth } from "@/store";
 *   syncAuthFromVue(usePiniaAuth());
 */
export function syncAuthFromVue(
  piniaStore: { currentUser: User | null },
  // Vue's watch function, injected to avoid direct Vue import in React code
  watch: (
    source: () => User | null,
    cb: (v: User | null) => void,
    opts?: { immediate: boolean }
  ) => void
) {
  watch(
    () => piniaStore.currentUser,
    (user) => {
      if (user) {
        useAuthStore.getState().setCurrentUser(user);
      } else {
        useAuthStore.getState().clearAuth();
      }
    },
    { immediate: true }
  );
}

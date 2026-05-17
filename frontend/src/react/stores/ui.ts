import { create } from "zustand";
import { devtools, persist } from "zustand/middleware";

/**
 * UI store — persisted client preferences (locale, theme, sidebar).
 * Stored in localStorage under key "bb-ui".
 */
interface UIState {
  locale: string;
  theme: "light" | "dark";
  sidebarCollapsed: boolean;
  quickstartDismissed: boolean;
}

interface UIActions {
  setLocale: (locale: string) => void;
  setTheme: (theme: "light" | "dark") => void;
  toggleSidebar: () => void;
  dismissQuickstart: () => void;
}

export type UIStore = UIState & UIActions;

export const useUIStore = create<UIStore>()(
  devtools(
    persist(
      (set) => ({
        // State
        locale: "en-US",
        theme: "light",
        sidebarCollapsed: false,
        quickstartDismissed: false,

        // Actions
        setLocale: (locale) =>
          set({ locale }, false, "setLocale"),
        setTheme: (theme) =>
          set({ theme }, false, "setTheme"),
        toggleSidebar: () =>
          set(
            (s) => ({ sidebarCollapsed: !s.sidebarCollapsed }),
            false,
            "toggleSidebar"
          ),
        dismissQuickstart: () =>
          set({ quickstartDismissed: true }, false, "dismissQuickstart"),
      }),
      { name: "bb-ui" }
    ),
    { name: "bb-ui" }
  )
);

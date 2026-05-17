import type { NotificationCreate } from "@/types/notification";

export interface ReactQuickstartResetDetail {
  keys: string[];
}

export interface ShellBridgeEventMap {
  localeChange: string;
  themeChange: "light" | "dark" | "system";
  routeChange: { path: string };
  authStateChange: { isAuthenticated: boolean };
  notification: NotificationCreate;
  quickstartReset: ReactQuickstartResetDetail;
}

export class ShellBridge {
  private lastEvents = new Map<keyof ShellBridgeEventMap, unknown>();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private listeners = new Map<keyof ShellBridgeEventMap, Set<(data: any) => void>>();

  dispatch<K extends keyof ShellBridgeEventMap>(event: K, data: ShellBridgeEventMap[K]) {
    this.lastEvents.set(event, data);
    const eventListeners = this.listeners.get(event);
    if (eventListeners) {
      for (const callback of eventListeners) {
        try {
          callback(data);
        } catch (err) {
          console.error(`Error in ShellBridge listener for ${event}:`, err);
        }
      }
    }
    // Backward compatibility for native CustomEvent listeners (like in App.vue)
    window.dispatchEvent(
      new CustomEvent(`bb.react-${event}`, { detail: data })
    );
  }

  on<K extends keyof ShellBridgeEventMap>(
    event: K,
    callback: (data: ShellBridgeEventMap[K]) => void,
    options?: { replay?: boolean }
  ) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(callback);

    if (options?.replay && this.lastEvents.has(event)) {
      try {
        callback(this.lastEvents.get(event) as ShellBridgeEventMap[K]);
      } catch (err) {
        console.error(`Error in ShellBridge replay for ${event}:`, err);
      }
    }

    return () => {
      const eventListeners = this.listeners.get(event);
      if (eventListeners) {
        eventListeners.delete(callback);
      }
    };
  }
}

export const shellBridge = new ShellBridge();

export const ReactShellBridgeEvent = {
  localeChange: "bb.react-localeChange",
  notification: "bb.react-notification",
  quickstartReset: "bb.react-quickstartReset",
  themeChange: "bb.react-themeChange",
  routeChange: "bb.react-routeChange",
  authStateChange: "bb.react-authStateChange",
} as const;

export type ReactShellBridgeEventName =
  (typeof ReactShellBridgeEvent)[keyof typeof ReactShellBridgeEvent];

export function emitReactLocaleChange(lang: string) {
  shellBridge.dispatch("localeChange", lang);
}

export function emitReactNotification(notification: NotificationCreate) {
  shellBridge.dispatch("notification", notification);
}

export function emitReactQuickstartReset(detail: ReactQuickstartResetDetail) {
  shellBridge.dispatch("quickstartReset", detail);
}

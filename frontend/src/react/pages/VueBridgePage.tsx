// i18n: i18next | use t("key") from useTranslation()
/**
 * VueBridgePage — Temporary React component that mounts the Vue app
 * for routes that haven't been migrated to React yet.
 *
 * This component:
 * 1. Creates a container div inside the React render tree
 * 2. Mounts the Vue app (createApp + router) into that container
 * 3. Syncs React Router location → Vue Router on navigation
 * 4. Unmounts the Vue app on cleanup (React effect cleanup)
 *
 * ⚠️ TEMPORARY: This bridge will be removed when all routes are migrated to React.
 */

import { useEffect, useRef, useCallback } from "react";

/** Lazily loaded Vue app creator to avoid bundling Vue in the initial React chunk. */
async function mountVueApp(container: HTMLElement, initialPath: string) {
  const { createApp } = await import("vue");
  const { default: App } = await import("@/App.vue");
  const { router: vueRouter } = await import("@/router");
  const { pinia } = await import("@/store");
  const { default: i18n } = await import("@/plugins/i18n");
  const { default as NaiveUI } = await import("@/plugins/naive-ui");
  const { default: highlight } = await import("@/plugins/highlight");

  const app = createApp(App);
  app.use(pinia);
  app.use(vueRouter);
  app.use(i18n);
  app.use(NaiveUI);
  app.use(highlight);

  // Navigate Vue Router to the current path before mounting.
  await vueRouter.push(initialPath);

  app.mount(container);

  return { app, vueRouter };
}

interface VueBridgeProps {
  /** Current React Router location pathname + search. */
  currentPath?: string;
}

export function VueBridgePage({ currentPath }: VueBridgeProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const vueInstanceRef = useRef<{
    app: ReturnType<typeof import("vue")["createApp"]> extends Promise<infer T>
      ? T
      : never;
    // biome-ignore lint/suspicious/noExplicitAny: Vue Router type
    vueRouter: any;
  } | null>(null);

  // Mount Vue app on first render.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const path = currentPath || window.location.pathname + window.location.search;
    let cancelled = false;

    mountVueApp(container, path)
      .then((instance) => {
        if (cancelled) {
          instance.app.unmount();
          return;
        }
        // biome-ignore lint/suspicious/noExplicitAny: Runtime Vue instance
        vueInstanceRef.current = instance as any;
      })
      .catch((err) => {
        console.error("[VueBridge] Failed to mount Vue app:", err);
      });

    return () => {
      cancelled = true;
      if (vueInstanceRef.current) {
        vueInstanceRef.current.app.unmount();
        vueInstanceRef.current = null;
      }
    };
  }, []); // Mount once

  // Sync React Router path changes → Vue Router.
  const syncPath = useCallback(async () => {
    const instance = vueInstanceRef.current;
    if (!instance || !currentPath) return;

    try {
      const currentVuePath =
        instance.vueRouter.currentRoute?.value?.fullPath ?? "";
      if (currentVuePath !== currentPath) {
        await instance.vueRouter.push(currentPath);
      }
    } catch (err) {
      console.warn("[VueBridge] Failed to sync path to Vue Router:", err);
    }
  }, [currentPath]);

  useEffect(() => {
    syncPath();
  }, [syncPath]);

  return (
    <div
      ref={containerRef}
      id="vue-bridge-container"
      style={{ width: "100%", height: "100%", minHeight: "100vh" }}
    />
  );
}

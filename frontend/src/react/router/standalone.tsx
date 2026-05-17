// i18n: i18next | use t("key") from useTranslation()
/**
 * React Router Shell — standalone entry point for React-first frontend.
 *
 * This router config is used when the frontend is deployed standalone
 * (separate from the Go backend). It provides:
 *
 * 1. Auth routes (login/signup) — no AuthGuard
 * 2. Protected routes — wrapped in AuthGuard with RootLayout
 * 3. Already-migrated React pages — lazy-loaded
 * 4. Catch-all (*) → VueBridgePage — temporary bridge for unmigrated routes
 *
 * Usage:
 *   import { reactRouter } from '@/react/router/standalone';
 *   createBrowserRouter(reactRouter);
 */

import { lazy, Suspense } from "react";
import {
  createBrowserRouter,
  type RouteObject,
} from "react-router-dom";
import { AuthGuard } from "@/react/pages/auth/AuthGuard";
import { RootLayout } from "@/react/layouts/RootLayout";

// ─── Lazy-loaded pages ────────────────────────────────────

const VueBridgePage = lazy(() =>
  import("@/react/pages/VueBridgePage").then((m) => ({
    default: m.VueBridgePage,
  }))
);

// ─── Loading fallback ─────────────────────────────────────

function LoadingFallback() {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        height: "100vh",
        width: "100%",
        fontSize: "14px",
        color: "#888",
      }}
    >
      Loading...
    </div>
  );
}

// ─── Route definitions ────────────────────────────────────

const routes: RouteObject[] = [
  // Auth routes — no guard required
  {
    path: "/auth/login",
    element: (
      <Suspense fallback={<LoadingFallback />}>
        <VueBridgePage currentPath="/auth/login" />
      </Suspense>
    ),
  },
  {
    path: "/auth/signup",
    element: (
      <Suspense fallback={<LoadingFallback />}>
        <VueBridgePage currentPath="/auth/signup" />
      </Suspense>
    ),
  },

  // Protected routes — AuthGuard + RootLayout
  {
    element: (
      <AuthGuard>
        <RootLayout />
      </AuthGuard>
    ),
    children: [
      // ─── Migrated React routes go here ─────────
      // Example (uncomment when ready):
      // {
      //   path: "/settings",
      //   element: <Suspense fallback={<LoadingFallback />}><SettingsPage /></Suspense>,
      // },

      // ─── Catch-all: delegate to Vue for unmigrated routes ─────
      {
        path: "*",
        element: (
          <Suspense fallback={<LoadingFallback />}>
            <VueBridgeCatchAll />
          </Suspense>
        ),
      },
    ],
  },
];

/**
 * VueBridgeCatchAll reads the current URL and passes it to VueBridgePage.
 * This ensures the Vue Router receives the correct path for unmigrated routes.
 */
function VueBridgeCatchAll() {
  const currentPath =
    typeof window !== "undefined"
      ? window.location.pathname + window.location.search
      : "/";

  return <VueBridgePage currentPath={currentPath} />;
}

// ─── Router instance ──────────────────────────────────────

export const reactShellRouter = createBrowserRouter(routes);

// i18n: i18next | use t("key") from useTranslation()
/**
 * React Shell — standalone main entry point.
 *
 * This is the entry point for when the frontend runs in standalone mode
 * (AUTH_MODE=token, separate API backend). It bootstraps React Router
 * with AuthGuard and the Vue bridge for unmigrated routes.
 *
 * In embedded mode (the default), the existing main.ts with Vue bootstrapping
 * is used instead. This file is conditionally loaded based on configuration.
 *
 * Usage in vite.config.ts:
 *   input: {
 *     main: resolve(__dirname, "index.html"),             // Vue entry (default)
 *     standalone: resolve(__dirname, "standalone.html"),   // React entry
 *   }
 */

import "react";
import "react-dom/client";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router-dom";
import { reactShellRouter } from "@/react/router/standalone";

// Initialize the React app shell.
const container = document.getElementById("app");
if (!container) {
  throw new Error("#app container not found — check index.html");
}

const root = createRoot(container);
root.render(<RouterProvider router={reactShellRouter} />);

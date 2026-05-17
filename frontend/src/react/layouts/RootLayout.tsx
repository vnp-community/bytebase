// i18n: i18next | use t("key") from useTranslation()
/**
 * RootLayout — Top-level React layout with sidebar/header chrome.
 *
 * This is the outermost layout for the React Router shell.
 * It wraps an <Outlet /> for child routes and provides the
 * persistent navigation chrome (sidebar + header).
 *
 * In the current migration phase, most routes are delegated to
 * VueBridgePage, which renders the Vue app inside this layout.
 * As routes are migrated to React, they replace the bridge
 * with native React components.
 */

import { Outlet } from "react-router-dom";

export function RootLayout() {
  return (
    <div
      id="react-root-layout"
      style={{
        display: "flex",
        flexDirection: "column",
        minHeight: "100vh",
        width: "100%",
      }}
    >
      {/* Main content area — Outlet renders the matched child route */}
      <main
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          overflow: "auto",
        }}
      >
        <Outlet />
      </main>
    </div>
  );
}

// i18n: i18next | use t("key") from useTranslation()
/**
 * AuthGuard — React component that protects routes requiring authentication.
 *
 * In cookie mode: checks for the presence of the access-token cookie.
 * In token mode: checks tokenManager.isAuthenticated().
 *
 * Redirects to /auth/login when unauthenticated.
 * Renders children when authenticated.
 */

import { useEffect, useState, type ReactNode } from "react";
import { getEnvConfig } from "@/config/env";
import { getAccessToken } from "@/auth/token-manager";

function getCookie(name: string): string | null {
  const cookies = document.cookie.split(";");
  for (const cookie of cookies) {
    const [key, value] = cookie.trim().split("=");
    if (key === name) {
      return value;
    }
  }
  return null;
}

function isUserAuthenticated(): boolean {
  const { authMode } = getEnvConfig();

  if (authMode === "token") {
    return getAccessToken() !== null;
  }

  // Cookie mode: check for access-token cookie presence.
  // Note: HttpOnly cookies are NOT accessible via JS, so we check for
  // the non-HttpOnly indicator or rely on a previous successful API call.
  // In practice, the Vue app's useAuthStore().fetchCurrentUser() handles this.
  // Here we do a best-effort check.
  return getCookie("access-token") !== null;
}

interface AuthGuardProps {
  children: ReactNode;
  /** Custom login path. Defaults to "/auth/login". */
  loginPath?: string;
}

export function AuthGuard({
  children,
  loginPath = "/auth/login",
}: AuthGuardProps) {
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);

  useEffect(() => {
    const isAuth = isUserAuthenticated();
    if (!isAuth) {
      // Redirect to login, preserving the current URL as redirect target.
      const currentPath = window.location.pathname + window.location.search;
      const redirectUrl = `${loginPath}?redirect=${encodeURIComponent(currentPath)}`;
      window.location.href = redirectUrl;
      return;
    }
    setAuthenticated(true);
    setChecked(true);
  }, [loginPath]);

  if (!checked || !authenticated) {
    // Render nothing while checking auth or redirecting.
    return null;
  }

  return <>{children}</>;
}

/**
 * 3-tier environment config reader.
 *
 * Priority (highest → lowest):
 *   1. Runtime: `window.__ENV__` (set by public/env-config.js, modifiable at deploy-time)
 *   2. Build-time: `import.meta.env.VITE_*` (baked into the bundle)
 *   3. Default: hardcoded fallbacks (same-origin, cookie mode)
 *
 * Usage:
 *   import { getEnvConfig } from '@/config/env';
 *   const { apiUrl, authMode } = getEnvConfig();
 */

declare global {
  interface Window {
    __ENV__?: {
      API_URL?: string;
      AUTH_MODE?: string;
    };
  }
}

export type AuthMode = "token" | "cookie";

export interface EnvConfig {
  /** Base URL for the Bytebase API. Empty = same-origin. */
  apiUrl: string;
  /** Auth token delivery mode: 'cookie' (default) or 'token' (Bearer). */
  authMode: AuthMode;
}

/**
 * Read the 3-tier config.
 * Called at app startup — result is stable for the lifetime of the page.
 */
export function getEnvConfig(): EnvConfig {
  const runtime = window.__ENV__ || {};

  const apiUrl =
    runtime.API_URL ||
    (import.meta.env.VITE_API_URL as string | undefined) ||
    "";

  const rawAuthMode =
    runtime.AUTH_MODE ||
    (import.meta.env.VITE_AUTH_MODE as string | undefined) ||
    "cookie";
  const authMode: AuthMode = rawAuthMode === "token" ? "token" : "cookie";

  return { apiUrl, authMode };
}

/**
 * Returns true when the frontend is deployed standalone (separate from backend).
 * In standalone mode, API_URL is set and we typically use token auth.
 */
export function isStandaloneMode(): boolean {
  return getEnvConfig().apiUrl !== "";
}

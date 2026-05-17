/**
 * JWT Bearer Token Manager for standalone frontend deployment.
 *
 * When AUTH_MODE is 'token', this manager handles:
 * - Storing access token in memory (NOT localStorage — prevents XSS leakage)
 * - Storing refresh token in localStorage (acceptable: opaque, single-use)
 * - Auto-refreshing access token 1 minute before JWT expiry
 * - Creating ConnectRPC transport with Bearer header injection
 * - Redirecting to login on refresh failure
 */

import { createConnectTransport } from "@connectrpc/connect-web";
import type { Transport, Interceptor } from "@connectrpc/connect";
import { getEnvConfig } from "@/config/env";
import { storageService } from "@/utils/storage-service";

const REFRESH_TOKEN_KEY = "refresh-token";
const REFRESH_TOKEN_NAMESPACE = "auth";
const REFRESH_TOKEN_TTL_MS = 7 * 24 * 60 * 60 * 1000; // 7 days
const REFRESH_MARGIN_MS = 60 * 1000; // 1 minute before expiry

// Legacy keys for migration cleanup
const LEGACY_KEYS = ["bb_refresh_token", "bb_rt_enc"];




/** In-memory access token — never written to localStorage. */
let accessToken: string | null = null;
let refreshTimerId: ReturnType<typeof setTimeout> | null = null;

// ─── Public API ──────────────────────────────────────────────

/** Returns the current access token, or null if not authenticated. */
export function getAccessToken(): string | null {
  return accessToken;
}

/** Store tokens after a successful login (token mode). */
export async function setTokens(access: string, refresh: string): Promise<void> {
  accessToken = access;
  // Clean up any legacy plaintext tokens
  for (const legacyKey of LEGACY_KEYS) {
    try { localStorage.removeItem(legacyKey); } catch { /* ignore */ }
  }
  await storageService.save(REFRESH_TOKEN_KEY, refresh, {
    namespace: REFRESH_TOKEN_NAMESPACE,
    encrypt: true,
    ttlMs: REFRESH_TOKEN_TTL_MS,
  });
  scheduleRefresh(access);
}

export async function getRefreshToken(): Promise<string | null> {
  // Try loading from StorageService (encrypted)
  const token = await storageService.load<string | null>(
    REFRESH_TOKEN_KEY,
    { namespace: REFRESH_TOKEN_NAMESPACE, encrypt: true },
    null
  );
  if (token) return token;

  // Graceful migration: clear any legacy keys
  for (const legacyKey of LEGACY_KEYS) {
    try { localStorage.removeItem(legacyKey); } catch { /* ignore */ }
  }
  return null;
}

/** Clear all tokens and cancel pending refresh. */
export function clearTokens(): void {
  accessToken = null;
  // Clear entire auth namespace (covers refresh-token + any future auth keys)
  storageService.clearNamespace(REFRESH_TOKEN_NAMESPACE);
  // Clean up legacy keys
  for (const legacyKey of LEGACY_KEYS) {
    try { localStorage.removeItem(legacyKey); } catch { /* ignore */ }
  }
  if (refreshTimerId) {
    clearTimeout(refreshTimerId);
    refreshTimerId = null;
  }
}

/** Check if the user has a valid token session. */
export function isAuthenticated(): boolean {
  return accessToken !== null;
}

/**
 * Create a ConnectRPC transport that injects the Bearer token
 * into every request's Authorization header.
 */
export function createAuthenticatedTransport(
  baseUrl?: string
): Transport {
  const config = getEnvConfig();
  const url = baseUrl || config.apiUrl || "";

  const bearerInterceptor: Interceptor = (next) => async (req) => {
    if (accessToken) {
      req.header.set("Authorization", `Bearer ${accessToken}`);
    }
    // Signal to backend that we want token-based auth responses.
    req.header.set("X-Auth-Mode", "token");
    return next(req);
  };

  return createConnectTransport({
    baseUrl: url,
    interceptors: [bearerInterceptor],
  });
}

// ─── Refresh Logic ───────────────────────────────────────────

/** Schedule an automatic token refresh based on the JWT `exp` claim. */
function scheduleRefresh(token: string): void {
  if (refreshTimerId) {
    clearTimeout(refreshTimerId);
  }

  const exp = parseJwtExp(token);
  if (!exp) return;

  const msUntilExpiry = exp * 1000 - Date.now();
  const refreshIn = Math.max(msUntilExpiry - REFRESH_MARGIN_MS, 0);

  refreshTimerId = setTimeout(() => {
    refreshAccessToken().catch(() => {
      // On failure, redirect to login.
      clearTokens();
      window.location.href = "/auth/login";
    });
  }, refreshIn);
}

/** Refresh the access token using the stored refresh token. */
export async function refreshAccessToken(): Promise<void> {
  const refreshToken = await getRefreshToken();
  if (!refreshToken) {
    throw new Error("No refresh token available");
  }

  const config = getEnvConfig();
  const baseUrl = config.apiUrl || "";

  const res = await fetch(`${baseUrl}/v1/auth/refresh`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Auth-Mode": "token",
    },
    body: JSON.stringify({ refreshToken }),
  });

  if (!res.ok) {
    throw new Error(`Token refresh failed: ${res.status}`);
  }

  const data = await res.json();
  if (data.accessToken && data.refreshToken) {
    await setTokens(data.accessToken, data.refreshToken);
  } else if (data.token) {
    // Fallback for single-token response.
    accessToken = data.token;
    scheduleRefresh(data.token);
  } else {
    throw new Error("Invalid refresh response");
  }
}

// ─── JWT Parsing ─────────────────────────────────────────────

/** Extract the `exp` claim (seconds since epoch) from a JWT. */
function parseJwtExp(token: string): number | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    const payload = JSON.parse(atob(parts[1]));
    return typeof payload.exp === "number" ? payload.exp : null;
  } catch {
    return null;
  }
}

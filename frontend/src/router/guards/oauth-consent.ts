/**
 * OAuth Consent Guard (TASK-W-024)
 *
 * Ensures unauthenticated users cannot access the OAuth consent page.
 * Redirects them to signin with a return URL.
 */

import type { RouteLocationNormalized } from "vue-router";
import { useAuthStore } from "@/store";
import type { GuardFn, GuardResult } from "./index";
import { guardContinue, guardNext, guardRedirect } from "./index";
import { AUTH_SIGNIN_MODULE, OAUTH2_CONSENT_MODULE } from "../auth";

export const oauthConsentGuard: GuardFn = (
  to: RouteLocationNormalized
): GuardResult => {
  if (to.name !== OAUTH2_CONSENT_MODULE) return guardContinue();

  const authStore = useAuthStore();
  if (!authStore.isLoggedIn) {
    return guardRedirect({
      name: AUTH_SIGNIN_MODULE,
      query: { redirect: to.fullPath },
    });
  }

  return guardNext();
};

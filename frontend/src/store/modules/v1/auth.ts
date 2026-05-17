import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, createContextValues } from "@connectrpc/connect";
import { useLocalStorage } from "@vueuse/core";
import { uniqueId } from "lodash-es";
import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { authServiceClientConnect } from "@/connect";
import { ignoredCodesContextKey } from "@/connect/context-key";
import { router } from "@/router";
import {
  AUTH_MFA_MODULE,
  AUTH_PASSWORD_RESET_MODULE,
  AUTH_SIGNIN_MODULE,
} from "@/router/auth";
import { SETUP_MODULE } from "@/router/setup";
import { SQL_EDITOR_HOME_MODULE } from "@/router/sqlEditor";
import {
  useActuatorV1Store,
  useAppFeature,
  useDatabaseV1Store,
  useInstanceV1Store,
  useProjectV1Store,
  useUserStore,
  useWorkspaceV1Store,
} from "@/store";
import { UNKNOWN_USER_NAME, unknownUser } from "@/types";
import {
  type LoginRequest,
  LoginRequestSchema,
  SendEmailLoginCodeRequestSchema,
  SignupRequestSchema,
} from "@/types/proto-es/v1/auth_service_pb";
import { DatabaseChangeMode } from "@/types/proto-es/v1/setting_service_pb";
import type { User } from "@/types/proto-es/v1/user_service_pb";
import { storageKeyResetPassword } from "@/utils";
import { clearPageCache } from "@/react/mount";
import { extractUserEmail } from "./common";

export const useAuthStore = defineStore("auth_v1", () => {
  const userStore = useUserStore();
  const authSessionKey = ref<string>(uniqueId());
  const actuatorStore = useActuatorV1Store();
  const unauthenticatedOccurred = ref<boolean>(false);
  // Format: users/{email}. Changes when user email is updated.
  const currentUserName = ref<string | undefined>(undefined);
  // Flag to suppress "logged in as another user" notification during self email update
  const isSelfEmailUpdate = ref(false);

  const isLoggedIn = computed(() => {
    return (
      Boolean(currentUserName.value) &&
      currentUserName.value !== UNKNOWN_USER_NAME
    );
  });

  const currentUserEmail = computed(() =>
    extractUserEmail(currentUserName.value || "")
  );

  const requireResetPassword = computed(() => {
    if (!isLoggedIn.value) {
      return false;
    }
    return useLocalStorage<boolean>(
      storageKeyResetPassword(currentUserEmail.value),
      false
    ).value;
  });

  const setRequireResetPassword = (requireResetPassword: boolean) => {
    if (!isLoggedIn.value) {
      return false;
    }
    const needResetPasswordCache = useLocalStorage<boolean>(
      storageKeyResetPassword(currentUserEmail.value),
      false
    );
    needResetPasswordCache.value = requireResetPassword;
  };

  const getRedirectQuery = () => {
    const query = new URLSearchParams(window.location.search);
    return query.get("redirect");
  };

  // sometimes we have to redirect users even if we don't want to redirect them.
  // for example, the user is forced to reset their password,
  // or the user is using the LDAP to signin.
  const login = async ({
    request,
    redirect = true,
    redirectUrl,
  }: {
    request: LoginRequest;
    redirect?: boolean;
    redirectUrl?: string;
  }) => {
    const resp = await authServiceClientConnect.login(
      create(LoginRequestSchema, {
        ...request,
        web: true,
      }),
      {
        contextValues: createContextValues().set(ignoredCodesContextKey, [
          Code.NotFound,
        ]),
      }
    );
    let nextPage = redirectUrl ?? (getRedirectQuery() || "/");
    if (resp.mfaTempToken) {
      unauthenticatedOccurred.value = false;
      return router.push({
        name: AUTH_MFA_MODULE,
        query: {
          mfaTempToken: resp.mfaTempToken,
          redirect: nextPage,
        },
      });
    }

    const user = await fetchCurrentUser();
    unauthenticatedOccurred.value = !user;

    if (unauthenticatedOccurred.value) {
      return;
    }

    setRequireResetPassword(resp.requireResetPassword);
    await actuatorStore.fetchServerInfo(user?.workspace);
    // Re-fetch the current workspace now that we're authenticated — the
    // workspace store watcher may have tried (and failed) pre-login when
    // workspaceResourceName was already set from the signin page's actuator fetch.
    await useWorkspaceV1Store().fetchCurrentWorkspace();

    // After user login, we need to reset the auth session key.
    authSessionKey.value = uniqueId();

    const mode = useAppFeature("bb.feature.database-change-mode");
    if (mode.value === DatabaseChangeMode.EDITOR) {
      const route = router.resolve({
        name: SQL_EDITOR_HOME_MODULE,
      });
      nextPage = route.fullPath;
    }
    if (resp.requireResetPassword) {
      return router.push({
        name: AUTH_PASSWORD_RESET_MODULE,
        query: {
          redirect: nextPage,
        },
      });
    }
    if (redirect) {
      router.replace(nextPage);
    }
  };

  const signup = async (request: Partial<User>) => {
    await authServiceClientConnect.signup(
      create(SignupRequestSchema, {
        email: request.email,
        title: request.name,
        password: request.password,
      })
    );

    // Signup sets HTTP-only cookies automatically.
    // Fetch the current user and proceed with post-login flow.
    const user = await fetchCurrentUser();
    unauthenticatedOccurred.value = !user;
    if (unauthenticatedOccurred.value) {
      return;
    }

    await actuatorStore.fetchServerInfo(user?.workspace);
    authSessionKey.value = uniqueId();

    if (actuatorStore.enableOnboarding) {
      return router.replace({ name: SETUP_MODULE });
    }

    const mode = useAppFeature("bb.feature.database-change-mode");
    let nextPage = getRedirectQuery() || "/";
    if (mode.value === DatabaseChangeMode.EDITOR) {
      nextPage = router.resolve({ name: SQL_EDITOR_HOME_MODULE }).fullPath;
    }
    router.replace(nextPage);
  };

  const cleanupUserStorage = (email: string) => {
    if (!email) return;
    const keysToRemove: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.endsWith(`.${email}`)) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach((key) => localStorage.removeItem(key));
  };

  const logout = async () => {
    let logoutSuccess = false;
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        await authServiceClientConnect.logout({});
        logoutSuccess = true;
        break;
      } catch (error) {
        console.warn(`[AuthStore] Logout attempt ${attempt + 1} failed:`, error);
        if (attempt < 2) await new Promise(r => setTimeout(r, 500 * (attempt + 1)));
      }
    }
    if (!logoutSuccess) {
      console.error("[AuthStore] All logout attempts failed — server session may persist");
    }
    try {
      // TASK-W-023: Reset domain stores on logout (moved from router auth-page visit)
      useDatabaseV1Store().reset();
      useProjectV1Store().reset();
      useInstanceV1Store().reset();
      try {
        const { useConversationStore } = await import("@/plugins/ai/store");
        useConversationStore().reset();
      } catch {
        // AI plugin may not be loaded — safe to ignore
      }
    } finally {
      cleanupUserStorage(currentUserEmail.value);
      clearPageCache();
      unauthenticatedOccurred.value = false;
      const pathname = location.pathname;
      // Replace and reload the page to clear frontend state directly.
      window.location.href = router.resolve({
        name: AUTH_SIGNIN_MODULE,
        query: {
          redirect:
            getRedirectQuery() ||
            (pathname.startsWith("/auth") ? undefined : pathname),
        },
      }).fullPath;
    }
  };

  const fetchCurrentUser = async () => {
    try {
      const user = await userStore.fetchCurrentUser();
      currentUserName.value = user.name;
      return user;
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
        return undefined; // Expected when not logged in
      }
      console.error("[AuthStore] fetchCurrentUser failed:", error);
      return undefined;
    }
  };

  const sendEmailLoginCode = async (email: string, workspace?: string) => {
    await authServiceClientConnect.sendEmailLoginCode(
      create(SendEmailLoginCodeRequestSchema, { email, workspace })
    );
  };

  // Update currentUserName after self email change.
  // Sets flag to suppress "logged in as another user" notification.
  const updateCurrentUserNameForEmailChange = (newName: string) => {
    isSelfEmailUpdate.value = true;
    currentUserName.value = newName;
  };

  return {
    currentUserName,
    isLoggedIn,
    unauthenticatedOccurred,
    requireResetPassword,
    authSessionKey,
    isSelfEmailUpdate,
    login,
    signup,
    logout,
    fetchCurrentUser,
    setRequireResetPassword,
    sendEmailLoginCode,
    updateCurrentUserNameForEmailChange,
  };
});

export const useCurrentUserV1 = () => {
  const authStore = useAuthStore();
  const userStore = useUserStore();
  return computed(
    () =>
      userStore.getUserByIdentifier(authStore.currentUserName || "") ||
      unknownUser()
  );
};

<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <NConfigProvider
    :locale="generalLang"
    :date-locale="dateLang"
    :theme-overrides="themeOverrides"
    :csp="cspConfig"
  >
    <Watermark />

    <NNotificationProvider
      :max="MAX_NOTIFICATION_DISPLAY_COUNT"
      placement="bottom-right"
    >
      <NDialogProvider>
        <OverlayStackManager>
          <NotificationContext>
            <AuthContext>
              <router-view />
            </AuthContext>
          </NotificationContext>
        </OverlayStackManager>
      </NDialogProvider>
    </NNotificationProvider>
  </NConfigProvider>
</template>

<script lang="ts" setup>
import { Code, ConnectError } from "@connectrpc/connect";
import {
  NConfigProvider,
  NDialogProvider,
  NNotificationProvider,
} from "naive-ui";
import {
  onErrorCaptured,
  onMounted,
  onUnmounted,
  watch,
  watchEffect,
  nextTick,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import Watermark from "@/components/misc/Watermark.vue";
import { dateLang, generalLang, themeOverrides } from "../naive-ui.config";
import AuthContext from "./AuthContext.vue";
import OverlayStackManager from "./components/misc/OverlayStackManager.vue";
import { overrideAppProfile } from "./customAppProfile";
import NotificationContext from "./NotificationContext.vue";
import { locale, t } from "./plugins/i18n";
import {
  type ReactQuickstartResetDetail,
  ReactShellBridgeEvent,
} from "./react/shell-bridge";
import { useNotificationStore, useUIStateStore } from "./store";
import { isDev, setDocumentTitle } from "./utils";
import { localeManager } from "./localeManager";

// Show at most 3 notifications to prevent excessive notification when shit hits the fan.
const MAX_NOTIFICATION_DISPLAY_COUNT = 3;

// TASK-WEAK-001-3: Read CSP nonce from meta tag injected by backend.
// When CSP nonce is enabled, Naive UI uses this nonce for dynamically injected styles.
const cspNonce =
  document
    .querySelector('meta[name="csp-nonce"]')
    ?.getAttribute("content") || "";
const cspConfig =
  cspNonce && cspNonce !== "CSP_NONCE_PLACEHOLDER"
    ? { nonce: cspNonce }
    : undefined;

const route = useRoute();
const router = useRouter();
const notificationStore = useNotificationStore();
const uiStateStore = useUIStateStore();

let unsubscribeLocale: (() => void) | undefined;

const handleReactQuickstartReset = (event: Event) => {
  const keys = (event as CustomEvent<ReactQuickstartResetDetail>).detail?.keys;
  if (!Array.isArray(keys)) {
    return;
  }
  void Promise.all(
    keys
      .filter((key): key is string => typeof key === "string")
      .map((key) =>
        uiStateStore.saveIntroStateByKey({
          key,
          newState: false,
        })
      )
  );
};

const handleOAuthUnknown = () => {
  notificationStore.pushNotification({
    module: "bytebase",
    style: "CRITICAL",
    title: t("oauth.unknown-event"),
  });
};

function handleUnhandledRejection(event: PromiseRejectionEvent) {
  if (event.reason instanceof ConnectError) return; // Already handled
  console.error("[Unhandled Rejection]", event.reason);
  notificationStore.pushNotification({
    module: "bytebase",
    style: "CRITICAL",
    title: "Unexpected error",
    description: isDev() ? String(event.reason) : undefined,
  });
}

onMounted(() => {
  unsubscribeLocale = localeManager.subscribe(() => {
    if (route.meta.title) {
      setDocumentTitle(route.meta.title(route));
    }
  });
  window.addEventListener(
    ReactShellBridgeEvent.quickstartReset,
    handleReactQuickstartReset
  );
  window.addEventListener("bb.oauth.unknown", handleOAuthUnknown);
  window.addEventListener("unhandledrejection", handleUnhandledRejection);
});

onUnmounted(() => {
  if (unsubscribeLocale) unsubscribeLocale();
  window.removeEventListener(
    ReactShellBridgeEvent.quickstartReset,
    handleReactQuickstartReset
  );
  window.removeEventListener("bb.oauth.unknown", handleOAuthUnknown);
  window.removeEventListener("unhandledrejection", handleUnhandledRejection);
});

watchEffect(async () => {
  // Override app profile.
  overrideAppProfile();
});

// Only these codes are explicitly handled by interceptors
const INTERCEPTOR_HANDLED_CODES = [
  Code.Unauthenticated,   // authInterceptor → SessionExpiredSurface
  Code.PermissionDenied,   // authInterceptor → 403 page
  Code.Canceled,           // explicit aborts
];

onErrorCaptured((error: unknown /* , _, info */) => {
  if (error instanceof ConnectError) {
    if (INTERCEPTOR_HANDLED_CODES.includes(error.code)) return;
    console.error("[App] Unhandled ConnectError:", error.code, error.message);
  }

  const err = error as { response?: unknown; stack?: string };
  if (!err.response) {
    notificationStore.pushNotification({
      module: "bytebase",
      style: "CRITICAL",
      title: `Internal error captured`,
      description: isDev() ? err.stack : undefined,
    });
  }
  return false;
});

// TASK-W-028: Shallow field check replaces cloneDeep + isEqual for query preservation.
// Preserve specific query fields when navigating between pages.
watch(route, (current, prev) => {
  const preservable = current.meta.preserveQuery as string[] | undefined;
  if (!preservable || preservable.length === 0) return;
  let needsUpdate = false;
  const updates: Record<string, string> = {};
  for (const field of preservable) {
    if (!(field in current.query) && prev.query[field]) {
      updates[field] = prev.query[field] as string;
      needsUpdate = true;
    }
  }
  if (!needsUpdate) return;
  nextTick(() => {
    router.replace({ ...current, query: { ...current.query, ...updates } });
  });
}, { flush: "post" });
</script>

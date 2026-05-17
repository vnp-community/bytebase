# TASK-L-007: OAuth Listener Fix + Unhandled Rejection Handler

> **Source**: SOL-LIM-004 §2.4 | **Priority**: P1 | **Effort**: 1h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (2 changes: OAuth listener + unhandledrejection)

## What
1. Move OAuth event listener vào lifecycle hooks (fix HMR leak)
2. Thêm global `unhandledrejection` handler cho React async errors

## Implementation

### Change 1: OAuth listener lifecycle
```diff
+const handleOAuthUnknown = () => {
+  notificationStore.pushNotification({
+    module: "bytebase",
+    style: "CRITICAL",
+    title: t("oauth.unknown-event"),
+  });
+};

-// Outside lifecycle — never removed
-window.addEventListener("bb.oauth.unknown", () => {
-  notificationStore.pushNotification({ ... });
-});

+onMounted(() => {
+  window.addEventListener("bb.oauth.unknown", handleOAuthUnknown);
+  window.addEventListener("unhandledrejection", handleUnhandledRejection);
+});
+
+onUnmounted(() => {
+  window.removeEventListener("bb.oauth.unknown", handleOAuthUnknown);
+  window.removeEventListener("unhandledrejection", handleUnhandledRejection);
+});
```

### Change 2: Unhandled rejection handler (NEW)
```typescript
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
```

## AC
- [ ] OAuth listener added in `onMounted`, removed in `onUnmounted`
- [ ] HMR no longer creates duplicate OAuth listeners
- [ ] Unhandled React async errors → CRITICAL notification
- [ ] ConnectErrors in async code are NOT double-notified

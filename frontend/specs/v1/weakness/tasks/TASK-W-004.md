# TASK-W-004: OAuth Listener Lifecycle

> **Source**: SOL-WEAK-003 §3.6 | **Priority**: P1 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (~L154-160)

## What
Move `bb.oauth.unknown` event listener into `onMounted`/`onUnmounted` lifecycle to prevent leak.

## Implementation
```diff
+const handleOAuthUnknown = () => {
+  notificationStore.pushNotification({
+    module: "bytebase",
+    style: "WARN",
+    title: t("oauth.unknown-event"),
+  });
+};

-window.addEventListener("bb.oauth.unknown", () => {
-  notificationStore.pushNotification({ ... });
-});

+onMounted(() => {
+  window.addEventListener("bb.oauth.unknown", handleOAuthUnknown);
+});
+onUnmounted(() => {
+  window.removeEventListener("bb.oauth.unknown", handleOAuthUnknown);
+});
```

## AC
- [ ] OAuth listener added in `onMounted`
- [ ] OAuth listener removed in `onUnmounted`
- [ ] No duplicate listeners on HMR reload

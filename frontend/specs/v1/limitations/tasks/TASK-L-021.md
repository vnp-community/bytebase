# TASK-L-021: Wire Vue i18n + React i18next to LocaleManager

> **Source**: SOL-LIM-007 §1.1 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: L-020

## Scope
- **EDIT** `src/plugins/i18n.ts` (subscribe vue-i18n)
- **EDIT** `src/react/i18n.ts` (subscribe i18next, remove CustomEvent)

## What
Wire cả 2 i18n systems vào `localeManager` để bidirectional sync. Xóa CustomEvent-based locale sync.

## Implementation

### File 1: `src/plugins/i18n.ts`
```diff
+import { localeManager } from "@/localeManager";

 // After i18n.global initialization
+localeManager.subscribe((newLocale) => {
+  i18n.global.locale.value = newLocale;
+});
+
+export function setLocale(newLocale: string) {
+  localeManager.setLocale(newLocale);
+}
```

### File 2: `src/react/i18n.ts`
```diff
+import { localeManager } from "@/localeManager";

 // After i18next.init()
+localeManager.subscribe(async (newLocale) => {
+  if (i18next.language !== newLocale) {
+    await i18next.changeLanguage(newLocale);
+  }
+});

-// Remove CustomEvent-based sync
-window.addEventListener("bb.react-locale-change", ...);
```

## AC
- [ ] Vue locale change → React locale updated (bidirectional)
- [ ] React locale change → Vue locale updated (bidirectional)
- [ ] No CustomEvent used for locale sync
- [ ] No duplicate locale change events

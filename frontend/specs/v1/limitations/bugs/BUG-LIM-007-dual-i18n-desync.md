# BUG-LIM-007 — Dual i18n System Desync và Missing Translations

> **Category**: Internationalization  
> **Severity**: Medium  
> **Impact**: UI Text Mismatch, Translation Drift, Maintenance Overhead  
> **Affected Files**: `src/plugins/i18n.ts`, `src/react/i18n.ts`, `src/locales/`, `src/react/locales/`

---

## 1. Mô Tả Vấn Đề

### 1.1 Locale Sync Qua CustomEvent — One-Way và Lossy

```typescript
// shell-bridge.ts
export function emitReactLocaleChange(lang: string) {
  window.dispatchEvent(
    new CustomEvent(ReactShellBridgeEvent.localeChange, { detail: lang })
  );
}

// App.vue:76-83
const handleReactLocaleChange = (event: Event) => {
  const lang = (event as CustomEvent<unknown>).detail;
  if (typeof lang === "string") {
    locale.value = lang;
  }
};
```

**Vấn đề**: Locale sync là **React → Vue** direction qua `CustomEvent`. Nhưng:
- Nếu Vue thay đổi locale (ví dụ: qua URL query `?lang=ja-JP`), React i18next KHÔNG tự động sync.
- `ReactPageMount.vue` check `i18nModule.default.language !== locale.value` nhưng chỉ khi component re-render, không real-time.
- Race condition: Vue locale change → React page đang mount → React nhận locale cũ.

### 1.2 Translation Key Drift

Hai hệ thống i18n hoàn toàn độc lập:

| Aspect | Vue (vue-i18n) | React (i18next) |
|---|---|---|
| Source files | `src/locales/*.json` (5 langs × ~120KB) | `src/react/locales/` (separate structure) |
| API | `t("key.path")` | `useTranslation().t("key.path")` |
| Missing key behavior | Fallback to key string | Fallback to key string |
| Compilation | Build-time (VueI18nPlugin) | Runtime loading |

**Rủi ro**:
- Cùng một concept (ví dụ: "database", "project") có thể có translation key khác nhau giữa Vue và React.
- Thêm language mới phải update ở CẢ HAI hệ thống.
- Không có tool tự động verify consistency giữa hai bộ translation.

### 1.3 Missing Translation Fallback Không Nhất Quán

- Vue `vue-i18n` với `strictMessage: false` → fallback silently to key string.
- React i18next → fallback to key string nhưng có thể configured differently.
- User thấy raw key strings (`common.save`, `database.schema.column`) thay vì fallback language.

## 2. Tác Động

| Scenario | Xác suất | Hậu quả |
|---|---|---|
| Vue locale change không sync React | Medium | React components hiển thị ngôn ngữ cũ |
| Translation key inconsistency | High | UX inconsistent — cùng term, khác translation |
| Add new language | Certain | Phải update 2 systems, dễ miss |
| Missing translation key | Medium | Raw key strings visible to users |
| Build-time vs runtime loading mismatch | Low | Vue translations available trước React |

## 3. Root Cause

- Dual i18n là hệ quả trực tiếp của hybrid Vue+React architecture.
- Không có shared translation source of truth.
- Sync mechanism (CustomEvent) là fire-and-forget, không guarantee delivery.

## 4. Khuyến Nghị

1. **Bidirectional sync**: Vue locale change phải trigger React i18next `changeLanguage()`:
   ```typescript
   watch(locale, async (newLocale) => {
     const i18n = await import("@/react/i18n");
     await i18n.default.changeLanguage(newLocale);
   });
   ```

2. **Shared translation source**: Tạo build script extract common keys từ Vue → inject vào React i18next.

3. **CI check script**: Extend `scripts/check-react-i18n.mjs` để cross-validate keys giữa 2 systems.

4. **Unified missing key reporting**: Thêm `missingKeyHandler` cho cả vue-i18n và i18next, log warnings cho developer.

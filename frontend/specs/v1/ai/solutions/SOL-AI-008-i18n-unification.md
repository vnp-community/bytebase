# SOL-AI-008 — i18n Unification Strategy

> **Resolves**: ISS-AI-008 (Hệ Thống i18n Song Song)  
> **Type**: Technical Design Change  
> **Priority**: Medium  
> **Effort**: Medium (~3–4 weeks)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Thống nhất hệ thống i18n từ **2 parallel systems** (vue-i18n + i18next) xuống còn **1 system** (i18next) — loại bỏ hoàn toàn sự nhầm lẫn về key location và API.

---

## 2. Target State

```
Before: vue-i18n (src/locales/) ← Vue components
        i18next (src/react/locales/) ← React components
        CustomEvent sync layer ← locale change propagation

After:  i18next ONLY (src/locales/) ← ALL components
        No sync needed — single source of truth
```

---

## 3. Giải Pháp

### 3.1 Phase 1 — Immediate: File Header Annotation (1 ngày)

Thêm comment vào đầu mỗi file để AI biết dùng system nào:

```typescript
// i18n: vue-i18n | use t('key') from useI18n()
// Vue component using vue-i18n
<script setup lang="ts">
import { useI18n } from "vue-i18n";
const { t } = useI18n();
</script>
```

```typescript
// i18n: i18next | use t('key') from useTranslation()
// React component using i18next
import { useTranslation } from "react-i18next";
const { t } = useTranslation();
```

### 3.2 Phase 2 — Key Namespace Document

Tạo `.ai-context/I18N_GUIDE.md`:

```markdown
# i18n Guide for AI

## Current State (Transition Period)
Two i18n systems exist simultaneously:

| System | Files | Used In |
|---|---|---|
| `vue-i18n` | `src/locales/en-US.json` (121KB) | Vue .vue files |
| `i18next` | `src/react/locales/en-US/` | React .tsx files |

## Finding the Right Key

**For Vue files (.vue):**
1. Look in `src/locales/en-US.json`
2. Use `t('key.path')` from `useI18n()`

**For React files (.tsx):**
1. Look in `src/react/locales/en-US/`
2. Use `t('key.path')` from `useTranslation()`

## Adding New Translation Key

**Vue component:**
1. Add key to ALL 5 locale files in `src/locales/`
2. Import: `const { t } = useI18n();`
3. Usage: `t('common.save')`

**React component:**
1. Add key to ALL 5 locale files in `src/react/locales/`
2. Import: `const { t } = useTranslation();`
3. Usage: `t('common.save')`

**AFTER MIGRATION (React-only):**
→ Add key to `src/locales/{lang}.json` only
→ Always `const { t } = useTranslation();`
```

### 3.3 Phase 3 — Migrate to i18next (aligned with SOL-AI-001 React migration)

**Strategy**: Migrate translation keys alongside Vue → React page migration.

**Step 1**: Move all React locale files to `src/locales/react/`:

```
src/locales/
├── en-US.json         # Existing vue-i18n (will become unified)
├── zh-CN.json
├── ja-JP.json
├── es-ES.json
├── vi-VN.json
└── react/             # Current i18next files (temporary)
    ├── en-US/
    └── ...
```

**Step 2**: For each Vue page migrated to React, merge its keys into unified `src/locales/*.json`.

**Step 3**: Configure i18next to read from unified `src/locales/`:

```typescript
// src/react/i18n.ts (updated)
import i18n from "i18next";
import { initReactI18next } from "react-i18next";

// Load unified locale files (same as vue-i18n source)
i18n.use(initReactI18next).init({
  resources: {
    "en-US": { translation: await import("@/locales/en-US.json") },
    "zh-CN": { translation: await import("@/locales/zh-CN.json") },
    // ...
  },
  fallbackLng: "en-US",
});
```

**Step 4**: Create Vue i18n ↔ i18next sync util (transition period only):

```typescript
// src/plugins/i18n-sync.ts
// Syncs i18next from vue-i18n during transition
export function syncI18n(vueI18n: I18n, i18next: typeof i18nInstance) {
  watch(() => vueI18n.locale.value, (locale) => {
    i18next.changeLanguage(locale);
  }, { immediate: true });
}
```

**Step 5**: Remove CustomEvent approach entirely (replaced by direct sync).

**Step 6**: After all Vue removed, remove vue-i18n dependency, keep only i18next.

### 3.4 Phase 4 — Full Unification

```typescript
// Final state: ALL components use i18next
// Vue (if any remain): use vue-i18next adapter
// React: native i18next

// src/locales/ = single source of truth
// No CustomEvent, no sync utilities
// AI always does: const { t } = useTranslation();
```

### 3.5 Lint Rule — Detect Wrong i18n System Usage

```javascript
// Flag: useI18n() used in .tsx files (should use useTranslation)
// Flag: useTranslation() used in .vue files (should use useI18n)
export const correctI18nSystem = {
  create(context) {
    const isVueFile = context.filename.endsWith(".vue");
    const isReactFile = context.filename.endsWith(".tsx");
    return {
      CallExpression(node) {
        if (isReactFile && node.callee.name === "useI18n") {
          context.report({ node, message: "Use useTranslation() not useI18n() in React files" });
        }
        if (isVueFile && node.callee.name === "useTranslation") {
          context.report({ node, message: "Use useI18n() not useTranslation() in Vue files" });
        }
      },
    };
  },
};
```

---

## 4. Thay Đổi Technical Design Document

**Cập nhật `specs/technical-design-document.md` Section 3.8 "Internationalization Design":**

- Thêm migration roadmap Phase 1–4
- Update target state: i18next only
- Remove CustomEvent sync section (deprecated)

---

## 5. Implementation Checklist

- [ ] Thêm `// i18n:` header comments vào tất cả Vue và React files
- [ ] Tạo `.ai-context/I18N_GUIDE.md`
- [ ] Thêm ESLint rule `correct-i18n-system`
- [ ] Align key migration với SOL-AI-001 React migration timeline
- [ ] Phase 3: Merge locale files + update i18n.ts config
- [ ] Phase 4: Remove vue-i18n dependency post full-migration

---

## 6. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| i18n systems | 2 | 1 (i18next) |
| Key lookup confusion | High | None |
| Translation key location | 2 directories | 1 directory |
| AI wrong i18n system usage | ~40% | 0% (lint + one system) |

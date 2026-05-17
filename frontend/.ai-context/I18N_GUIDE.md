# I18N Guide for AI Assistants

This document provides instructions on how to handle internationalization (i18n) across the dual-framework architecture (Vue.js + React).

## 1. Current State (vue-i18n vs i18next)

| Feature | Vue 3 (Legacy) | React (New) |
|---|---|---|
| **Library** | `vue-i18n` | `i18next` / `react-i18next` |
| **Locales Dir** | `src/locales/` | `src/react/locales/` |
| **JSON Format** | Nested JSON (`{"common": {"save": "Save"}}`) | Flat JSON with dot notation keys (`{"common.save": "Save"}`) |
| **Usage** | `useI18n()` → `t('key')` | `useTranslation()` → `t('key')` |
| **Hook Import** | `import { useI18n } from "vue-i18n"` | `import { useTranslation } from "react-i18next"` |
| **File Header** | `<!-- i18n: vue-i18n -->` | `// i18n: i18next` |

## 2. Finding the Right Key

**For Vue (`.vue`, `.ts` outside `src/react`):**
Search inside `src/locales/en-US.json`. Keys are nested.
If you need `common.save`, it looks like:
```json
{
  "common": {
    "save": "Save"
  }
}
```

**For React (`.tsx`, `.ts` inside `src/react`):**
Search inside `src/react/locales/en-US/translation.json`. Keys are flat.
If you need `common.save`, it looks like:
```json
{
  "common.save": "Save"
}
```

## 3. Adding a New Key

### Adding to React (`i18next`)
1. Open `src/react/locales/en-US/translation.json`.
2. Add a **flat key** with dot notation.
   ```json
   "setting.members.role": "Role",
   "setting.members.editRole": "Edit Role"
   ```
3. Use it in `.tsx` files:
   ```tsx
   import { useTranslation } from "react-i18next";
   
   export function MembersPage() {
     const { t } = useTranslation();
     return <span>{t("setting.members.role")}</span>;
   }
   ```

### Adding to Vue (`vue-i18n`)
1. Open `src/locales/en-US.json`.
2. Add a **nested key**.
   ```json
   "setting": {
     "members": {
       "role": "Role"
     }
   }
   ```
3. Use it in `.vue` files:
   ```vue
   <script setup lang="ts">
   import { useI18n } from "vue-i18n";
   const { t } = useI18n();
   </script>
   <template>
     <span>{{ t("setting.members.role") }}</span>
   </template>
   ```

## 4. Migration Roadmap

Our ultimate goal is to migrate fully to React and `i18next`.
- **Phase 1-3:** Introduce React, migrate UI components and State architecture.
- **Phase 4:** Dual i18n maintenance. React code uses `src/react/locales`, Vue uses `src/locales`.
- **Phase N:** Once all Vue files are migrated to React, `vue-i18n` and `src/locales/` will be removed.

## 5. Transition Period Rules

While both frameworks exist:
1. **Never share JSON files** between `vue-i18n` and `i18next`. They have different formats (nested vs flat).
2. **Duplicate keys if necessary.** If you migrate a Vue component to React, copy the translation values from `src/locales/en-US.json` into `src/react/locales/en-US/translation.json` as flat keys.
3. **Pay attention to headers.** Always look at the first line of the file.
   - `// i18n: i18next` → React.
   - `<!-- i18n: vue-i18n -->` → Vue.

# TASK-W-029: Merge i18n Shared Namespace

> **Source**: SOL-WEAK-006 §2.2 | **Priority**: P3 | **Effort**: 5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/react/i18n.ts` — load from Vue locale files
- **DELETE** `src/react/locales/{en,zh,ja,es,vi}.json` (duplicated files)
- **NEW** `src/react/locales/react-en.json` (React-only keys, ~10KB)

## What
Make React `i18next` load from same source as Vue `vue-i18n`. Keep only React-specific keys in separate small file.

## Implementation — see SOL-WEAK-006 §2.2
```typescript
import enUS from "@/locales/en-US.json";
i18next.init({
  resources: { "en-US": { translation: enUS } },
  ns: ["translation", "react"],
  defaultNS: "translation",
});
```

## AC
- [x] React loads translations from `src/locales/` (same as Vue)
- [x] Duplicate React locale files removed (~500KB saved)
- [x] React-specific keys in separate `react-{lang}.json`
- [x] All React pages display correct translations

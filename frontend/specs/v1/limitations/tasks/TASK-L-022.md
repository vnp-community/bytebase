# TASK-L-022: i18n Key Sync CI Script

> **Source**: SOL-LIM-007 §1.2 | **Priority**: P3 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `scripts/sync-i18n-keys.mjs`

## What
CI script cross-validates common translation keys giữa Vue (`src/locales/`) và React (`src/react/locales/`). Fail nếu phát hiện key drift.

## Implementation

```javascript
// scripts/sync-i18n-keys.mjs — NEW
import { readFileSync, readdirSync, existsSync } from "fs";
import { resolve, join } from "path";

const VUE_DIR = resolve("src/locales");
const REACT_DIR = resolve("src/react/locales");
const COMMON_PREFIXES = ["common.", "database.", "project.", "instance.", "settings."];

function flattenKeys(obj, prefix = "") {
  return Object.entries(obj).flatMap(([k, v]) => {
    const key = prefix ? `${prefix}.${k}` : k;
    return typeof v === "object" && v !== null ? flattenKeys(v, key) : [key];
  });
}

function loadLocale(dir, locale) {
  const file = join(dir, `${locale}.json`);
  if (!existsSync(file)) return [];
  return flattenKeys(JSON.parse(readFileSync(file, "utf-8")));
}

const vueKeys = loadLocale(VUE_DIR, "en-US");
const reactKeys = loadLocale(REACT_DIR, "en");

const commonVue = vueKeys.filter(k => COMMON_PREFIXES.some(p => k.startsWith(p)));
const commonReact = reactKeys.filter(k => COMMON_PREFIXES.some(p => k.startsWith(p)));

const onlyVue = commonVue.filter(k => !commonReact.includes(k));
const onlyReact = commonReact.filter(k => !commonVue.includes(k));

if (onlyVue.length > 0 || onlyReact.length > 0) {
  console.error("i18n key drift detected:");
  if (onlyVue.length) console.error("  Vue-only:", onlyVue.slice(0, 10));
  if (onlyReact.length) console.error("  React-only:", onlyReact.slice(0, 10));
  process.exit(1);
}
console.log("i18n keys consistent ✓");
```

## AC
- [ ] Script exits 0 when keys are consistent
- [ ] Script exits 1 when key drift detected
- [ ] Only common namespace prefixes are checked (not all keys)
- [ ] Can be added to CI pipeline: `node scripts/sync-i18n-keys.mjs`

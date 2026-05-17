# TASK-AI-P1-005: Thêm ESLint Rules — Named Export + i18n System + Semantic Tokens

> **Source**: SOL-AI-006 §2.3-2.4 + SOL-AI-008 §3.5 | **Priority**: P1 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P1-004  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `eslint-rules/react-page-named-export.mjs`
- **NEW** `eslint-rules/correct-i18n-system.mjs`
- **NEW** `scripts/check-semantic-tokens.mjs`
- **EDIT** `eslint.config.mjs` — register 2 new rules + CI hook
- **EDIT** `package.json` — add `check:tokens` script

## What
3 thêm guardrails: named export enforcement, wrong i18n system detection, raw color class detection.

## Implementation

### `react-page-named-export.mjs`
```javascript
// src/react/pages/**/*.tsx must have named export matching filename
import path from "path";
export const reactPageNamedExport = {
  create(context) {
    if (!context.filename.includes("/pages/")) return {};
    const expected = path.basename(context.filename, ".tsx");
    return {
      Program(node) {
        const hasNamed = node.body.some(
          (s) =>
            s.type === "ExportNamedDeclaration" &&
            (s.declaration?.id?.name === expected ||
              s.specifiers?.some((sp) => sp.exported?.name === expected))
        );
        if (!hasNamed) {
          context.report({ node, message: `Page file must export named function "${expected}". Do not use "export default".` });
        }
      },
    };
  },
};
```

### `correct-i18n-system.mjs`
```javascript
// .vue files must use useI18n(), .tsx files must use useTranslation()
export const correctI18nSystem = {
  create(context) {
    const isVue = context.filename.endsWith(".vue");
    const isReact = context.filename.endsWith(".tsx");
    return {
      CallExpression(node) {
        if (isReact && node.callee.name === "useI18n")
          context.report({ node, message: "Use useTranslation() from react-i18next, not useI18n(), in React files" });
        if (isVue && node.callee.name === "useTranslation")
          context.report({ node, message: "Use useI18n() from vue-i18n, not useTranslation(), in Vue files" });
      },
    };
  },
};
```

### `check-semantic-tokens.mjs` (Node script)
```javascript
// Scans .tsx files for raw Tailwind color classes
// Run: node scripts/check-semantic-tokens.mjs
// Report: file:line class-name → use semantic token instead
const RAW_COLOR = /\b(bg|text|border|ring)-(blue|red|green|yellow|purple|gray|slate|zinc|stone|neutral)-\d{2,3}\b/g;
// Scan src/react/**/*.tsx, report violations with suggested semantic equivalents
```

### `package.json` additions
```json
{
  "scripts": {
    "check:tokens": "node scripts/check-semantic-tokens.mjs",
    "check:all": "pnpm check && pnpm check:tokens"
  }
}
```

### Register in `eslint.config.mjs`
Apply `react-page-named-export` to `src/react/pages/**/*.tsx` glob.
Apply `correct-i18n-system` to all `.tsx` + `.vue` files.

## AC
- [ ] `react-page-named-export` catches `export default function Page()` in pages/
- [ ] `correct-i18n-system` catches `useI18n()` in .tsx files
- [ ] `check:tokens` script reports `bg-blue-500` violations with file + line
- [ ] `pnpm lint` pass on existing codebase (no false positives)
- [ ] `pnpm check:all` works in CI

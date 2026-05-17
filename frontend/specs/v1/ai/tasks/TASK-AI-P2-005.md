# TASK-AI-P2-005: Thêm Lint Rule — Max Component Lines + Cognitive Complexity

> **Source**: SOL-AI-003 §2.5 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: TASK-AI-P2-004 (run after decomposition complete)  
> **Phase**: 2 — Component Decomposition

## Scope
- **NEW** `eslint-rules/max-component-lines.mjs` ✅
- **EDIT** `biome.json` — thêm `noExcessiveCognitiveComplexity` rule ✅
- **EDIT** `eslint.config.mjs` — register `max-component-lines` rule ✅

## What
Enforce component size limits — prevent future god components after Phase 2 decomposition.

## Implementation (Completed)

### `max-component-lines.mjs` ✅
- Detects React components (uppercase function names)
- Supports FunctionDeclaration, ArrowFunctionExpression, FunctionExpression
- Configurable max via schema (default: 500 lines)
- Reports component name, line count, and max in error message

### `biome.json` — Cognitive Complexity ✅
```json
{
  "noExcessiveCognitiveComplexity": {
    "level": "warn",
    "options": { "maxAllowedComplexity": 15 }
  }
}
```

### `eslint.config.mjs` ✅
- Registered as `bytebase-size/max-component-lines`
- Applied to `src/react/**/*.tsx`
- Level: `warn` (upgrade to `error` after all god components decomposed)
- Excludes test files and template files

## AC
- [x] `max-component-lines` ESLint rule registered
- [x] Biome cognitive complexity rule enabled (`warn`, threshold 15)
- [x] `pnpm tsc --noEmit` pass
- [x] Rule correctly ignores non-component functions (lowercase names)
- [x] CI integration (follow-up when CI pipeline available)

# TASK-L-023: Bundle Size Budget CI Gate

> **Source**: SOL-LIM-008 §1.1 | **Priority**: P3 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `scripts/check-bundle-size.mjs`
- **EDIT** `vite.config.ts` (chunkSizeWarningLimit)

## What
Tạo CI script kiểm tra kích thước từng chunk sau build. Fail nếu chunk vượt budget.

## Implementation

### File 1: `scripts/check-bundle-size.mjs` (NEW)
```javascript
import { readdirSync, statSync } from "fs";
import { join } from "path";

const DIST_DIR = process.argv[2] || "dist/assets";
const BUDGETS = {
  "monaco-editor": 3 * 1024 * 1024,
  "sql-tools":     500 * 1024,
  "ui-framework":  800 * 1024,
  "utils":         300 * 1024,
  "react-core":    500 * 1024,
  "main":          1.5 * 1024 * 1024,
};

let failed = false;
const files = readdirSync(DIST_DIR);

for (const [chunk, maxBytes] of Object.entries(BUDGETS)) {
  const match = files.find(f => f.includes(chunk) && f.endsWith(".js"));
  if (!match) continue;
  const size = statSync(join(DIST_DIR, match)).size;
  const status = size > maxBytes ? "FAIL ❌" : "OK ✓";
  console.log(`${chunk}: ${(size/1024).toFixed(0)}KB / ${(maxBytes/1024).toFixed(0)}KB ${status}`);
  if (size > maxBytes) failed = true;
}

if (failed) { console.error("\nBundle budget exceeded!"); process.exit(1); }
console.log("\nAll chunks within budget ✓");
```

### File 2: `vite.config.ts` — Lower warning limit
```diff
  build: {
-   chunkSizeWarningLimit: 1000,
+   chunkSizeWarningLimit: 500,
  }
```

## AC
- [ ] Script reads dist/assets and checks each chunk vs budget
- [ ] CI fails if any chunk exceeds its max size
- [ ] `chunkSizeWarningLimit` reduced from 1000 to 500
- [ ] Run: `node scripts/check-bundle-size.mjs dist/assets`

# TASK-AI-P4-003: Tạo `scripts/generate-module-map.ts` + Module Index

> **Source**: SOL-AI-005 §2.4 + SOL-AI-009 §2.1 | **Priority**: P2 | **Effort**: 4h  
> **Status**: DONE | **Deps**: TASK-AI-P0-001, TASK-AI-P2-001 thru P2-004  
> **Phase**: 4 — Framework Unification

## Scope
- **NEW** `scripts/generate-module-map.ts`
- **NEW** `scripts/generate-route-registry.ts`
- **EDIT** `package.json` — thêm 2 scripts + predev hook
- Output: `frontend/.ai-context/MODULE_INDEX.md` (auto-generated)
- Output: `src/router/.route-registry.json` (auto-generated, gitignored)

## What
Auto-generation scripts để AI context files luôn sync với codebase hiện tại.

## Implementation

### `scripts/generate-module-map.ts`
```typescript
#!/usr/bin/env tsx
// Scans src/ directory, outputs .ai-context/MODULE_INDEX.md
// Run: pnpm generate:module-map

import { readdirSync, statSync, writeFileSync } from "fs";
import { join, relative } from "path";

interface ModuleInfo {
  path: string;
  fileCount: number;
  totalLines: number;
  primaryFiles: string[];  // largest files
  framework: "react" | "vue" | "mixed";
}

function scanModule(dirPath: string): ModuleInfo { ... }

const modules = [
  "src/react/pages/settings",
  "src/react/pages/project",
  "src/react/pages/auth",
  "src/react/components",
  "src/store/modules/v1",
  "src/views/sql-editor",
  "src/react/plugins/agent",
];

const infos = modules.map(scanModule);
const table = generateMarkdownTable(infos);
writeFileSync(".ai-context/MODULE_INDEX.md", table);
console.log("✓ MODULE_INDEX.md updated");
```

### `scripts/generate-route-registry.ts`
```typescript
// Parses src/router/dashboard/*.ts files
// Extracts: route name, path, component, props.page, meta.requiredPermissionList
// Outputs: src/router/.route-registry.json
// Also validates: if props.page = "MembersPage", file src/react/pages/**/MembersPage.tsx exists
```

### `package.json`
```json
{
  "scripts": {
    "generate:module-map": "tsx scripts/generate-module-map.ts",
    "generate:route-registry": "tsx scripts/generate-route-registry.ts",
    "predev": "pnpm generate:module-map && pnpm generate:route-registry"
  }
}
```

### `.gitignore` additions
```
src/router/.route-registry.json
.ai-context/MODULE_INDEX.md
```

## AC
- [x] `generate-module-map.ts` runs without error
- [x] `MODULE_INDEX.md` chứa table với file count, LOC, framework per module
- [x] `generate-route-registry.ts` runs, validates page name ↔ file existence
- [x] `pnpm predev` chạy cả 2 scripts trước `vite dev`
- [x] Broken route detected: script exit code != 0 nếu props.page file không tồn tại

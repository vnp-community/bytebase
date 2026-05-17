# SOL-LIM-008 — Build Pipeline Hardening

> **Resolves**: BUG-LIM-008 (Build System Fragility)  
> **Type**: DevOps / Configuration | **Priority**: Medium | **Effort**: Small (~2 ngày) | **Status**: Proposed

---

## 1. Giải Pháp

### 1.1 Bundle Size Budget (CI Gate)

Thêm `scripts/check-bundle-size.mjs` chạy sau build, verify từng chunk không vượt budget:

| Chunk | Max Size |
|---|---|
| `monaco-editor` | 3MB |
| `sql-tools` | 500KB |
| `ui-framework` | 800KB |
| `utils` | 300KB |
| `main` | 1.5MB |

CI fail nếu bất kỳ chunk vượt limit. Giảm `chunkSizeWarningLimit` từ 1000 về 500 (default Vite).

### 1.2 Consolidate Linting → Biome Only

Migrate ESLint rules sang Biome (đã có `biome.json`). Xóa `eslint.config.mjs`, `@vue/eslint-config-typescript`, `eslint-plugin-vue`. Giữ `@intlify/eslint-plugin-vue-i18n` riêng nếu Biome không cover i18n rules.

**Lý do**: Biome nhanh hơn ~100x so với ESLint, native support cho TypeScript/React/Vue.

### 1.3 Dynamic Chunk Strategy

Thay `manualChunks` string matching bằng package.json name matching:

```typescript
manualChunks: (id) => {
  if (id.includes("node_modules")) {
    const pkg = id.match(/node_modules\/(.+?)\//)?.[1];
    if (!pkg) return undefined;
    // Group by known categories
    if (pkg.startsWith("@codingame/monaco") || pkg === "monaco-editor") return "monaco-editor";
    if (["sql-formatter", "antlr4"].includes(pkg)) return "sql-tools";
    if (pkg === "naive-ui") return "ui-framework";
    if (["lodash-es", "dayjs"].includes(pkg)) return "utils";
    if (["react", "react-dom", "react-i18next", "i18next"].includes(pkg)) return "react-core";
  }
}
```

### 1.4 Naive UI Migration Tracking

Thêm CI metric: `grep -r "naive-ui" src/ --include="*.vue" --include="*.ts" -l | wc -l`. Track số files import Naive UI → alert nếu tăng.

### 1.5 React TSX Transform — Forward tsconfig

```diff
  async transform(code, id) {
    const result = await esbuildTransform(code, {
      loader: "tsx",
      jsx: "automatic",
      jsxImportSource: "react",
-     tsconfigRaw: { compilerOptions: {} },
+     tsconfigRaw: {
+       compilerOptions: {
+         strict: true,
+         target: "ES2022",
+         useDefineForClassFields: true,
+       }
+     },
    });
  }
```

---

## 2. Thay Đổi Architecture Document

**`architecture.md` Section 10.2 Chunk Strategy**: Thêm `react-core` chunk. Document bundle size budgets. Giảm `chunkSizeWarningLimit: 500`.

**`architecture.md` Section 13.2 Known Constraints**: Thêm:
> - **Bundle budget**: CI gate enforces per-chunk size limits

---

## 3. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Bundle size regression detection | Manual | Automated CI gate |
| Lint CI time | ~2x (ESLint + Biome) | ~1x (Biome only) |
| Chunk strategy dependency on naming | Fragile string match | Package name based |
| Naive UI usage tracking | None | CI metric per PR |
| React TSX type safety in dev | Empty tsconfig | Forwarded strict config |

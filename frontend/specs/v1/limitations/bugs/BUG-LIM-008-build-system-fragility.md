# BUG-LIM-008 — Build System Fragility

> **Category**: Build / DevOps  
> **Severity**: Medium  
> **Impact**: Build Failures, Large Bundle, Slow CI/CD  
> **Affected Files**: `vite.config.ts`, `package.json`, `tsconfig.*.json`

---

## 1. Mô Tả Vấn Đề

### 1.1 Node Memory Hard Requirement (8GB)

Tất cả build scripts yêu cầu `--max_old_space_size=8000`. CI runner cần ≥12GB RAM. Developer machine ≤8GB RAM sẽ OOM.

### 1.2 Custom React TSX Transform Bypass TypeScript

`react-tsx-transform` plugin dùng esbuild với `tsconfigRaw: { compilerOptions: {} }` (empty) → type errors chỉ phát hiện qua `tsc --project tsconfig.react.json` riêng.

### 1.3 Dual Linting (ESLint + Biome)

Overlap coverage, configuration conflict potential, double CI time.

### 1.4 Manual Chunk Strategy Fragile

String-based `id.includes("monaco-editor")` matching → dependency rename/upgrade breaks chunking. `chunkSizeWarningLimit: 1000` che đậy bloat.

### 1.5 Patched Naive UI

`naive-ui@2.44.1` cần patch thủ công → mỗi upgrade phải review/reapply patch.

## 2. Khuyến Nghị

1. Bundle size budget CI check.
2. Consolidate linting sang một tool.
3. Dynamic chunk splitting thay manual matching.
4. Track Node memory trong build.
5. Plan Naive UI → React UI kit migration.

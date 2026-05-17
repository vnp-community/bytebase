# SOL-AI-005 — AI Context System + Module Boundary Documentation

> **Resolves**: ISS-AI-005 (Quy Mô Codebase ~307K LOC Vượt Context Của AI)  
> **Type**: Documentation Infrastructure  
> **Priority**: Critical  
> **Effort**: Medium (~2 weeks initial, ongoing maintenance)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Thay vì yêu cầu AI đọc 307K LOC, xây dựng hệ thống **AI Context System** — bộ tài liệu nhỏ gọn (<5K LOC tổng) chứa đủ thông tin để AI làm việc hiệu quả trong từng domain.

---

## 2. Giải Pháp

### 2.1 AI Context Directory Structure

```
frontend/
├── .ai-context/                     # Root AI context (domain-agnostic)
│   ├── INDEX.md                     # Entry point — first file AI reads
│   ├── FRAMEWORK_MAP.md             # Vue vs React file ownership
│   ├── BRIDGE_CONTRACT.md           # Bridge patterns explained
│   ├── PROTOBUF_PATTERNS.md         # Proto-ES construction rules
│   ├── STATE_GUIDE.md               # State management decision tree
│   ├── PATTERNS_CHEATSHEET.md       # All non-standard patterns
│   ├── CONVENTIONS.md               # Naming, styling, overlay rules
│   └── MODULE_INDEX.md              # All modules with file counts + entry points
│
├── src/
│   ├── react/pages/
│   │   ├── settings/
│   │   │   └── .ai-context.md       # Settings module: files, responsibilities, patterns
│   │   ├── project/
│   │   │   └── .ai-context.md       # Project module: files, responsibilities, patterns
│   │   └── auth/
│   │       └── .ai-context.md
│   ├── react/components/
│   │   └── .ai-context.md           # Shared components: what exists, don't recreate
│   ├── store/
│   │   └── .ai-context.md           # Store module: domain → store mapping
│   └── connect/
│       └── .ai-context.md           # ConnectRPC: client list, error policy
```

### 2.2 `INDEX.md` — Master Entry Point

```markdown
# Bytebase Frontend — AI Context Index

## Quy tắc đọc AI Context
1. Luôn đọc file này TRƯỚC khi làm bất cứ task nào
2. Đọc module-specific `.ai-context.md` trong thư mục liên quan
3. KHÔNG đọc `src/types/proto-es/` — dùng `src/types/ai-ref/` thay thế
4. KHÔNG đọc `src/plugins/agent/logic/tools/gen/openapi-index.ts` — generated code

## Decision Tree Nhanh

**Tôi cần thêm một trang mới?**
→ Đọc `.ai-context/FRAMEWORK_MAP.md` + xem template ở `.ai-context/templates/new-page.tsx`

**Tôi cần gọi API?**
→ Đọc `src/types/ai-ref/service-map.ts` để tìm client + store

**Tôi cần thêm state?**
→ Đọc `.ai-context/STATE_GUIDE.md` → quyết định TanStack Query vs Zustand

**Tôi cần fix bug trong component?**
→ Đọc `.ai-context.md` trong thư mục đó trước để hiểu module scope

## Module Quick Reference

| Module | Entry | LOC | Primary Files |
|---|---|---|---|
| Settings | `src/react/pages/settings/` | ~25K | 35 page files |
| Project | `src/react/pages/project/` | ~20K | 30 page files |
| SQL Editor | `src/views/sql-editor/` + `src/react/components/sql-editor/` | ~15K | Vue+React hybrid |
| Auth | `src/react/pages/auth/` | ~5K | 8 files |
| Components (shared) | `src/react/components/` | ~30K | 67 files |
| Store (Vue/legacy) | `src/store/modules/v1/` | ~8K | 33 files |

## Files to NEVER Read Directly (AI excluded):
- `src/types/proto-es/**` (~38K LOC) — use `src/types/ai-ref/` instead
- `src/plugins/agent/logic/tools/gen/openapi-index.ts` (~15K LOC) — generated
- `src/plugins/ai/logic/tools/gen/openapi-index.ts` (~14K LOC) — generated
- `pnpm-lock.yaml` — dependency lock file
```

### 2.3 Per-Module `.ai-context.md` Format

Template chuẩn cho mỗi module:

```markdown
# Module: [Name]

## Scope
Mô tả chính xác module này làm gì, không làm gì.

## Primary Files (read THESE for tasks in this module)
- `PageName.tsx` — main page, 320 LOC
- `hooks/usePageNameData.ts` — data fetching, 85 LOC
- `hooks/usePageNameActions.ts` — CRUD, 110 LOC

## Dependencies
- Store: useDatabaseV1Store (data) / useAuthStore (permissions)
- API Client: databaseServiceClientConnect
- Proto types: use src/types/ai-ref/database.ts (not proto-es)

## Common Tasks
- Add filter: edit `hooks/usePageNameFilters.ts`
- Fix table column: edit `components/PageNameTable.tsx`
- Add new action: edit `hooks/usePageNameActions.ts`

## Prohibited
- Do NOT add useVueState calls here (already migrated to TanStack Query)
- Do NOT use raw z-index (use overlay layering policy)
```

### 2.4 Module Map Auto-generation

Script `scripts/generate-module-map.ts` — chạy khi thêm file mới:

```typescript
// Auto-generates MODULE_INDEX.md từ filesystem
// Reads package.json, tsconfig paths, và directory structure
// Output: .ai-context/MODULE_INDEX.md with file counts, sizes, dependencies
```

```json
// package.json
{
  "scripts": {
    "generate:module-map": "tsx scripts/generate-module-map.ts",
    "predev": "pnpm generate:module-map"  // Auto-update on dev start
  }
}
```

### 2.5 AI-Excludable File Tagging

Thêm marker vào đầu mỗi generated file:

```typescript
// @ai-exclude: This file is auto-generated from Protobuf definitions.
// AI: Do NOT read this file. Use src/types/ai-ref/ instead.
// Generated by: buf generate
// Source: proto/v1/database_service.proto
```

Thêm `.aiignore` file (tương tự `.gitignore`) cho AI tools hỗ trợ:

```
# .aiignore — Files AI agents should NOT read
src/types/proto-es/
src/plugins/agent/logic/tools/gen/
src/plugins/ai/logic/tools/gen/
pnpm-lock.yaml
node_modules/
```

### 2.6 Codebase Size Reduction (Generated Code)

Chuyển proto-es generated code ra khỏi `src/`:

```
Before: src/types/proto-es/  (tracked in git, 38K LOC)
After:  generated/proto-es/  (gitignored, regenerated in CI)
```

Lợi ích:
- `src/` giảm từ 307K → ~269K LOC
- AI tools không scan `generated/` directory nếu config đúng
- Rõ ràng hơn: "src/ = human-written code"

---

## 3. Thay Đổi Architecture Document

**Thêm Section 14 vào `specs/architecture.md`:**

```markdown
## 14. AI Context System

### 14.1 Context Hierarchy
[Diagram của .ai-context directory structure]

### 14.2 Module Boundary Policy
- Mỗi module có .ai-context.md tại root directory
- .ai-context.md phải được update khi thêm/xóa files

### 14.3 AI-Excluded Files
[List files với @ai-exclude marker]
```

---

## 4. Implementation Checklist

- [ ] Tạo `frontend/.ai-context/` directory với 8 root files
- [ ] Tạo `.aiignore` file
- [ ] Thêm `@ai-exclude` marker vào 86 proto-es files
- [ ] Thêm `@ai-exclude` marker vào 2 openapi-index files
- [ ] Tạo per-module `.ai-context.md` cho 6 primary modules
- [ ] Viết `scripts/generate-module-map.ts`
- [ ] Move proto-es sang `generated/proto-es/` (gitignored)
- [ ] Update `.gitignore` và `tsconfig.json` paths
- [ ] Add `generate:module-map` to package.json scripts

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| LOC AI cần đọc cho bất kỳ task | ~307K (all) | < 5K (context) + module files |
| Time for AI to understand a module | Nhiều prompts | 1–2 files |
| Generated code in AI context window | ~68K LOC | 0 (excluded) |
| Module boundary clarity | None | Explicit .ai-context.md per module |

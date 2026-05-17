# TASK-AI-P0-002: Tạo `.ai-context/STATE_GUIDE.md` + `BRIDGE_CONTRACT.md`

> **Source**: SOL-AI-001 §2.1 + SOL-AI-004 §2.1 | **Priority**: P0 | **Effort**: 2h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001  
> **Phase**: 0 — Quick Wins

## Scope
- **NEW** `frontend/.ai-context/STATE_GUIDE.md`
- **NEW** `frontend/.ai-context/BRIDGE_CONTRACT.md`

## What
Giải thích cho AI khi nào dùng state nào và bridge hoạt động như thế nào — không cần đọc 15K LOC store code.

## Implementation

### 1. `STATE_GUIDE.md`
Nội dung:
```markdown
## State Decision Tree
Server data (từ API)?
  → Dùng TanStack Query hook: useDatabase(name), useProject(name)...
  → KHÔNG dùng Pinia store trực tiếp từ React component

Client/UI state?
  → Dùng Zustand: useAuthStore(), useUIStore()
  → Auth info: useAuthStore().currentUser
  → Locale/theme: useUIStore()

SQL Editor tab state?
  → src/react/stores/sqlEditor (Zustand)

## Cross-framework (Legacy, đang migrate ra)
- useVueState(() => usePiniaStore().field)
  → Chỉ dùng trong files chưa migrate sang TanStack Query
  → Không tạo useVueState mới
```

### 2. `BRIDGE_CONTRACT.md`
Nội dung:
- `useVueState(getter)` — tạo Vue watch subscription, return React state
- `mountReactPage(container, "PageName")` — load React component bằng name lookup
- `shell-bridge.ts` — CustomEvent bus giữa Vue shell và React pages
- Diagram 3-layer: Vue Router → ReactPageMount → React Component
- Warning: Không import Vue composables trực tiếp trong React (dùng useVueState wrapper)

## AC
- [ ] `STATE_GUIDE.md` có decision tree rõ ràng với code examples
- [ ] `BRIDGE_CONTRACT.md` giải thích đủ 3 bridge files với usage rules
- [ ] AI đọc 2 files này có thể quyết định đúng state approach

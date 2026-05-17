# SOL-AI-001 — Unified React-First Architecture

> **Resolves**: ISS-AI-001 (Hybrid Vue + React Framework Gây Nhầm Lẫn Cho AI)  
> **Type**: Architectural Change  
> **Priority**: Critical  
> **Effort**: Large (3–6 months progressive migration)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Loại bỏ sự mập mờ do hybrid framework gây ra bằng cách:
1. **Accelerate Vue → React migration** theo lộ trình đã định trong `AGENTS.md`.
2. **Codify bridge contracts** để AI luôn biết đang làm việc trong context nào.
3. **Eliminate `useVueState`** bằng cách chuyển domain state sang Zustand.

---

## 2. Giải Pháp Kiến Trúc

### 2.1 Phase 1 — AI Context Codification (Ngay lập tức, ~1 tuần)

Tạo file `.ai-context/FRAMEWORK_MAP.md` tại root frontend:

```
frontend/
├── .ai-context/
│   ├── FRAMEWORK_MAP.md        # Vue vs React file mapping
│   ├── BRIDGE_CONTRACT.md      # useVueState + mountReactPage specs
│   ├── STATE_GUIDE.md          # Which store to use for what
│   └── PATTERNS_CHEATSHEET.md  # Non-standard pattern quick reference
```

**Nội dung `FRAMEWORK_MAP.md`:**

```markdown
## Quy tắc tuyệt đối
- File trong `src/views/` + `src/components/` (không có tiền tố react/) = VUE
- File trong `src/react/` = REACT
- File `.vue` = VUE, file `.tsx` = REACT
- Tất cả NEW features → viết REACT
- KHÔNG tạo Vue file mới (trừ bug fix existing)

## Bridge entry points (duy nhất 3 files được phép import Vue trong React context)
- src/react/hooks/useVueState.ts — Vue→React reactivity bridge
- src/react/ReactPageMount.vue — Mount React page vào Vue route
- src/react/shell-bridge.ts — Vue↔React event bus
```

### 2.2 Phase 2 — Eliminate `useVueState` Via Zustand Migration (2–3 tháng)

**Hiện trạng problematic** (68 `useVueState` calls):
```
MembersPage.tsx: 16 calls → useAuthStore, usePermissionStore, useGroupStore...
ConnectionPane.tsx: 15 calls → useSQLEditorStore, useTabStore...
DatabaseObjectExplorer.tsx: 12 calls → useDbSchemaStore, useInstanceStore...
```

**Migration Strategy** — Tạo Zustand mirrors cho top-5 high-usage Pinia stores:

```typescript
// src/react/stores/auth.ts — NEW Zustand mirror
import { create } from "zustand";
import { subscribeWithSelector } from "zustand/middleware";

interface AuthState {
  currentUser: User | null;
  isLoggedIn: boolean;
  // Derived actions
  fetchCurrentUser: () => Promise<void>;
}

export const useAuthStore = create<AuthState>()(
  subscribeWithSelector((set) => ({
    currentUser: null,
    isLoggedIn: false,
    fetchCurrentUser: async () => {
      const user = await authServiceClientConnect.getCurrentUser({});
      set({ currentUser: user, isLoggedIn: true });
    },
  }))
);

// Sync FROM Pinia (one-time bootstrap)
export function syncFromPinia(piniaAuthStore: ReturnType<typeof usePiniaAuthStore>) {
  watch(
    () => piniaAuthStore.currentUser,
    (user) => useAuthStore.setState({ currentUser: user, isLoggedIn: !!user }),
    { immediate: true }
  );
}
```

**Migration priority order** (by `useVueState` call count):
1. `useAuthStore` → Zustand
2. `usePermissionStore` → Zustand  
3. `useInstanceV1Store` → Zustand
4. `useEnvironmentV1Store` → Zustand
5. `useSQLEditorStore` → Zustand

### 2.3 Phase 3 — Vue Shell Replacement (3–6 tháng)

Thay thế Vue Router shell bằng React Router v7 khi đủ pages đã migrate:

```
Current:  Vue Router → ReactPageMount.vue → React Page
Target:   React Router v7 → React Page (direct)
          Vue Router (legacy, only for 100% pure Vue pages)
```

**Điều kiện để thực hiện**: Số Vue-only pages < 20% tổng pages.

### 2.4 Phase 4 — Full React SPA (6+ tháng)

```
Target Architecture:
├── React Router v7 (owns all routing)
├── TanStack Query (data fetching + caching, replaces Pinia cache)
├── Zustand (global state)
├── ConnectRPC (transport, unchanged)
└── Vue (zero files — legacy deleted)
```

---

## 3. Thay Đổi Architecture Document

**Cập nhật `specs/architecture.md` Section 4 "Hybrid Vue + React":**

Thêm **Migration Roadmap** với 4 phases và completion criteria cho từng phase. Ghi rõ:
- Phase hiện tại đang ở đâu
- Đường ranh giới nghiêm cấm: không tạo Vue component mới
- Expected end state: React-only SPA

---

## 4. Implementation Checklist

- [ ] Tạo `.ai-context/FRAMEWORK_MAP.md`
- [ ] Tạo `.ai-context/BRIDGE_CONTRACT.md` với useVueState + mountReactPage specs
- [ ] Tạo Zustand store mirrors cho top-5 Pinia stores
- [ ] Update AGENTS.md với migration phase tracking
- [ ] Lint rule: cấm tạo `.vue` files mới ngoài `src/views/sql-editor/`
- [ ] Add CI check: count Vue files — alert nếu số lượng tăng

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| `useVueState` calls | 68 | < 10 |
| Vue files | 186 | < 50 |
| AI framework confusion errors | High | Near zero (via context files) |
| Bridge layers to trace | 4 | 1 (direct React) |

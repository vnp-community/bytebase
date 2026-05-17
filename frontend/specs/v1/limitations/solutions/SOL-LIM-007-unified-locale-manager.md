# SOL-LIM-007 — Unified Locale Manager

> **Resolves**: BUG-LIM-007 (Dual i18n Desync)  
> **Type**: Architectural Change (i18n)  
> **Priority**: Medium | **Effort**: Medium (~1 tuần) | **Status**: Proposed

---

## 1. Giải Pháp

### 1.1 LocaleManager Singleton

Tạo `src/localeManager.ts` — framework-agnostic pub/sub quản lý locale duy nhất. Cả `vue-i18n` và `i18next` subscribe vào manager → bidirectional sync, loại bỏ CustomEvent.

### 1.2 Shared Key CI Check

Tạo `scripts/sync-i18n-keys.mjs` cross-validate common translation keys giữa `src/locales/` và `src/react/locales/`. Chạy trong CI pipeline.

### 1.3 Unified Missing Key Handler

Cấu hình cả 2 systems dùng chung `missingKeyHandler` → console.warn trong dev mode.

---

## 2. Thay Đổi Architecture

**`architecture.md` Section 11.2**: Thay thế bằng mô tả LocaleManager pattern, xóa CustomEvent locale sync.

**`technical-design-document.md` Section 3.8**: Thay mermaid diagram thành graph LocaleManager → vue-i18n / i18next bidirectional.

---

## 3. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Bidirectional locale sync | No | Yes (LocaleManager) |
| Translation key drift | Undetected | CI check on every PR |
| Missing key handling | Silent | Console warning in dev |

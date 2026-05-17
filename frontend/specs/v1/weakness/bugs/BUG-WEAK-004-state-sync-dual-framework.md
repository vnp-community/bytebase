# BUG-WEAK-004: Dual Framework State Synchronization Issues

> **Severity**: MEDIUM-HIGH  
> **Category**: State Management / Architecture Bug  
> **Status**: OPEN | **Created**: 2026-05-13

## 1. Mô tả

Kiến trúc hybrid Vue + React duy trì 2 hệ thống state (Pinia + Zustand) với đồng bộ qua `useVueState` hook và `CustomEvent` bridge — gây inconsistency.

## 2. Chi tiết lỗi

### 2.1 useVueState Deep Tracking Opt-in Fragile
- **File**: `src/react/hooks/useVueState.ts` — `deep` flag mặc định `false`
- Pinia mutate entities in-place → shallow tracking bỏ lỡ mutations
- Components phải biết Pinia internals để chọn đúng `deep` flag

### 2.2 Dual Data Fetching Race
- **File**: `src/react/hooks/useAppState.ts`
- `useCurrentUser()` (Zustand) và `useCurrentUserV1()` (Pinia) fetch cùng API independently
- Duplicate network requests + transient state inconsistency

### 2.3 Shell Bridge Event No Delivery Guarantee
- **File**: `src/react/shell-bridge.ts`
- `CustomEvent<unknown>` trên `window` — untyped, no retry, no replay
- Events mất nếu listener chưa attached

### 2.4 Redundant Fetches trong useAppState Hooks
- **File**: `src/react/hooks/useAppState.ts` (L50-89)
- Mỗi component mount trigger `loadSubscription()` dù data đã có
- 12+ selector calls trong 1 hook → nhiều re-renders

## 3. Đề xuất Fix
1. Pinia stores là single source of truth; React dùng `useVueState` thay vì duplicate
2. `useVueState` deep by default
3. Typed shell bridge thay vì raw `CustomEvent<unknown>`
4. Thêm `isLoaded` flag, skip fetch nếu data đã có

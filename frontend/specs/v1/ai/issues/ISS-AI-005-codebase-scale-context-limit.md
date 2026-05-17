# ISS-AI-005 — Quy Mô Codebase ~307K LOC Vượt Khả Năng Xử Lý Context Của AI

> **Category**: Scale Limitation  
> **Severity**: Critical  
> **Impact**: Codebase Comprehension, Cross-file Reasoning, Holistic Refactoring  
> **Affected Area**: Toàn bộ `src/` — 1,465 files, ~307,651 LOC

---

## 1. Mô Tả Vấn Đề

### 1.1 Thống Kê Quy Mô

| Metric | Giá trị |
|---|---|
| **Tổng files** | 1,465 (.ts + .tsx + .vue) |
| **Tổng LOC** | 307,651 |
| **Vue files** (.vue) | 186 |
| **React TSX** (.tsx) | 628 |
| **TypeScript** (.ts) | 651 |
| **Test files** | 179 |
| **Proto-ES generated** | 86 files, ~38K LOC |
| **Auto-generated (openapi-index)** | 2 files, ~30K LOC |

### 1.2 Context Window vs Codebase Size

| AI Model Context | % Codebase Coverage |
|---|---|
| 32K tokens (~24K LOC) | ~8% |
| 128K tokens (~96K LOC) | ~31% |
| 200K tokens (~150K LOC) | ~49% |

Ngay cả AI với 200K context cũng chỉ xử lý ~50% codebase. Phần còn lại là "blind spot".

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **"Find all usages"** | AI không thể scan 1465 files |
| **Cross-file refactoring** | Rename ảnh hưởng 50+ files — AI dễ bỏ sót |
| **Impact analysis** | Ripple effect qua 10+ layers — AI chỉ thấy immediate dependencies |
| **Dead code detection** | Không verify unused exports khi không thấy toàn bộ importers |

## 3. Khuyến Nghị

1. **Module boundary docs**: Tạo `MODULE_MAP.md` per domain.
2. **Generated code exclusion**: Tag `proto-es/` và `openapi-index.ts` (~68K LOC) để AI skip.
3. **Scope narrowing**: Provide explicit file list cho mỗi AI task.
4. **AI-friendly barrel files**: Tạo `index.ts` per domain với JSDoc public API.

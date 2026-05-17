# TASK-AI-P0-001: Tạo `.ai-context/` Directory và INDEX.md

> **Source**: SOL-AI-005 §2.1-2.2 | **Priority**: P0 | **Effort**: 2h  
> **Status**: ✅ DONE | **Deps**: —  
> **Phase**: 0 — Quick Wins

## Scope
- **NEW** `frontend/.ai-context/INDEX.md`
- **NEW** `frontend/.ai-context/FRAMEWORK_MAP.md`

## What
Tạo AI context directory entry point với 2 file cốt lõi để AI có thể định hướng nhanh trong codebase.

## Implementation

### 1. Tạo `frontend/.ai-context/INDEX.md`
Nội dung bao gồm:
- Decision tree: "Tôi cần thêm trang mới?" → "Tôi cần gọi API?" → "Tôi cần fix bug?"
- Module quick reference table (Settings, Project, SQL Editor, Auth, Components, Store)
- Files to NEVER read: `src/types/proto-es/**`, `src/plugins/agent/logic/tools/gen/openapi-index.ts`
- Links đến tất cả `.ai-context/*.md` files

### 2. Tạo `frontend/.ai-context/FRAMEWORK_MAP.md`
Nội dung:
- Quy tắc tuyệt đối: file trong `src/views/` + `src/components/` = VUE, file trong `src/react/` = REACT
- `.vue` = VUE, `.tsx` = REACT
- ALL new features → React
- Bridge entry points (chỉ 3 files được phép import Vue trong React)
- Migration phase tracker (Phase 0: current state)

## AC
- [ ] `frontend/.ai-context/INDEX.md` tồn tại với decision tree và module reference
- [ ] `frontend/.ai-context/FRAMEWORK_MAP.md` tồn tại với file ownership rules rõ ràng
- [ ] Tất cả paths trong INDEX.md đều hợp lệ

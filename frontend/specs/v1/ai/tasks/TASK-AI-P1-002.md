# TASK-AI-P1-002: Tạo `src/types/ai-ref/` — Remaining 7 Domain Types

> **Source**: SOL-AI-002 §2.1 | **Priority**: P1 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P1-001  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `src/types/ai-ref/issue.ts`
- **NEW** `src/types/ai-ref/plan.ts`
- **NEW** `src/types/ai-ref/rollout.ts`
- **NEW** `src/types/ai-ref/user.ts`
- **NEW** `src/types/ai-ref/setting.ts`
- **NEW** `src/types/ai-ref/sql.ts`
- **NEW** `src/types/ai-ref/policy.ts`
- **EDIT** `src/types/ai-ref/index.ts` — add exports

## What
Hoàn thiện AI reference layer với 7 domain types còn lại.

## Implementation

Mỗi file theo pattern từ TASK-AI-P1-001:
- Header comment với path đến full proto-es file
- Condensed interface với top-level fields + JSDoc descriptions
- `{DOMAIN}_CLIENT` constant
- `{DOMAIN}_UPDATE_MASK_FIELDS` constant (nếu applicable)
- JSDoc listing common service methods

### `issue.ts` — IssueRef interface
Fields: name, title, status (OPEN/DONE/CANCELED), type, creator, assignee, project, approvalStatus, approvalTemplates

### `plan.ts` — PlanRef interface  
Fields: name, title, description, creator, specs[] (summary only: id, type, sheet)

### `rollout.ts` — RolloutRef interface
Fields: name, plan, stages[] (summary: id, environment, tasks[] summary)

### `user.ts` — UserRef interface
Fields: name, email, title, role, mfaEnabled, phone, profile

### `setting.ts` — SettingRef interface
Top-level Settings (workspace name, logo, onboarding, smtp, etc.) — chỉ common fields

### `sql.ts` — QueryResult, ExportRequest patterns

### `policy.ts` — Policy types (mask, backup, access control)

## AC
- [ ] 7 files tạo xong với condensed interfaces
- [ ] `index.ts` export tất cả 10 domain types
- [ ] TypeScript compiles
- [ ] Mỗi file < 100 LOC

# TASK-AI-P0-005: Tạo `.ai-context/NEW_PAGE_PLAYBOOK.md` + `GUARD_FLOWCHART.md`

> **Source**: SOL-AI-009 §2.2-2.3 | **Priority**: P0 | **Effort**: 2h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001  
> **Phase**: 0 — Quick Wins

## Scope
- **NEW** `frontend/.ai-context/NEW_PAGE_PLAYBOOK.md`
- **NEW** `frontend/.ai-context/GUARD_FLOWCHART.md`

## What
Step-by-step playbook để AI thêm page mới mà không sai naming convention, routing, hay glob pattern.

## Implementation

### 1. `NEW_PAGE_PLAYBOOK.md`
5-step checklist:

**Step 1 — Tạo file**: `src/react/pages/{section}/YourPageName.tsx`, copy từ template, named export bắt buộc.

**Step 2 — Register route**: Router file theo section (workspaceSetting.ts / projectV1.ts / workspace.ts / instance.ts). Code snippet với đầy đủ `name`, `path`, `component`, `props: { page: "YourPageName" }`, `meta.requiredPermissionList`.

**Step 3 — Mount glob**: Check `src/react/mount.ts` — list 5 directories đang được glob. Skip nếu page trong directory đã có.

**Step 4 — Navigation entry**: Sidebar files (DashboardSidebar.tsx / ProjectSidebar.tsx).

**Step 5 — Verify**: `pnpm dev`, navigate to route, check console.

**Common Errors table**: blank page, 404, permission error, component not found — cause + fix.

### 2. `GUARD_FLOWCHART.md`
ASCII flowchart 9 guards:
```
[1] Same path? → ABORT
[2] Error page? → ALLOW
[3] OAuth callback? → ALLOW
[4] Logged in + /auth/*? → REDIRECT /
[5] Not logged in? → REDIRECT /auth/signin
[6] 2FA required? → REDIRECT /auth/2fa-setup
[7] Password reset? → REDIRECT /auth/reset-password
[8] Permission? → CHECK → ALLOW / REDIRECT /403
```
+ debugging tips per symptom.

## AC
- [ ] NEW_PAGE_PLAYBOOK.md có đủ 5 steps với code snippets
- [ ] Code snippets đúng syntax (không có typo)
- [ ] Guard flowchart 9 bước chính xác theo `src/router/guard.ts`
- [ ] Common errors table cover 4 failure cases

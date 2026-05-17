# TASK-AI-P0-003: Tạo `.ai-context/CONNECTRPC_GUIDE.md` + `ERROR_POLICY.md`

> **Source**: SOL-AI-007 §2.1-2.2 | **Priority**: P0 | **Effort**: 2h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001  
> **Phase**: 0 — Quick Wins

## Scope
- **NEW** `frontend/.ai-context/CONNECTRPC_GUIDE.md`
- **NEW** `frontend/.ai-context/ERROR_POLICY.md`

## What
Cung cấp lookup table đầy đủ client → method → pattern để AI không bao giờ generate `fetch()` cho gRPC calls.

## Implementation

### 1. `CONNECTRPC_GUIDE.md`
Nội dung bắt buộc:
- Pattern cơ bản: import từ `@/connect`, call method, updateMask required
- Client lookup table 24+ services (database, project, instance, issue, plan, rollout, user, auth, setting, sql, worksheet, sheet, orgPolicy, reviewConfig, subscription, role, group, identityProvider, auditLog, accessGrant, workloadIdentity, serviceAccount, aiService, cel)
- Code examples: getDatabase, listDatabases, updateDatabase (với updateMask)
- Anti-patterns: ❌ `fetch('/v1/...')`, ❌ `new Database({})`, ❌ plain JSON object

### 2. `ERROR_POLICY.md`
Nội dung:
- Interceptors đã handle: 401, tất cả gRPC error codes, network timeout
- → AI KHÔNG cần try/catch trong components cho standard errors
- Chỉ catch khi cần custom behavior (AlreadyExists, optimistic rollback)
- Pattern: onError NOT needed trong useMutation (interceptor shows notification)
- Token refresh flow: Web Locks → BroadcastChannel → cross-tab coordination

## AC
- [ ] Client lookup table có đủ 24+ services với tên đúng
- [ ] Code examples hoạt động nếu copy-paste vào codebase
- [ ] ERROR_POLICY.md phân biệt rõ khi nào catch, khi nào không

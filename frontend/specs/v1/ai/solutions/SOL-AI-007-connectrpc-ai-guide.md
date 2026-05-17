# SOL-AI-007 — ConnectRPC AI Integration Guide + Error Policy

> **Resolves**: ISS-AI-007 (ConnectRPC + Interceptor Chain Gây Khó Debug)  
> **Type**: Documentation + Tooling  
> **Priority**: Medium  
> **Effort**: Small (~3–5 days)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Loại bỏ hoàn toàn sự nhầm lẫn về ConnectRPC qua:
1. **AI Cheat Sheet** — mapping domain → client → method
2. **Error Policy Document** — khi nào AI NÊN và KHÔNG NÊN catch errors
3. **Lint Rules** — tự động phát hiện fetch() usage và missing updateMask

---

## 2. Giải Pháp

### 2.1 ConnectRPC AI Cheat Sheet

Tạo `.ai-context/CONNECTRPC_GUIDE.md`:

```markdown
# ConnectRPC Usage Guide for AI

## Pattern cơ bản (LUÔN dùng pattern này)

```typescript
// Import client từ src/connect/index.ts
import { databaseServiceClientConnect } from "@/connect";

// Call method — sử dụng await/async
const database = await databaseServiceClientConnect.getDatabase({
  name: "instances/prod-instance/databases/my-db",
});

// List với pagination
const { databases, nextPageToken } = await databaseServiceClientConnect.listDatabases({
  parent: "instances/prod-instance",
  pageSize: 100,
  filter: "environment == 'environments/prod'",
});

// Update — updateMask REQUIRED, list ONLY changed fields
await databaseServiceClientConnect.updateDatabase({
  database: { ...existing, labels: newLabels },
  updateMask: ["labels"],  // ← chỉ list fields đã thay đổi
});
```

## Client Lookup Table

| Domain | Import | Client Name |
|---|---|---|
| Database | `@/connect` | `databaseServiceClientConnect` |
| Project | `@/connect` | `projectServiceClientConnect` |
| Instance | `@/connect` | `instanceServiceClientConnect` |
| Issue | `@/connect` | `issueServiceClientConnect` |
| Plan | `@/connect` | `planServiceClientConnect` |
| Rollout | `@/connect` | `rolloutServiceClientConnect` |
| User | `@/connect` | `userServiceClientConnect` |
| Auth | `@/connect` | `authServiceClientConnect` |
| Setting | `@/connect` | `settingServiceClientConnect` |
| SQL | `@/connect` | `sqlServiceClientConnect` |
| Worksheet | `@/connect` | `worksheetServiceClientConnect` |
| Sheet | `@/connect` | `sheetServiceClientConnect` |
| Environment | → Use `settingServiceClientConnect` | (via SettingService) |
| Policy | `@/connect` | `orgPolicyServiceClientConnect` |
| Review | `@/connect` | `reviewConfigServiceClientConnect` |
| Subscription | `@/connect` | `subscriptionServiceClientConnect` |
| Role | `@/connect` | `roleServiceClientConnect` |
| Group | `@/connect` | `groupServiceClientConnect` |
| IDP | `@/connect` | `identityProviderServiceClientConnect` |
| Audit Log | `@/connect` | `auditLogServiceClientConnect` |
| Access Grant | `@/connect` | `accessGrantServiceClientConnect` |
| Workload Identity | `@/connect` | `workloadIdentityServiceClientConnect` |
| Service Account | `@/connect` | `serviceAccountServiceClientConnect` |
| AI | `@/connect` | `aiServiceClientConnect` |
| CEL | `@/connect` | `celServiceClientConnect` |

## KHÔNG được dùng:

```typescript
// ❌ NEVER: raw fetch()
const res = await fetch("/v1/databases/my-db");

// ❌ NEVER: new Constructor()
const db = new Database({ name: "..." });

// ❌ NEVER: plain JSON object (must use create())
const req = { database: { name: "..." } }; // missing protobuf schema
```
```

### 2.2 Error Policy Document

Tạo `.ai-context/ERROR_POLICY.md`:

```markdown
# Error Handling Policy for AI-generated Code

## Interceptors Handle These — DO NOT catch in components:

| Error Type | Handler | Action |
|---|---|---|
| `401 Unauthenticated` | `authInterceptor` | Shows session expired UI |
| All gRPC error codes | `errorNotificationInterceptor` | Shows notification toast |
| Network timeout | `errorNotificationInterceptor` | Shows error notification |

## CORRECT pattern — NO try/catch for ConnectRPC calls:

```typescript
// ✅ CORRECT: Let interceptors handle errors
const { mutate } = useMutation({
  mutationFn: () => databaseServiceClientConnect.deleteDatabase({ name }),
  onSuccess: () => router.push("/databases"),
  // onError NOT needed — interceptor shows notification automatically
});
```

## ONLY catch when you need custom error behavior:

```typescript
// ✅ OK: Custom behavior needed (e.g., optimistic rollback)
try {
  await databaseServiceClientConnect.updateDatabase({ database, updateMask });
} catch (error) {
  if (error instanceof ConnectError && error.code === Code.AlreadyExists) {
    setNameError("Name already taken");
    return; // Stop here, interceptor will also show notification
  }
  throw error; // Re-throw unknown errors to interceptor
}
```

## Auth debugging flow:

```
401 response
  → authInterceptor attempts refreshTokens()
    → success: retry original request
    → fail: set unauthenticatedOccurred = true
      → App.vue shows SessionExpiredSurface
        → User re-authenticates
```

Token refresh uses Web Locks API — only ONE tab refreshes at a time.
DO NOT implement custom token refresh logic — it's centralized in `src/connect/refreshToken.ts`.
```

### 2.3 Lint Rules — Detect REST-style fetch in Service Calls

Thêm ESLint rule (`eslint-rules/no-fetch-for-grpc.mjs`):

```javascript
// Flag: fetch('/v1/...') calls that should be ConnectRPC client calls
export const noFetchForGrpc = {
  create(context) {
    return {
      CallExpression(node) {
        if (
          node.callee.name === "fetch" &&
          node.arguments[0]?.type === "Literal" &&
          node.arguments[0].value?.startsWith("/v1/")
        ) {
          context.report({
            node,
            message: "Use ConnectRPC service client instead of fetch() for /v1/ endpoints. See .ai-context/CONNECTRPC_GUIDE.md",
          });
        }
      },
    };
  },
};
```

Thêm lint rule cho missing `updateMask`:

```javascript
// Flag: {client}.update{Domain}({ {domain} }) without updateMask field
export const requireUpdateMask = {
  create(context) {
    return {
      CallExpression(node) {
        const isUpdateCall = /\.(update|patch)[A-Z]/.test(
          node.callee.property?.name ?? ""
        );
        if (isUpdateCall && node.arguments[0]?.type === "ObjectExpression") {
          const hasUpdateMask = node.arguments[0].properties.some(
            (p) => p.key?.name === "updateMask"
          );
          if (!hasUpdateMask) {
            context.report({
              node,
              message: "update* calls require updateMask field. List only changed fields.",
            });
          }
        }
      },
    };
  },
};
```

### 2.4 TanStack Query Integration (replaces direct client calls)

Sau khi migrate sang TanStack Query (SOL-AI-004), AI không còn cần gọi ConnectRPC clients trực tiếp — chỉ dùng hooks:

```typescript
// Before (AI cần biết ConnectRPC client):
const db = await databaseServiceClientConnect.getDatabase({ name });

// After (AI chỉ cần biết hook name):
const { data: db, isLoading, error } = useDatabase(name);
```

Đây là lý do SOL-AI-004 (TanStack Query) giải quyết 70% complexity của ISS-AI-007.

---

## 3. Implementation Checklist

- [ ] Tạo `.ai-context/CONNECTRPC_GUIDE.md` với full client lookup table
- [ ] Tạo `.ai-context/ERROR_POLICY.md`
- [ ] Thêm ESLint rule `no-fetch-for-grpc`
- [ ] Thêm ESLint rule `require-update-mask`
- [ ] Update AGENTS.md: link đến CONNECTRPC_GUIDE.md
- [ ] Sau SOL-AI-004: update guide để ưu tiên TanStack Query hooks

---

## 4. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| AI generates fetch() for gRPC | Common | Zero (lint-enforced) |
| AI forgets updateMask | Common | Zero (lint-enforced) |
| AI double-handles interceptor errors | Frequent | Rare (policy doc) |
| Wrong client selection | ~30% of cases | < 5% (cheat sheet) |

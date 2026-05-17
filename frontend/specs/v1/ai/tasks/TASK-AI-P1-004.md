# TASK-AI-P1-004: Thêm ESLint Rules — Proto Constructor + Fetch + UpdateMask

> **Source**: SOL-AI-002 §2.4 + SOL-AI-007 §2.3 | **Priority**: P1 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: —  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `eslint-rules/no-proto-constructor.mjs`
- **NEW** `eslint-rules/no-fetch-for-grpc.mjs`
- **NEW** `eslint-rules/require-update-mask.mjs`
- **EDIT** `eslint.config.mjs` — register 3 new rules

## What
3 lint rules tự động phát hiện top-3 AI mistakes trong API layer.

## Implementation

### `no-proto-constructor.mjs`
```javascript
// Flag: new Database(), new Project(), new Issue() — proto types dùng constructor
// Allow: create(DatabaseSchema, {...})
export const noProtoConstructor = {
  meta: { type: "problem", messages: { useCreate: "Use create(Schema, {...}) not new Constructor()" } },
  create(context) {
    const protoTypes = new Set(["Database", "Project", "Instance", "Issue", "Plan", "Rollout", "User"]);
    return {
      NewExpression(node) {
        if (protoTypes.has(node.callee.name)) {
          context.report({ node, messageId: "useCreate" });
        }
      },
    };
  },
};
```

### `no-fetch-for-grpc.mjs`
```javascript
// Flag: fetch('/v1/...') — should be ConnectRPC client call
export const noFetchForGrpc = {
  create(context) {
    return {
      CallExpression(node) {
        if (
          node.callee.name === "fetch" &&
          node.arguments[0]?.type === "Literal" &&
          typeof node.arguments[0].value === "string" &&
          node.arguments[0].value.startsWith("/v1/")
        ) {
          context.report({ node, message: "Use ConnectRPC service client instead of fetch() for /v1/ endpoints. See .ai-context/CONNECTRPC_GUIDE.md" });
        }
      },
    };
  },
};
```

### `require-update-mask.mjs`
```javascript
// Flag: client.updateXxx({...}) without updateMask property
export const requireUpdateMask = {
  create(context) {
    return {
      CallExpression(node) {
        const methodName = node.callee?.property?.name ?? "";
        if (/^update[A-Z]/.test(methodName) && node.arguments[0]?.type === "ObjectExpression") {
          const hasUpdateMask = node.arguments[0].properties.some(
            (p) => p.type === "Property" && (p.key?.name === "updateMask" || p.key?.value === "updateMask")
          );
          if (!hasUpdateMask) {
            context.report({ node, message: `${methodName}() requires updateMask field. List only the changed fields.` });
          }
        }
      },
    };
  },
};
```

### Update `eslint.config.mjs`
```javascript
import { noProtoConstructor } from "./eslint-rules/no-proto-constructor.mjs";
import { noFetchForGrpc } from "./eslint-rules/no-fetch-for-grpc.mjs";
import { requireUpdateMask } from "./eslint-rules/require-update-mask.mjs";

// In config array, add plugin + rules:
{
  plugins: { "bytebase-ai": { rules: { "no-proto-constructor": noProtoConstructor, "no-fetch-for-grpc": noFetchForGrpc, "require-update-mask": requireUpdateMask } } },
  rules: {
    "bytebase-ai/no-proto-constructor": "error",
    "bytebase-ai/no-fetch-for-grpc": "error",
    "bytebase-ai/require-update-mask": "warn",
  },
}
```

## AC
- [ ] 3 rule files tạo xong
- [ ] Rules registered trong `eslint.config.mjs`
- [ ] `pnpm lint` pass (no false positives on existing code)
- [ ] Test: thêm `new Database({})` → lint error
- [ ] Test: thêm `fetch('/v1/databases')` → lint error
- [ ] Test: thêm `.updateDatabase({ database })` không có updateMask → lint warn

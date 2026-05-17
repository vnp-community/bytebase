# SOL-AI-002 — Proto-ES AI Reference Layer

> **Resolves**: ISS-AI-002 (Proto-ES Generated Code Gây Quá Tải Context Window)  
> **Type**: Tooling + Documentation  
> **Priority**: High  
> **Effort**: Medium (~2 weeks)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Giảm cognitive load của AI từ 38K LOC proto-es xuống còn ~2K LOC AI-friendly reference layer, đảm bảo AI luôn dùng đúng type, đúng client, đúng pattern.

---

## 2. Giải Pháp

### 2.1 AI Reference Layer — Condensed Type Stubs

Tạo `src/types/ai-ref/` với các file condensed type stubs:

```
src/types/ai-ref/
├── index.ts           # Re-exports all AI reference types
├── database.ts        # Top-level Database fields only (no deep nesting)
├── project.ts
├── instance.ts
├── issue.ts
├── plan.ts
├── rollout.ts
├── sql.ts
├── setting.ts
├── user.ts
└── service-map.ts     # Domain → Client → Method → Types lookup
```

**Ví dụ `src/types/ai-ref/database.ts`:**

```typescript
/**
 * AI REFERENCE — Condensed Database type for AI code generation.
 * Full type: src/types/proto-es/v1/database_service_pb.d.ts
 *
 * IMPORTANT: Always use `create(DatabaseSchema, {...})` NOT `new Database()`
 */
export interface DatabaseRef {
  /** Resource name: "instances/{instance}/databases/{db}" */
  name: string;
  /** Display name */
  title: string;
  /** "environments/{env}" */
  environment: string;
  /** "instances/{instance}" */
  instance: string;
  /** "projects/{project}" */
  project: string;
  /** Key-value labels */
  labels: Record<string, string>;
  /** Schema last synced at */
  lastSuccessfulSyncTime?: string; // google.protobuf.Timestamp as ISO string
  /** Masked schema */
  schemaSize: bigint;
}

/**
 * Service client: databaseServiceClientConnect
 * Common methods:
 *   .getDatabase({ name })
 *   .listDatabases({ parent, filter, pageSize, pageToken })
 *   .updateDatabase({ database, updateMask: string[] }) ← updateMask REQUIRED
 *   .searchDatabases({ query, filter })
 *
 * Store: useDatabaseV1Store() in src/store/modules/v1/database.ts
 *   .getOrFetchDatabaseByName(name)
 *   .fetchDatabaseList({ parent })
 *   .updateDatabase(database, updateMask)
 */
export const DATABASE_CLIENT = "databaseServiceClientConnect";
export const DATABASE_UPDATE_MASK_FIELDS = [
  "labels", "environment", "project", "title"
] as const;
```

**Ví dụ `src/types/ai-ref/service-map.ts`:**

```typescript
/**
 * AI SERVICE MAP — Domain → Client → Store lookup table
 * Use this to find the correct client and store for any domain.
 */
export const SERVICE_MAP = {
  database: {
    client: "databaseServiceClientConnect",
    store: "useDatabaseV1Store",
    protoFile: "database_service_pb",
    commonMethods: ["getDatabase", "listDatabases", "updateDatabase", "searchDatabases"],
  },
  project: {
    client: "projectServiceClientConnect",
    store: "useProjectV1Store",
    protoFile: "project_service_pb",
    commonMethods: ["getProject", "listProjects", "createProject", "updateProject"],
  },
  instance: {
    client: "instanceServiceClientConnect",
    store: "useInstanceV1Store",
    protoFile: "instance_service_pb",
    commonMethods: ["getInstance", "listInstances", "createInstance", "updateInstance"],
  },
  issue: {
    client: "issueServiceClientConnect",
    store: "useIssueV1Store",
    protoFile: "issue_service_pb",
    commonMethods: ["getIssue", "listIssues", "createIssue", "updateIssue"],
  },
  user: {
    client: "userServiceClientConnect",
    store: "useUserStore",
    protoFile: "user_service_pb",
    commonMethods: ["getUser", "listUsers", "createUser", "updateUser"],
  },
  // ... remaining 25+ services
} as const;
```

### 2.2 Protobuf Construction Cheat Sheet

Tạo `.ai-context/PROTOBUF_PATTERNS.md`:

```markdown
## Protobuf Message Construction Rules

### ❌ NEVER do this:
```typescript
const db = new Database({ name: "..." });           // No constructor
const req = { database: db, updateMask: [] };       // Missing schema
```

### ✅ ALWAYS do this:
```typescript
import { create } from "@bufbuild/protobuf";
import { DatabaseSchema } from "@/types/proto-es/v1/database_service_pb";
const db = create(DatabaseSchema, { name: "...", title: "..." });
```

### updateMask — ALWAYS required for update calls:
```typescript
await databaseServiceClientConnect.updateDatabase({
  database: db,
  updateMask: ["labels", "title"], // List ONLY changed fields
});
```

### oneof fields pattern:
```typescript
// Protobuf oneof → TypeScript discriminated union:
const value: RowValue = {
  kind: { case: "stringValue", value: "hello" }  // NOT: { stringValue: "hello" }
};
```

### Enum values — use enum name, not number:
```typescript
import { Engine } from "@/types/proto-es/v1/common_pb";
instance.engine = Engine.MYSQL;  // NOT: instance.engine = 1
```
```

### 2.3 Script Auto-generation

Tạo `scripts/generate-ai-ref.ts` — script tự động extract top-level fields từ proto-es:

```typescript
// scripts/generate-ai-ref.ts
// Run: pnpm generate:ai-ref
// Output: src/types/ai-ref/ (regenerate on proto changes)
import { readdir, readFile, writeFile } from "fs/promises";
// Parse .d.ts files, extract top-level interface fields, generate condensed stubs
```

Thêm vào `package.json`:
```json
{
  "scripts": {
    "generate:ai-ref": "tsx scripts/generate-ai-ref.ts"
  }
}
```

### 2.4 Lint Rule — Enforce Proto Constructor Pattern

Thêm ESLint custom rule (`eslint-rules/no-proto-constructor.mjs`):

```javascript
// Flag: new DatabaseService() hoặc new MessageClass()
// Allow: create(MessageSchema, {...})
export const noProtoConstructor = {
  create(context) {
    return {
      NewExpression(node) {
        if (node.callee.name?.endsWith("Message") || 
            isProtoType(node.callee.name)) {
          context.report({ node, message: "Use create(Schema, {...}) not new Constructor()" });
        }
      }
    };
  }
};
```

---

## 3. Thay Đổi Technical Design Document

**Cập nhật `specs/technical-design-document.md` Section 3.2 "ConnectRPC Transport":**

Thêm subsection **3.2.4 AI Reference Layer**:
- Giải thích `src/types/ai-ref/` structure
- Link đến SERVICE_MAP cho AI code generation
- Declare condensed stubs as authoritative for AI, full proto-es as runtime source of truth

---

## 4. Implementation Checklist

- [ ] Tạo `src/types/ai-ref/` directory với 10 domain type files
- [ ] Tạo `src/types/ai-ref/service-map.ts` với đầy đủ 30 services
- [ ] Tạo `.ai-context/PROTOBUF_PATTERNS.md`
- [ ] Viết `scripts/generate-ai-ref.ts` auto-generation script
- [ ] Thêm `generate:ai-ref` vào package.json scripts
- [ ] Thêm ESLint rule `no-proto-constructor`
- [ ] Update AGENTS.md: "For API types, use `src/types/ai-ref/` NOT `src/types/proto-es/`"

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| LOC AI cần đọc cho types | ~38K | ~2K (ai-ref layer) |
| Proto constructor errors | Common | Zero (lint enforcement) |
| Missing updateMask errors | Common | Zero (lint + patterns) |
| Service client confusion | Frequent | Rare (service-map lookup) |

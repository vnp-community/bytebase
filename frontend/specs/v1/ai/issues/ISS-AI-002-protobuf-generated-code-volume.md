# ISS-AI-002 — Proto-ES Generated Code Gây Quá Tải Context Window Của AI

> **Category**: Context Window Limitation  
> **Severity**: High  
> **Impact**: Code Generation, Type Inference, API Integration  
> **Affected Area**: `src/types/proto-es/` — 86 files, ~37,851 LOC

---

## 1. Mô Tả Vấn Đề

Frontend sử dụng **ConnectRPC** với **Proto-ES** (generated từ Protobuf definitions). Hệ thống sinh ra 86 files với tổng cộng ~38K dòng code trong `src/types/proto-es/v1/`.

### 1.1 Khối Lượng Type Khổng Lồ

| File | Lines | Mô tả |
|---|---|---|
| `database_service_pb.d.ts` | 3,449 | Database schemas, metadata, change history |
| `rollout_service_pb.d.ts` | 1,962 | Rollout stages, tasks, runs |
| `instance_service_pb.d.ts` | 1,760 | Instance config, data sources |
| `setting_service_pb.d.ts` | 1,668 | Workspace settings (50+ fields) |
| `sql_service_pb.d.ts` | 1,480 | SQL execution, result schemas |
| `issue_service_pb.d.ts` | 1,364 | Issues, approval flows |
| `subscription_service_pb.d.ts` | 1,070 | License, plans, features |

### 1.2 Nested Message Complexity

Protobuf types có **deep nesting** — ví dụ:

```
Database
  ├── SchemaMetadata
  │   ├── SchemaConfig
  │   │   ├── TableConfig[]
  │   │   │   ├── ColumnConfig[]
  │   │   │   │   ├── MaskingLevel
  │   │   │   │   ├── SemanticTypeId
  │   │   │   │   └── ClassificationConfig
  │   │   │   └── IndexConfig[]
  │   │   └── ViewConfig[]
  │   └── SchemaMetadata[]
  ├── EffectiveEnvironment
  ├── Labels (Map)
  └── InstanceResource
```

### 1.3 Schema-specific Enum Proliferation

30+ services × 10+ enums mỗi service = **300+ enum types** mà AI cần nhận biết chính xác value mapping.

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **Type completion** | AI thường hallucinate field names vì không fit toàn bộ proto types vào context |
| **API call construction** | Request/Response types quá lớn, AI dễ bỏ sót `updateMask` fields hoặc enum values |
| **Protobuf message creation** | `create(SchemaName, { ... })` pattern yêu cầu biết chính xác Schema → AI dễ dùng sai constructor |
| **Service client selection** | 30+ singleton clients trong `src/connect/index.ts` → AI chọn sai client cho domain cụ thể |
| **Union types** (`oneof`) | Protobuf `oneof` fields map sang `{ case: "fieldName", value: ... }` pattern phi standard |

## 3. Ví Dụ Lỗi AI Thường Gặp

```typescript
// ❌ AI thường generate:
const db = new Database({ name: "..." });

// ✅ Correct pattern trong codebase:
import { create } from "@bufbuild/protobuf";
import { DatabaseSchema } from "@/types/proto-es/v1/database_service_pb";
const db = create(DatabaseSchema, { name: "..." });
```

```typescript
// ❌ AI thường quên updateMask:
await databaseServiceClientConnect.updateDatabase({ database });

// ✅ Correct:
await databaseServiceClientConnect.updateDatabase({
  database,
  updateMask: ["labels", "environment"],
});
```

## 4. Khuyến Nghị Giảm Thiểu

1. **Tạo AI-friendly type summaries**: Generate condensed `.d.ts` stubs cho top-20 most-used types (Database, Project, Instance, Issue, Plan, Rollout) chỉ với field names + types, không có JSDoc noise.
2. **Document service-to-store mapping**: Tạo lookup table: `{domain} → {storeFile} → {serviceClient} → {protoTypes}`.
3. **Enforce Protobuf constructor cheat sheet**: Cung cấp cho AI danh sách `Schema` names và `create()` pattern thay vì `new Constructor()`.
4. **Validate `updateMask` usage**: Lint rule hoặc AI prompt directive bắt buộc `updateMask` khi gọi `update*` methods.

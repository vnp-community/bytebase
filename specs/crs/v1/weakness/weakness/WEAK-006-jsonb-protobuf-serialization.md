# WEAK-006 — JSONB Protobuf Serialization Creates Query Opacity

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-006                                   |
| Category       | Data Access / Database Design              |
| Severity       | LOW                                        |
| Affected Layer | L8 (Store)                                 |
| Source Files   | `backend/store/*.go`                       |

---

## Mô tả

Nhiều columns lưu dữ liệu phức tạp dưới dạng Protocol Buffers JSON (camelCase) trong JSONB columns. Tạo query opacity và naming confusion.

## Chi tiết

### Affected Columns

| Table       | Column   | Content                              |
|-------------|----------|--------------------------------------|
| plan        | config   | Plan configuration (specs, steps)    |
| task        | payload  | Task-specific payload                |
| task_run    | result   | Task execution result                |
| policy      | payload  | Policy rules (masking, access, etc.) |
| setting     | value    | Setting value (various types)        |
| issue       | payload  | Issue metadata                       |

### camelCase vs snake_case Confusion

```go
// Go code uses protojson which outputs camelCase:
data, _ := protojson.Marshal(protoMessage)
// Stored as: {"taskRun": {...}, "planConfig": {...}}
// NOT:       {"task_run": {...}, "plan_config": {...}}
```

- PostgreSQL convention is `snake_case` for column names.
- JSONB content uses `camelCase` from protobuf.
- SQL queries filtering JSONB fields must use camelCase keys.

### Query Complexity

```sql
-- Example: finding plans with specific config
SELECT * FROM plan WHERE config->>'planConfig' = '...';
-- Developer might try: config->>'plan_config' (WRONG)
```

## Impact

- JSONB columns cannot be efficiently indexed for arbitrary queries.
- camelCase/snake_case mismatch creates developer confusion.
- Schema evolution inside JSONB is opaque — no column-level constraints.

## Khuyến nghị

1. Document JSONB field naming convention clearly.
2. Add GIN indexes on frequently queried JSONB paths.
3. Consider extracting frequently queried fields to dedicated columns.

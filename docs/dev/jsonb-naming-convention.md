# JSONB Naming Convention

## Column Names vs JSONB Keys

| Layer          | Convention   | Example                  |
|----------------|-------------|--------------------------|
| SQL columns    | `snake_case` | `issue_type`, `plan_id`  |
| JSONB keys     | `camelCase`  | `hasRollout`, `schemaVersion` |

### Why the difference?

- **SQL columns** follow PostgreSQL convention: all identifiers are `snake_case`.
- **JSONB keys** are serialized by Go's `protojson.Marshal`, which uses **camelCase** based on protobuf field names. Since JSONB is opaque to PostgreSQL, we preserve the proto serialization format.

### Generated Columns

When extracting a JSONB path into a generated column:

```sql
-- The JSONB key is camelCase (from protobuf),
-- but the generated column name is snake_case (SQL convention).
ALTER TABLE issue ADD COLUMN issue_type TEXT
    GENERATED ALWAYS AS (payload->>'type') STORED;
```

### Common Patterns

| Table           | JSONB Column | Key (camelCase)       | Generated Column (snake_case) |
|-----------------|-------------|----------------------|-------------------------------|
| `issue`         | `payload`   | `type`               | `issue_type`                  |
| `issue`         | `payload`   | `riskLevel`          | *(future)*                    |
| `plan`          | `config`    | `hasRollout`         | *(future)*                    |
| `task`          | `payload`   | `schemaVersion`      | *(future)*                    |
| `task`          | `payload`   | `skipped`            | *(future)*                    |

### Rules

1. **Never mix conventions**: JSONB keys are always `camelCase`, columns are always `snake_case`.
2. **Prefix generated columns**: Use `{table}_{field}` or a descriptive name to avoid ambiguity (e.g., `issue_type` not just `type`).
3. **Always add partial indexes**: Generated columns used for filtering should have `WHERE ... IS NOT NULL` partial indexes.
4. **Document new extractions**: Add new generated columns to the table above when created.

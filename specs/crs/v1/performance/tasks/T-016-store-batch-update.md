# T-016: Store — True Batch Update SQL

| Field | Value |
|-------|-------|
| **Task ID** | T-016 |
| **Solution** | SOL-PERF-005 |
| **Type** | New function in existing file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-017 |
| **Status** | DONE |

## Objective

Tạo `BatchUpdateDatabases` — single SQL UPDATE FROM VALUES thay vì N queries.

## Target File

`backend/store/database.go` — add new function

## Implementation

```go
func (s *Store) BatchUpdateDatabases(ctx context.Context,
    updates []*UpdateDatabaseMessage) ([]*DatabaseMessage, error) {
    if len(updates) == 0 {
        return nil, nil
    }

    q := qb.Q().Space(`
        UPDATE db SET
            project = v.project,
            environment = v.environment,
            metadata = v.metadata::jsonb
        FROM (VALUES `)

    for i, u := range updates {
        if i > 0 {
            q.Space(",")
        }
        metadataJSON, _ := protojson.Marshal(u.Metadata)
        q.Space("(?, ?, ?, ?)",
            u.InstanceID, u.DatabaseName, u.ProjectID, string(metadataJSON))
    }

    q.Space(`) AS v(instance_id, database_name, project, metadata)
        WHERE db.instance = v.instance_id AND db.name = v.database_name
        RETURNING db.instance, db.name, db.project, db.workspace,
                  db.environment, db.effective_environment, db.deleted`)

    query, args, err := q.ToSQL()
    if err != nil {
        return nil, err
    }

    rows, err := s.GetDB().QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []*DatabaseMessage
    for rows.Next() {
        var d DatabaseMessage
        if err := rows.Scan(
            &d.InstanceID, &d.DatabaseName, &d.ProjectID, &d.Workspace,
            &d.EnvironmentID, &d.EffectiveEnvironmentID, &d.Deleted,
        ); err != nil {
            return nil, err
        }
        results = append(results, &d)
        s.databaseCache.Remove(getDatabaseCacheKey(d.Workspace, d.InstanceID, d.DatabaseName))
    }
    return results, rows.Err()
}
```

## Verification

- 1K batch: ~10s (N queries) → ~300ms (single query)

# Solution: CR-PERF-005 — API Batch Operations & Query Optimization

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-005                              |
| **Solution ID**| SOL-PERF-005                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L4 (Service), L8 (Store)                 |
| **TDD Refs**   | §4 Data Access Layer, §3.2 Dual Transport |

---

## 1. Solution Overview

Từ Architecture L4, `DatabaseService` (57KB) là service lớn nhất. Hiện `BatchUpdateDatabases` loop gọi `UpdateDatabase` N lần. `ListDatabases` deserialize protobuf metadata trên mỗi row. Từ TDD §4.4, metadata lưu as JSONB protobuf — expensive to deserialize.

Giải pháp:
1. **True batch SQL** cho update/create operations
2. **View-based lazy loading** — skip metadata deserialization cho BASIC view
3. **Batch IAM check** — group by project
4. **Cursor-based pagination** (thay OFFSET)

---

## 2. Detailed Technical Design

### 2.1 True Batch Update — Single SQL Statement

**File**: `backend/store/database.go`

```go
// BatchUpdateDatabases updates multiple databases in a single SQL statement
// using PostgreSQL's UPDATE FROM VALUES pattern.
func (s *Store) BatchUpdateDatabases(ctx context.Context,
    updates []*UpdateDatabaseMessage) ([]*DatabaseMessage, error) {

    if len(updates) == 0 {
        return nil, nil
    }

    // Build VALUES clause for all updates
    q := qb.Q().Space(`
        UPDATE db SET
            project = v.project,
            environment = v.environment,
            metadata = v.metadata::jsonb,
            effective_environment = COALESCE(v.environment,
                (SELECT environment FROM instance WHERE resource_id = db.instance))
        FROM (VALUES `)

    for i, u := range updates {
        if i > 0 {
            q.Space(",")
        }
        metadataJSON, _ := protojson.Marshal(u.Metadata)
        q.Space("(?, ?, ?, ?)",
            u.InstanceID, u.DatabaseName, u.ProjectID,
            string(metadataJSON))
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
        // Invalidate cache
        s.databaseCache.Remove(getDatabaseCacheKey(d.Workspace, d.InstanceID, d.DatabaseName))
    }

    return results, rows.Err()
}
```

**File**: `backend/api/v1/database_service.go` — update BatchUpdateDatabases handler

```go
// BEFORE (line 421-431): Loop calling UpdateDatabase N times
func (s *DatabaseService) BatchUpdateDatabases(ctx context.Context,
    req *connect.Request[v1pb.BatchUpdateDatabasesRequest],
) (*connect.Response[v1pb.BatchUpdateDatabasesResponse], error) {
    // Convert proto requests to store update messages
    var updates []*store.UpdateDatabaseMessage
    for _, updateReq := range req.Msg.GetRequests() {
        u, err := s.convertUpdateRequest(updateReq)
        if err != nil { return nil, err }
        updates = append(updates, u)
    }

    // Batch permission check (see §2.3)
    if err := s.batchCheckPermission(ctx, updates); err != nil {
        return nil, err
    }

    // Single SQL execution
    results, err := s.store.BatchUpdateDatabases(ctx, updates)
    if err != nil { return nil, err }

    // Convert results
    var responses []*v1pb.Database
    for _, r := range results {
        responses = append(responses, s.convertToDatabase(ctx, r))
    }
    return connect.NewResponse(&v1pb.BatchUpdateDatabasesResponse{
        Databases: responses,
    }), nil
}
```

### 2.2 View-Based Lazy Loading (BASIC vs FULL)

Metadata JSONB deserialization (`protojson.Unmarshal`) costs ~0.1ms per row. For 1000 rows in a list, that's 100ms of CPU time. Skip it for BASIC view.

**File**: `backend/store/database.go` — modify ListDatabases

```go
type FindDatabaseMessage struct {
    // ... existing fields ...

    // View controls which fields are loaded.
    // BASIC: skip metadata, labels, config JSONB deserialization
    // FULL: load everything (default for backward compat)
    View DatabaseView
}

type DatabaseView int
const (
    DatabaseViewFull  DatabaseView = 0  // default: backward compatible
    DatabaseViewBasic DatabaseView = 1  // skip metadata deserialization
)

func (s *Store) ListDatabases(ctx context.Context, find *FindDatabaseMessage) ([]*DatabaseMessage, error) {
    // Select columns based on view
    selectCols := qb.Q().Space(`
        db.instance, db.name, db.project, db.workspace,
        db.environment, db.effective_environment, db.engine, db.deleted`)

    if find.View == DatabaseViewFull {
        selectCols.Space(", db.metadata, db.db_schema_metadata")
    }

    q := qb.Q().Space("SELECT ?", selectCols)
    // ... WHERE, ORDER BY, LIMIT ...

    rows, err := s.GetDB().QueryContext(ctx, query, args...)
    // ...
    for rows.Next() {
        var d DatabaseMessage
        if find.View == DatabaseViewBasic {
            err = rows.Scan(
                &d.InstanceID, &d.DatabaseName, &d.ProjectID, &d.Workspace,
                &d.EnvironmentID, &d.EffectiveEnvironmentID, &d.Engine, &d.Deleted)
        } else {
            var metadata, schemaMetadata []byte
            err = rows.Scan(
                &d.InstanceID, &d.DatabaseName, &d.ProjectID, &d.Workspace,
                &d.EnvironmentID, &d.EffectiveEnvironmentID, &d.Engine, &d.Deleted,
                &metadata, &schemaMetadata)
            if err == nil {
                d.Metadata = &storepb.DatabaseMetadata{}
                _ = common.ProtojsonUnmarshaler.Unmarshal(metadata, d.Metadata)
            }
        }
        // ...
    }
}
```

### 2.3 Batch IAM Permission Check

Từ TDD §7.2, permission check is per-project. Group databases by project → single check per project.

**File**: `backend/api/v1/database_service.go`

```go
// batchCheckPermission groups databases by project and performs
// one IAM check per project instead of per database.
func (s *DatabaseService) batchCheckPermission(
    ctx context.Context, databases []*store.UpdateDatabaseMessage) error {

    // Group by project
    projectDBs := make(map[string]bool)
    for _, db := range databases {
        projectDBs[db.ProjectID] = true
    }

    // One permission check per project
    user := auth.GetUserFromContext(ctx)
    workspace := auth.GetWorkspaceFromContext(ctx)
    for projectID := range projectDBs {
        ok, err := s.iamManager.CheckPermission(ctx,
            iam.PermissionDatabaseUpdate, user, workspace, projectID)
        if err != nil {
            return err
        }
        if !ok {
            return status.Errorf(codes.PermissionDenied,
                "no permission to update databases in project %q", projectID)
        }
    }
    return nil
}
```

### 2.4 Cursor-Based Pagination

Replace OFFSET-based pagination (skips rows) with cursor-based (seeks to position).

**File**: `backend/store/database.go`

```go
type FindDatabaseMessage struct {
    // ... existing fields ...

    // Cursor for keyset pagination (replaces OFFSET)
    // Format: "project:instance:name" — composite sort key
    AfterCursor *string
}

// In ListDatabases query builder:
if cursor := find.AfterCursor; cursor != nil {
    parts := strings.SplitN(*cursor, ":", 3)
    if len(parts) == 3 {
        where.And(`(db.project, db.instance, db.name) > (?, ?, ?)`,
            parts[0], parts[1], parts[2])
    }
}
// ORDER BY db.project, db.instance, db.name — matches cursor format
```

### 2.5 API Proto Changes (Additive)

**File**: `proto/v1/database_service.proto` — additive changes only

```protobuf
// Add to ListDatabasesRequest (additive, backward compatible)
message ListDatabasesRequest {
    // ... existing fields ...

    // View controls response detail level.
    // BASIC returns core fields only (name, project, environment).
    // FULL includes metadata, labels, config. Default: FULL.
    DatabaseView view = 7;

    // Cursor for keyset pagination. Set to the last database's
    // cursor value from the previous response to get the next page.
    string page_cursor = 8;
}

enum DatabaseView {
    DATABASE_VIEW_UNSPECIFIED = 0;
    DATABASE_VIEW_BASIC = 1;
    DATABASE_VIEW_FULL = 2;
}

message ListDatabasesResponse {
    // ... existing fields ...

    // Cursor for next page (empty if no more results).
    string next_page_cursor = 3;
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L4 (Service) | **HIGH** | BatchUpdate refactored, view parameter, cursor pagination |
| L8 (Store) | **HIGH** | True batch SQL, view-based column selection, cursor |
| L5 (IAM) | **MEDIUM** | Batch permission check utility |
| Proto | **LOW** | Additive fields (backward compatible) |
| L1 (Frontend) | **LOW** | Optional: use BASIC view for lists |

---

## 4. Performance Estimates

| Operation | Before | After | Improvement |
|-----------|--------|-------|-------------|
| BatchUpdate 1K DBs | ~10s (1K queries) | ~300ms (1 query) | **33x** |
| ListDatabases BASIC 1K | ~120ms (deserialize) | ~30ms (no deserialize) | **4x** |
| Permission check 100 DBs | 100 checks | ~5 checks (by project) | **20x** |
| Page 1000 (OFFSET 50000) | ~200ms (skip rows) | ~15ms (cursor seek) | **13x** |

---

## 5. Configuration

| Setting | Value | Description |
|---------|-------|-------------|
| Default View | `FULL` | Backward compatible — existing clients unaffected |
| Max batch size | 1000 | Single BatchUpdate call |
| Cursor format | `project:instance:name` | Matches ORDER BY clause |

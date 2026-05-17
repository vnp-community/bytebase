# T-013: Proto — DatabaseView + Cursor Fields

| Field | Value |
|-------|-------|
| **Task ID** | T-013 |
| **Solution** | SOL-PERF-005 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-014, T-015 |
| **Status** | DONE |

## Objective

Thêm `DatabaseView` enum, `view` field, và `page_cursor` vào proto API (additive, backward compatible).

## Target File

`proto/v1/database_service.proto`

## Changes (additive only)

```protobuf
// Add enum after existing message definitions:
enum DatabaseView {
    DATABASE_VIEW_UNSPECIFIED = 0;
    DATABASE_VIEW_BASIC = 1;
    DATABASE_VIEW_FULL = 2;
}

// Add to ListDatabasesRequest:
message ListDatabasesRequest {
    // ... existing fields ...
    DatabaseView view = 7;
    string page_cursor = 8;
}

// Add to ListDatabasesResponse:
message ListDatabasesResponse {
    // ... existing fields ...
    string next_page_cursor = 3;
}
```

## Post-step

Run `make proto` to regenerate Go code.

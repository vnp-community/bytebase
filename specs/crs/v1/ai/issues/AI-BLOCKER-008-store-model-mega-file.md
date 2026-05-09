# AI-BLOCKER-008: Store Model `database.go` is a 1290-Line Mega-Model

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-008 |
| **Severity** | 🟡 Medium |
| **Category** | Model Complexity / Context Window |
| **Layer** | L8 Store Model (`backend/store/model/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

`store/model/database.go` (1290 LOC, 40KB) contains the entire database metadata model hierarchy: `DatabaseMetadata`, `SchemaMetadata`, `TableMetadata`, `ColumnMetadata`, `IndexMetadata`, `ExternalTableMetadata` — plus all CRUD operations (Create/Drop/Rename for Schema, Table, View, MaterializedView, Index, Column).

AI agents modifying a single column operation must load 1290 lines of unrelated table/view/sequence logic.

## Impact

- **40KB single file**: exceeds most AI tool chunk sizes (typically 8-16KB)
- **Mixed concerns**: Schema CRUD, metadata indexing, case-sensitivity normalization, proto conversion all in one file
- **Cascading edits**: Adding a new database object type requires touching this file in 5+ places

## Recommended Remediation

1. Split into domain-specific files:
   - `model/database_metadata.go` — `DatabaseMetadata` struct + schema operations
   - `model/schema_metadata.go` — `SchemaMetadata` + table/view lookups
   - `model/table_metadata.go` — `TableMetadata` + column/index operations
   - `model/ddl_operations.go` — Create/Drop/Rename operations

## Files to Modify

```
backend/store/model/database.go → split into 4 files
```

# TASK-WEAK-007-1: Store Interface Extraction

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | — |
| Est. | M (~120 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Extract role-based interfaces from concrete `Store` struct to enable unit testing without database. Foundation for all service-level testing.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/interfaces.go` |

## Implementation Notes

### Delivered

- **20+ role-based interfaces** extracted into `backend/store/interfaces.go` (~360 LoC):
  - `UserReader`, `UserWriter`, `UserStore` (composed)
  - `ProjectReader`, `ProjectWriter`
  - `PlanReader`, `IssueReader`, `DatabaseReader`, `DatabaseWriter`
  - `InstanceReader`, `PolicyReader`, `SettingReader`, `WorkspaceReader`
  - `AuditLogWriter`, `DBSchemaReader`, `SheetReader`, `RoleReader`
  - `ChangelogReader`, `TaskStore`, `TaskRunStore`, `QueryHistoryStore`
  - `AccessGrantReader`, `ExportArchiveReader`, `AccountReader`
  - `SignalWriter`, `PlanWebhookWriter`, `SyncHistoryReader`, `AuthStore`
  - `DataStore` — superset interface for backward compatibility

- **Compile-time assertions** in `backend/store/interfaces_verify_test.go`:
  ```go
  var _ UserReader = (*Store)(nil)
  var _ UserWriter = (*Store)(nil)
  // ...20+ assertions
  ```

- **Design principle**: Small role interfaces > one massive StoreInterface. Services declare only what they need.

### Verification

```bash
go build ./backend/store/...   # ✅ passes
go test ./backend/store/...    # ✅ passes
```

## Acceptance Criteria

- [x] 6+ role interfaces defined (20+ delivered)
- [x] Compile-time `var _` assertions pass
- [x] Existing code unaffected (additive file)
- [x] `go build ./backend/store/...` passes

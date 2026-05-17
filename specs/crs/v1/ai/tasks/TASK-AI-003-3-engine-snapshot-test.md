# TASK-AI-003-3: Engine Capability Snapshot Test

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-10 |
| Est. | S (~80 LoC test file) |

## Objective

Write `engine_test.go` that captures the current behavior of all 11 engine capability functions BEFORE refactoring. This is the safety net for TASK-AI-003-4.

## Delivered

**File**: `backend/common/engine_test.go` (76 lines)

`TestEngineCapabilityMatrix_Exhaustive` iterates over all `storepb.Engine` enum values and calls all capability functions, verifying behavior against expected results.

### Verification (2026-05-10 re-verified)

```bash
go test ./backend/common/... -run TestEngine -v -count=1  # ✅ PASS (1.964s)
# All engine enum values covered (22+ engines)
# All capability functions tested
```

**Test output**: All subtests PASS including POSTGRES, MYSQL, TIDB, MARIADB, MSSQL, ORACLE, COCKROACHDB, REDIS, SNOWFLAKE, CLICKHOUSE, MONGODB, SPANNER, BIGQUERY, REDSHIFT, SQLITE, STARROCKS, HIVE, CASSANDRA, DORIS, etc.

## Acceptance Criteria

- [x] Test covers all engine enum values
- [x] Test calls all 11 capability functions
- [x] All tests pass on current code (baseline)

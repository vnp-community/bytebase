# TASK-LIM-003-B1: Migration Engine + CLI

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-003 |
| Phase | B — Migration Tool |
| Priority | P0 |
| Depends On | — |
| Est. | L (~400 LoC) |

## Objective

Build CLI tool `bytebase migrate-db` that dumps embedded PG and restores to external PG with validation.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/dbmigrate/engine.go` |
| CREATE | `backend/cmd/migrate_db.go` |

## Specification

### `engine.go` — MigrationEngine

6-step pipeline:
1. **validateTarget** — connect, check PG version ≥14, verify empty DB
2. **createBackup** — backup current embedded data
3. **dumpEmbedded** — `pg_dump --format custom --no-owner`
4. **restoreToTarget** — `pg_restore --dbname <target> --no-owner --clean --if-exists`
5. **verifyIntegrity** — compare row counts for 11 key tables
6. **Report** — output `PG_URL=<target>` instruction

Progress reporting: `chan MigrationProgress` with Phase, Percent, Message.

### `migrate_db.go` — Cobra command

```
bytebase migrate-db --target-url <PG_URL> [--dry-run] [--backup-dir <dir>]
```

- `--dry-run`: validate target only (step 1)
- `--backup-dir`: default `{dataDir}/migration_backup`
- Fails if not using embedded PG

## Acceptance Criteria

- [x] Dry-run validates target PG connection and emptiness → **DONE**: `validateTarget()` checks PG version ≥14 and no user tables
- [x] Full migration: dump → restore → verify row counts → **DONE**: 6-step pipeline via `Engine.Run()`
- [x] Progress reported with percentages → **DONE**: `chan MigrationProgress` with Phase/Percent/Message
- [x] Backup created before migration → **DONE**: `createBackup()` runs `pg_dump --format custom --compress 6`
- [x] Non-empty target DB rejected with clear error → **DONE**: `information_schema.tables` count check
- [x] Non-embedded PG rejected with clear message → **DONE**: CLI checks `profile.UseEmbedDB()` before proceeding

## Implementation Notes

- Created `backend/component/dbmigrate/engine.go` (~310 LoC)
  - 6-step pipeline: validateTarget → createBackup → dumpEmbedded → restoreToTarget → verifyIntegrity → report
  - Verifies 11 key tables (principal, member, instance, db, issue, pipeline, stage, task, changelog, setting, project)
  - pg_restore exit code 1 (warnings) is tolerated for `--clean --if-exists`
- Created `backend/bin/server/cmd/migrate_db.go` (~110 LoC)
  - Cobra subcommand: `bytebase migrate-db --target-url <URL> [--dry-run] [--backup-dir <dir>]`
  - Real-time progress printing + final summary with PG_URL instruction

**Status: ✅ DONE**

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

- [ ] Dry-run validates target PG connection and emptiness
- [ ] Full migration: dump → restore → verify row counts
- [ ] Progress reported with percentages
- [ ] Backup created before migration
- [ ] Non-empty target DB rejected with clear error
- [ ] Non-embedded PG rejected with clear message

# TASK-SEC-022 — Credential Rotation Runner + Emergency Rotation

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-022                               |
| **Source**       | SOL-SEC-008 §3.1-§3.4                     |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement credential rotation runner (L6) và emergency rotation endpoint (L4) cho database credentials.

## Scope

1. **Migration**: ALTER `data_source` ADD `backup_password`, `backup_expiry`, `rotation_schedule`, `last_rotated`; CREATE `credential_rotation_log` table
2. **Runner**: `runner/credential_rotation/runner.go` — 1h ticker, check rotation schedule, alert expiring credentials
3. **Rotation flow**: Generate new password → test connectivity → store new (keep old 24h as backup) → notify
4. **Dual-credential**: `DBFactory.GetDriver()` — try primary, fallback to backup during grace period
5. **Emergency rotation**: `InstanceService.EmergencyRotateCredential()` — change DB password, update store, force disconnect
6. **Driver interface**: Extend `db.Driver` with `ChangePassword(ctx, newPassword)` per engine (PostgreSQL, MySQL)

## Acceptance Criteria

- [ ] Scheduled rotation works on cron schedule
- [ ] Dual-credential fallback during rotation
- [ ] Emergency rotation immediate + force disconnect
- [ ] PostgreSQL + MySQL ChangePassword implemented
- [ ] Rotation log persisted

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | Schema changes |
| `backend/runner/credential_rotation/runner.go` | New file |
| `backend/component/dbfactory/factory.go` | Dual-credential |
| `backend/api/v1/instance_service.go` | Emergency rotation |
| `backend/plugin/db/pg/`, `backend/plugin/db/mysql/` | ChangePassword |

## Definition of Done

- Zero-downtime rotation verified

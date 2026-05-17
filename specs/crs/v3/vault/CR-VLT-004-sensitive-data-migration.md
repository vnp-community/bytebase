# Change Request: Sensitive Data Migration Engine

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-004                                               |
| **Title**          | Sensitive Data Migration Engine                          |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-VLT-001, CR-VLT-003                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Xây dựng **Sensitive Data Migration Engine** để chuyển dữ liệu nhạy cảm từ storage hiện tại (plaintext/obfuscation trong PostgreSQL) sang external vault. Engine hỗ trợ:
- Migration từ local → vault (onboarding)
- Migration từ vault A → vault B (provider switch)
- Rollback từ vault → local (emergency)
- Dry-run mode cho verification trước khi commit

### 1.2 Bối cảnh
Khi doanh nghiệp quyết định chuyển sang vault-backed storage, cần migration tool để:
1. Scan tất cả sensitive fields hiện tại
2. Decrypt/deobfuscate từ local storage
3. Write vào target vault
4. Verify integrity
5. Update references trong PostgreSQL
6. Clean up plaintext/obfuscated values

**Rủi ro**: Migration sai có thể **lock out** toàn bộ instance connections. Cần rollback plan rõ ràng.

### 1.3 Mục tiêu
- Zero-downtime migration (no service interruption)
- Atomic per-secret migration (rollback granular)
- Verification step trước khi xóa local copy
- Progress tracking + resume from interruption
- Audit trail cho mọi migration action

---

## 2. Yêu cầu chức năng

### FR-001: Migration Plan Generator

Tự động tạo migration plan từ Sensitive Data Catalog:

```go
type MigrationPlan struct {
    ID          string
    Status      MigrationStatus  // DRAFT, RUNNING, PAUSED, COMPLETED, FAILED, ROLLED_BACK
    Direction   MigrationDirection // LOCAL_TO_VAULT, VAULT_TO_VAULT, VAULT_TO_LOCAL
    SourceProvider string         // "local", "vault-kv-v2", etc.
    TargetProvider string
    Categories  []SecretCategory  // Which categories to migrate
    DryRun      bool
    Items       []MigrationItem
    CreatedAt   time.Time
    CreatedBy   int              // principal ID
    StartedAt   *time.Time
    CompletedAt *time.Time
}

type MigrationItem struct {
    Ref         SecretRef
    Status      ItemStatus  // PENDING, MIGRATING, VERIFIED, COMPLETED, FAILED, SKIPPED
    SourceValue string      // Only in-memory during migration, never persisted
    Error       string
    MigratedAt  *time.Time
}

type MigrationStatus int
const (
    MigrationStatusDraft MigrationStatus = iota
    MigrationStatusRunning
    MigrationStatusPaused
    MigrationStatusCompleted
    MigrationStatusFailed
    MigrationStatusRolledBack
)
```

### FR-002: Migration Executor

Background runner thực hiện migration:

```
Migration Execution Flow:
  │
  ├─ 1. Validate: Check source readable + target writable
  │     ├─ Source provider health check
  │     ├─ Target provider health check
  │     └─ Test write/read/delete cycle on target
  │
  ├─ 2. Scan: Enumerate all secrets to migrate
  │     ├─ Query DataSource table for all instances
  │     ├─ Query Setting table for all sensitive settings
  │     ├─ Query server_config for auth_secret
  │     └─ Generate MigrationItem list
  │
  ├─ 3. FOR each MigrationItem (with concurrency limit):
  │     ├─ Read from source (decrypt/deobfuscate)
  │     ├─ Write to target vault
  │     ├─ Verify: read back from target, compare
  │     ├─ [DryRun=false] Update DB reference (VaultRef)
  │     ├─ [DryRun=false] Clear plaintext/obfuscated value
  │     └─ Update item status
  │
  ├─ 4. Summary: Report results
  │     ├─ Total items
  │     ├─ Migrated count
  │     ├─ Failed count (with errors)
  │     └─ Skipped count (already vault-backed)
  │
  └─ 5. [Optional] Cleanup: Remove old obfuscated values
        └─ Only after all items verified
```

### FR-003: Atomic Per-Secret Operation

Mỗi secret được migrate **independently**:
- Nếu 1 secret fails, các secret khác vẫn tiếp tục
- Failed items có thể retry individually
- Migration có thể pause/resume

```go
func (e *MigrationExecutor) migrateItem(ctx context.Context, item *MigrationItem) error {
    // 1. Read from source
    value, err := e.sourceProvider.GetSecret(ctx, item.Ref.Path, item.Ref.Key)
    if err != nil {
        return fmt.Errorf("read source: %w", err)
    }
    
    // 2. Write to target
    if err := e.targetProvider.SetSecret(ctx, item.Ref.Path, item.Ref.Key, value); err != nil {
        return fmt.Errorf("write target: %w", err)
    }
    
    // 3. Verify
    readBack, err := e.targetProvider.GetSecret(ctx, item.Ref.Path, item.Ref.Key)
    if err != nil || readBack != value {
        // Rollback: delete from target
        _ = e.targetProvider.DeleteSecret(ctx, item.Ref.Path, item.Ref.Key)
        return fmt.Errorf("verification failed")
    }
    
    // 4. Update DB reference (skip in dry-run)
    if !e.plan.DryRun {
        if err := e.updateDBReference(ctx, item.Ref); err != nil {
            return fmt.Errorf("update reference: %w", err)
        }
    }
    
    return nil
}
```

### FR-004: Rollback Support

Emergency rollback khi migration gặp vấn đề:

```go
func (e *MigrationExecutor) Rollback(ctx context.Context, plan *MigrationPlan) error {
    for _, item := range plan.Items {
        if item.Status == ItemStatusCompleted {
            // 1. Read from target vault
            value, err := e.targetProvider.GetSecret(ctx, item.Ref.Path, item.Ref.Key)
            if err != nil {
                continue // Log and skip
            }
            
            // 2. Re-obfuscate and write back to local
            if err := e.reObfuscate(ctx, item.Ref, value); err != nil {
                continue // Log and skip
            }
            
            // 3. Remove VaultRef from DB
            if err := e.clearVaultRef(ctx, item.Ref); err != nil {
                continue
            }
            
            item.Status = ItemStatusRolledBack
        }
    }
    plan.Status = MigrationStatusRolledBack
    return nil
}
```

### FR-005: Migration API & UI

gRPC API cho migration management:

```protobuf
service VaultMigrationService {
    rpc CreateMigrationPlan(CreateMigrationPlanRequest) returns (MigrationPlan);
    rpc GetMigrationPlan(GetMigrationPlanRequest) returns (MigrationPlan);
    rpc ListMigrationPlans(ListMigrationPlansRequest) returns (ListMigrationPlansResponse);
    rpc ExecuteMigrationPlan(ExecuteMigrationPlanRequest) returns (MigrationPlan);
    rpc PauseMigrationPlan(PauseMigrationPlanRequest) returns (MigrationPlan);
    rpc ResumeMigrationPlan(ResumeMigrationPlanRequest) returns (MigrationPlan);
    rpc RollbackMigrationPlan(RollbackMigrationPlanRequest) returns (MigrationPlan);
    rpc RetryMigrationItem(RetryMigrationItemRequest) returns (MigrationItem);
}
```

UI Dashboard:
- Progress bar with item-level detail
- Real-time status updates (via WebSocket)
- Per-category breakdown
- Error log viewer
- One-click rollback button

### FR-006: Cross-Vault Migration

Migrate secrets từ vault provider A sang vault provider B:

```
Ví dụ: HashiCorp Vault → Vaultwarden

1. Configure new target (Vaultwarden) alongside existing (Vault)
2. Create migration plan: direction=VAULT_TO_VAULT
3. Execute: read from Vault → write to Vaultwarden → verify
4. Switch workspace config to Vaultwarden
5. Old Vault kept as read-only fallback for configurable period
6. After confirmation, cleanup old Vault data
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| Migration Plan types               | `backend/component/secret/migration/types.go`        | New: plan, item, status types                |
| Migration Executor                 | `backend/component/secret/migration/executor.go`     | New: migration execution engine              |
| Migration Scanner                  | `backend/component/secret/migration/scanner.go`      | New: scan DB for sensitive data              |
| Migration Rollback                 | `backend/component/secret/migration/rollback.go`     | New: rollback mechanism                      |
| Migration Store                    | `backend/store/vault_migration.go`                   | New: migration plan persistence              |
| Migration API                      | `backend/api/v1/vault_migration_service.go`          | New: gRPC service for migration management   |
| Migration Runner                   | `backend/runner/vaultmigration/runner.go`            | New: background migration runner             |
| Proto: Migration messages          | `proto/v1/v1/vault_migration_service.proto`          | New: migration API proto definitions         |
| Database Schema                    | `backend/migrator/migration/*/`                      | Tables: `vault_migration_plan`, `vault_migration_item` |
| UI: Migration Dashboard            | `frontend/src/views/Setting/VaultMigration.vue`      | New: migration management UI                 |

### 3.1 Database Schema

```sql
CREATE TABLE vault_migration_plan (
    id SERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    direction VARCHAR(50) NOT NULL,  -- LOCAL_TO_VAULT, VAULT_TO_VAULT, VAULT_TO_LOCAL
    source_provider VARCHAR(100) NOT NULL,
    target_provider VARCHAR(100) NOT NULL,
    categories JSONB NOT NULL DEFAULT '[]',
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    total_items INT DEFAULT 0,
    completed_items INT DEFAULT 0,
    failed_items INT DEFAULT 0,
    skipped_items INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by INT REFERENCES principal(id),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE TABLE vault_migration_item (
    id SERIAL PRIMARY KEY,
    plan_id INT REFERENCES vault_migration_plan(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL,
    scope TEXT NOT NULL,
    path TEXT NOT NULL,
    key_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    error TEXT,
    migrated_at TIMESTAMPTZ,
    UNIQUE(plan_id, path, key_name)
);

CREATE INDEX idx_vault_migration_item_plan_status ON vault_migration_item(plan_id, status);
```

### 3.2 Concurrency & Performance

- Migration executor uses worker pool (configurable, default: 4 workers)
- Each worker handles one MigrationItem at a time
- Rate limiting to avoid overwhelming vault (configurable, default: 50 req/s)
- Progress checkpointing every 100 items
- Resume from last checkpoint on restart

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Secrets in-memory during migration | Each secret held only during its migration; zeroed after use |
| Migration plan access control    | Requires WORKSPACE_ADMIN + Enterprise license                 |
| Audit trail completeness         | Every item migration logged to audit_log table               |
| Rollback data integrity          | Verify round-trip (source → target → readback) before commit |
| Concurrent migration plans       | Only one active plan per workspace (enforced by DB constraint)|
| DBA can see migration progress   | Item details show path/key but never actual secret values    |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Dry-run migration: local → vault                     | All items scanned, no actual changes       |
| TC-002  | Full migration: 10 DataSource passwords → vault      | All 10 passwords in vault, DB refs updated |
| TC-003  | Migration with 1 vault write failure                  | 9 succeed, 1 failed, retry available      |
| TC-004  | Pause and resume migration at item 50/100            | Resumes from item 51                       |
| TC-005  | Full rollback after migration                        | All secrets back in local, vault refs cleared |
| TC-006  | Cross-vault: Vault KV V2 → Vaultwarden              | All secrets migrated, provider switched   |
| TC-007  | Migration with concurrent DB access                  | No locking issues, connections still work |
| TC-008  | Migration of auth_secret                             | JWT signing continues working throughout  |
| TC-009  | Cancel running migration                             | Graceful stop, completed items kept       |
| TC-010  | Re-run migration after partial failure               | Only failed/pending items re-attempted    |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Migration plan types + Scanner             | Sprint 1       |
| Phase 2 | Executor + per-item migration logic        | Sprint 2       |
| Phase 3 | Rollback + pause/resume                    | Sprint 3       |
| Phase 4 | API + Runner + UI dashboard                | Sprint 4       |
| Phase 5 | Cross-vault migration + E2E testing        | Sprint 5       |

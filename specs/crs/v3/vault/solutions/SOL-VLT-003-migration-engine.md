# SOL-VLT-003: Sensitive Data Migration Engine

| Field | Value |
|---|---|
| **SOL ID** | SOL-VLT-003 |
| **CR Reference** | CR-VLT-004 |
| **Layer** | L5 (Component) + L6 (Runner) + L4 (API) + L8 (Store) |
| **Dependencies** | SOL-VLT-001, SOL-VLT-002 |
| **Estimated Effort** | 5 sprints |

---

## 1. Bối cảnh

Khi doanh nghiệp chuyển từ local storage sang vault, cần migration engine để:
- Đọc secrets từ source (obfuscated DB / existing vault)
- Ghi vào target vault
- Verify integrity
- Cập nhật DB references
- Rollback nếu lỗi

**Rủi ro chính**: Migration sai → lock out toàn bộ DB connections.

---

## 2. Giải pháp

### 2.1 Component Architecture

```
component/secret/migration/
├── types.go       # MigrationPlan, MigrationItem, status types
├── planner.go     # Plan generator from Catalog + DB scan
├── executor.go    # Worker pool executor
├── verifier.go    # Round-trip verification
├── rollback.go    # Rollback mechanism
└── store.go       # Interface for plan persistence

runner/vaultmigration/
└── runner.go      # Background runner (L6, same pattern as runner/cleaner/)

api/v1/
└── vault_migration_service.go  # gRPC API

store/
└── vault_migration.go          # DB persistence for plans/items
```

### 2.2 Migration Plan Types

```go
// migration/types.go
type MigrationPlan struct {
    ID              int
    WorkspaceID     string
    Status          PlanStatus  // DRAFT → RUNNING → COMPLETED/FAILED/ROLLED_BACK
    Direction       Direction   // LOCAL_TO_VAULT, VAULT_TO_VAULT, VAULT_TO_LOCAL
    SourceProvider  string
    TargetProvider  string
    Categories      []SecretCategory
    DryRun          bool
    TotalItems      int
    CompletedItems  int
    FailedItems     int
    SkippedItems    int
    CreatedBy       int  // principal ID
    CreatedAt       time.Time
    StartedAt       *time.Time
    CompletedAt     *time.Time
}

type MigrationItem struct {
    ID        int
    PlanID    int
    Ref       SecretRef
    Status    ItemStatus  // PENDING → MIGRATING → VERIFIED → COMPLETED / FAILED / SKIPPED
    Error     string
    MigratedAt *time.Time
}

type PlanStatus int
const (
    PlanDraft PlanStatus = iota
    PlanRunning
    PlanPaused
    PlanCompleted
    PlanFailed
    PlanRolledBack
)
```

### 2.3 Execution Flow

```
CreateMigrationPlan(categories, direction, dryRun)
  │
  ├─ 1. Validate: health check source + target providers
  │     ├─ source.Healthy(ctx)
  │     ├─ target.Healthy(ctx)
  │     └─ target test cycle: SetSecret → GetSecret → DeleteSecret
  │
  ├─ 2. Scan: enumerate all secrets matching categories
  │     ├─ Query instance/datasource for DataSource category
  │     ├─ Query setting for Setting/IDP/Webhook categories
  │     ├─ Query server_config for Auth category
  │     └─ Generate MigrationItem list → persist to DB
  │
  ├─ 3. Execute: worker pool (default 4 workers, rate-limited 50/s)
  │     FOR each item (concurrency-limited):
  │     ├─ Read from source (decrypt/deobfuscate)
  │     ├─ Write to target vault
  │     ├─ Verify: read back from target, byte-compare
  │     ├─ [DryRun=false] Update DB reference to VaultRef
  │     ├─ [DryRun=false] Clear plaintext/obfuscated value
  │     └─ Update item status + checkpoint
  │
  ├─ 4. Report: summary of results
  │
  └─ 5. [Optional] Cleanup old obfuscated values (batch, after all verified)
```

### 2.4 Per-Item Migration (Atomic)

```go
// executor.go
func (e *Executor) migrateItem(ctx context.Context, item *MigrationItem) error {
    // 1. Read from source
    value, err := e.source.GetSecret(ctx, item.Ref.VaultPath(e.namespace), item.Ref.Key)
    if err != nil {
        return fmt.Errorf("read source: %w", err)
    }

    // 2. Write to target
    targetPath := item.Ref.VaultPath(e.namespace)
    if err := e.target.SetSecret(ctx, targetPath, item.Ref.Key, value); err != nil {
        return fmt.Errorf("write target: %w", err)
    }

    // 3. Verify round-trip
    readBack, err := e.target.GetSecret(ctx, targetPath, item.Ref.Key)
    if err != nil || readBack != value {
        _ = e.target.DeleteSecret(ctx, targetPath, item.Ref.Key) // Rollback write
        return fmt.Errorf("verification failed: readback mismatch")
    }

    // 4. Update DB reference (skip in dry-run)
    if !e.plan.DryRun {
        if err := e.updateDBReference(ctx, item.Ref); err != nil {
            return fmt.Errorf("update DB ref: %w", err)
        }
    }

    // Zero secret from memory
    value = ""
    return nil
}
```

### 2.5 Rollback Mechanism

```go
// rollback.go
func (e *Executor) Rollback(ctx context.Context, plan *MigrationPlan) error {
    for _, item := range completedItems(plan) {
        // 1. Read from target vault
        value, err := e.target.GetSecret(ctx, path, key)
        if err != nil {
            log.Warn("rollback: cannot read from target", "item", item, "err", err)
            continue
        }
        // 2. Re-obfuscate and store back to local DB
        if err := e.reObfuscate(ctx, item.Ref, value); err != nil {
            continue
        }
        // 3. Clear VaultRef from DB
        if err := e.clearVaultRef(ctx, item.Ref); err != nil {
            continue
        }
        item.Status = ItemRolledBack
    }
    plan.Status = PlanRolledBack
    return e.store.UpdatePlan(ctx, plan)
}
```

### 2.6 Pause/Resume

- Each item is independently tracked in `vault_migration_item` table
- Resume = query items with `status = PENDING` and continue execution
- Progress checkpoint every 100 items via DB update

### 2.7 Cross-Vault Migration

```
Direction: VAULT_TO_VAULT (e.g., HashiCorp Vault → Vaultwarden)

1. Admin configures new target provider alongside existing
2. Creates plan: direction=VAULT_TO_VAULT, source=vault-kv-v2, target=vaultwarden
3. Executor reads from old vault → writes to new vault → verifies
4. After completion: workspace VaultConfig switches to new provider
5. Old vault kept as read-only fallback for configurable period (default: 7d)
6. After confirmation: admin deletes old vault data
```

---

## 3. Database Schema

```sql
CREATE TABLE vault_migration_plan (
    id SERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    direction VARCHAR(50) NOT NULL,
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
    completed_at TIMESTAMPTZ,
    -- Only one active plan per workspace
    CONSTRAINT uq_active_plan EXCLUDE (workspace WITH =)
        WHERE (status IN ('RUNNING', 'PAUSED'))
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

CREATE INDEX idx_migration_item_status ON vault_migration_item(plan_id, status);
```

Migration file: `backend/migrator/migration/prod/YYYYMMDDHHMMSS##vault_migration.sql`

---

## 4. API Design

```protobuf
// proto/v1/v1/vault_migration_service.proto
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

**ACL**: Requires `WORKSPACE_ADMIN` role + Enterprise license check.

---

## 5. Runner Integration

```go
// runner/vaultmigration/runner.go
// Same pattern as runner/cleaner/cleaner.go

type Runner struct {
    store         *store.Store
    secretManager *secret.SecretManager
    interval      time.Duration // 10s — poll for running plans
}

func (r *Runner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            r.processActivePlans(ctx)
        }
    }
}
```

Registered in server bootstrap at step 9 alongside other runners.

---

## 6. Security Constraints

| Constraint | Implementation |
|---|---|
| Secret values never persisted in migration tables | Only path/key/status stored; values held in-memory during migration only |
| Concurrent access during migration | Existing connections use cached credentials; new connections resolved normally |
| Only one active plan per workspace | DB exclusion constraint on (workspace, active status) |
| Audit trail for all migrations | Each item migration logged to `audit_log` table |
| DBA can monitor but not see values | API returns plan progress, item paths — never actual secret values |

# Solution: CR-SEC-008 — Database Credential Rotation Automation

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-008                |
| **Solution**   | SOL-SEC-008               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Triển khai credential rotation runner (L6) tích hợp với DBFactory (L5) và DB Driver plugins (L7). Scheduled rotation sử dụng dual-credential approach — hai credentials active trong grace period. Emergency rotation endpoint (L4) trong InstanceService. Vault dynamic secrets via existing Secret component (L5).

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4** | `instance_service.go` (64KB) | Emergency rotation API endpoint |
| **L5** | `component/dbfactory/` | Dual-credential connection handling |
| **L5** | `component/secret/` | Vault dynamic secrets integration |
| **L6** | `runner/credential_rotation/` (new) | Scheduled rotation engine |
| **L7** | `plugin/db/*/` | Per-engine credential change support |
| **L8** | `store/instance.go` | Credential storage (encrypted via SOL-SEC-007) |

---

## 3. Chi tiết Implementation

### 3.1 L6 — Credential Rotation Runner

```go
type CredentialRotationRunner struct {
    store          *store.Store
    dbFactory      *dbfactory.Factory
    encryptor      *encryption.EnvelopeEncryptor
    webhookManager *webhook.Manager
    interval       time.Duration // Check every 1h
}

func (r *CredentialRotationRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    for {
        select {
        case <-ticker.C:
            r.checkRotationSchedule(ctx)
            r.alertExpiringCredentials(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (r *CredentialRotationRunner) rotateCredential(ctx context.Context, instance *store.InstanceMessage) error {
    // 1. Generate new credential
    newPassword := generateSecurePassword(32)

    // 2. Test new credential connectivity BEFORE swap
    testDriver, err := r.dbFactory.OpenWithCredential(ctx, instance, newPassword)
    if err != nil {
        return fmt.Errorf("pre-rotation test failed: %w", err)
    }
    testDriver.Close(ctx)

    // 3. Store new credential (keep old as backup during grace period)
    r.store.UpdateDataSourceCredential(ctx, instance.DataSourceUID, newPassword, 24*time.Hour)

    // 4. Notify admins
    r.webhookManager.NotifyCredentialRotation(ctx, instance)

    return nil
}
```

### 3.2 L5 — DBFactory Dual-Credential

**File**: `backend/component/dbfactory/factory.go`

```go
func (f *Factory) GetDriver(ctx context.Context, instance *store.InstanceMessage) (db.Driver, error) {
    ds := instance.PrimaryDataSource()

    // Try primary credential
    driver, err := db.Open(ctx, instance.Engine, f.buildConfig(ds))
    if err != nil {
        // Try backup credential during grace period
        if ds.BackupPassword != "" && ds.BackupExpiry.After(time.Now()) {
            backupConfig := f.buildConfig(ds)
            backupConfig.Password = ds.BackupPassword
            driver, err = db.Open(ctx, instance.Engine, backupConfig)
        }
    }
    return driver, err
}
```

### 3.3 L4 — Emergency Rotation Endpoint

```go
func (s *InstanceService) EmergencyRotateCredential(ctx context.Context, req *v1pb.EmergencyRotateRequest) error {
    instance, _ := s.store.GetInstance(ctx, req.InstanceName)

    // Execute immediate rotation
    newPassword := generateSecurePassword(32)

    // Change password on target database
    driver, _ := s.dbFactory.GetDriver(ctx, instance)
    if err := driver.ChangePassword(ctx, newPassword); err != nil {
        return err
    }

    // Update stored credential
    s.store.UpdateDataSourcePassword(ctx, instance.DataSourceUID, newPassword)

    // Force disconnect existing connections
    s.dbFactory.CloseAllConnections(ctx, instance.UID)

    // Audit + notify
    s.auditCredentialRotation(ctx, instance, "emergency")
    return nil
}
```

### 3.4 L7 — Driver ChangePassword Interface

Extend DB Driver interface (TDD Section 6.1):

```go
type CredentialManager interface {
    ChangePassword(ctx context.Context, newPassword string) error
    // PostgreSQL: ALTER ROLE ... PASSWORD '...'
    // MySQL: ALTER USER ... IDENTIFIED BY '...'
}
```

---

## 4. Database Changes

```sql
ALTER TABLE data_source ADD COLUMN backup_password TEXT;
ALTER TABLE data_source ADD COLUMN backup_expiry TIMESTAMPTZ;
ALTER TABLE data_source ADD COLUMN rotation_schedule TEXT; -- cron expression
ALTER TABLE data_source ADD COLUMN last_rotated TIMESTAMPTZ;

CREATE TABLE credential_rotation_log (
    id          BIGSERIAL PRIMARY KEY,
    instance_uid INT NOT NULL,
    rotation_type TEXT NOT NULL,  -- "scheduled", "emergency", "manual"
    status      TEXT NOT NULL,    -- "success", "failed", "rollback"
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-015 (Secret Manager) | Vault dynamic secrets |
| CR-SEC-007 (Encryption at Rest) | Credentials encrypted in store |
| CR-SEC-010 (SIEM) | Rotation events to SIEM |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Rotation runner + dual-credential | Sprint 1 |
| 2 | Emergency rotation endpoint | Sprint 2 |
| 3 | Driver ChangePassword per engine | Sprint 3 |
| 4 | Vault dynamic secrets | Sprint 4 |

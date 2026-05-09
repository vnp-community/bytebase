# Change Request: Backup, Recovery & RPO/RTO Compliance

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-005                                                |
| **Title**          | Backup, Recovery & RPO/RTO Compliance                        |
| **Category**       | Availability / Data Protection                              |
| **Priority**       | P0 — Critical                                               |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | FFIEC BCM §J, ISO 22301 §8.4, SBV TT09/2020 Điều 11, PCI-DSS Req 12.10 |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế và triển khai hệ thống **backup & recovery** toàn diện cho Bytebase metadata, đảm bảo đạt **RPO ≤ 15 phút** và **RTO ≤ 30 phút** — tiêu chuẩn bắt buộc cho hệ thống thông tin ngành tài chính.

### 1.2 Bối cảnh
Hệ thống hiện tại có backup cơ bản cho dữ liệu managed databases (DCM-07: Automatic Backup Before Data Changes), nhưng **thiếu hoàn toàn** backup cho:
- **Metadata PostgreSQL** — toàn bộ cấu hình, IAM policies, audit logs, project settings
- **Configuration state** — environment variables, license, integrations
- **Point-in-Time Recovery (PITR)** — không thể phục hồi tới thời điểm cụ thể
- **Backup verification** — không có automated restore testing
- **Cross-region backup** — single site backup only
- **Compliance documentation** — thiếu RPO/RTO SLA documentation

### 1.3 Mục tiêu
- **RPO ≤ 15 phút** — Maximum data loss window
- **RTO ≤ 30 phút** — Maximum recovery time
- Automated backup schedule với retention policy
- Point-in-Time Recovery (PITR) capability
- Backup encryption at rest
- Automated backup verification (restore testing)
- Cross-region backup replication
- Compliance reporting cho regulatory audit

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                               |
|-----------------------------------|------------------------------------------------------------|
| FFIEC BCM Appendix J              | Data backup, recovery testing, off-site storage            |
| ISO 22301 §8.4                    | Business continuity plans and procedures                   |
| SBV TT09/2020 — Điều 11          | Dự phòng dữ liệu và phục hồi hệ thống                    |
| PCI-DSS 4.0 — Req 12.10          | Incident response, recovery procedures                    |
| ISO 27001 — A.12.3               | Information backup                                         |

---

## 2. Yêu cầu chức năng

### FR-001: Automated Metadata Backup
- **Mô tả**: Automated backup cho Bytebase metadata PostgreSQL database.
- **Backup Types**:
  | Type          | Method              | Schedule         | Retention |
  |---------------|---------------------|------------------|-----------|
  | Full          | pg_dump (custom)    | Daily 02:00 UTC  | 30 days   |
  | Incremental   | WAL archiving       | Continuous       | 7 days    |
  | Snapshot      | PG basebackup       | Weekly           | 90 days   |

- **Logic**:
  ```
  BackupScheduler:
      // Full backup (daily)
      CRON "0 2 * * *":
          backupID = generateBackupID()
          result = pgDump(metadata_db, {
              format:      "custom",
              compress:    9,
              parallel:    4,
              excludeTable: ["health_check_log", "bus_message_dedup"],
          })
          encryptedBackup = encrypt(result, backupEncryptionKey)
          store(encryptedBackup, {
              path: "backups/{date}/{backupID}.dump.enc",
              metadata: { size, checksum, tables, rowCounts }
          })
          IF remoteStorageConfigured:
              replicate(encryptedBackup, remoteStorage)
          verifyBackup(backupID)  // automated restore test

      // WAL archiving (continuous — RPO driver)
      archive_command = 'wal-g wal-push %p'
      // WAL files archived every archive_timeout (default: 60s)
      // Effective RPO: min(archive_timeout, WAL generation rate)
  ```
- **Acceptance Criteria**:
  - AC-1: Full backup completes within 30 minutes for databases ≤ 100GB
  - AC-2: WAL archiving lag ≤ 60 seconds (RPO ≤ 1 minute for PITR)
  - AC-3: Backup encryption with AES-256-GCM
  - AC-4: Backup metadata (size, checksum, timestamp) stored in backup_registry
  - AC-5: Failed backups trigger P1 alert within 5 minutes

### FR-002: Point-in-Time Recovery (PITR)
- **Mô tả**: Khôi phục metadata database tới bất kỳ thời điểm nào trong retention window.
- **Logic**:
  ```
  PITR Recovery Procedure:
      1. INPUT: target_timestamp
      2. VALIDATE: target_timestamp within retention window
      3. FIND: latest full backup BEFORE target_timestamp
      4. RESTORE: pg_restore from full backup
      5. REPLAY: WAL logs up to target_timestamp
      6. VERIFY: data integrity checks
      7. REPORT: recovery details (time, data state, verification)

  // API Endpoint (admin only)
  POST /v1/admin/recovery/pitr
  Body: {
      "targetTimestamp": "2026-05-08T10:30:00Z",
      "dryRun": true,  // estimate recovery time, data delta
      "targetDatabase": "bytebase_metadata"
  }
  ```
- **Acceptance Criteria**:
  - AC-1: PITR granularity ≤ 1 minute within WAL retention window
  - AC-2: Recovery completes within RTO (≤ 30 minutes)
  - AC-3: Dry-run mode estimates recovery time and data delta
  - AC-4: Recovery process logged in audit trail

### FR-003: Backup Verification & Restore Testing
- **Mô tả**: Automated verification của mỗi backup qua restore testing.
- **Logic**:
  ```
  BackupVerifier:
      AFTER each full backup:
          1. Restore backup tới isolated test database
          2. Run integrity checks:
              - Table count matches source
              - Row count within 1% tolerance
              - Critical tables verified (principal, project, instance, policy)
              - Foreign key constraints valid
              - Checksum verification
          3. Record verification result
          4. Drop test database
          5. IF verification fails:
              alertOperations("backup_verification_failed", backupID)
              retry backup

      QUARTERLY:
          // Full recovery drill
          1. Restore latest backup to staging environment
          2. Start Bytebase on restored data
          3. Run smoke tests (login, list projects, query)
          4. Measure recovery time
          5. Document results for audit
  ```
- **Acceptance Criteria**:
  - AC-1: Every backup verified within 1 hour of creation
  - AC-2: Verification failure triggers immediate P1 alert
  - AC-3: Quarterly full restore drill with documented results
  - AC-4: Restore test environment isolated from production

### FR-004: Cross-Region Backup Replication
- **Mô tả**: Replicate backups tới remote storage cho off-site protection.
- **Logic**:
  ```
  BackupReplicator:
      primaryStorage:    local or S3-compatible (primary region)
      secondaryStorage:  S3-compatible (DR region)

      AFTER each backup (full + WAL):
          replicate(backup, secondaryStorage, {
              encryption:  "AES-256-GCM",
              transport:   "TLS 1.3",
              verify:      true,  // checksum verification after upload
              retryPolicy: exponentialBackoff(maxAttempts=5)
          })

      DAILY:
          // Verify cross-region backup integrity
          FOR each recent backup (7 days):
              remoteChecksum = secondaryStorage.getChecksum(backup)
              localChecksum = primaryStorage.getChecksum(backup)
              IF remoteChecksum != localChecksum:
                  alertOperations("backup_replication_mismatch")
                  re-replicate(backup)
  ```
- **Acceptance Criteria**:
  - AC-1: Backups replicated to DR region within 1 hour
  - AC-2: Replication encrypted in transit (TLS 1.3)
  - AC-3: Checksums verified after replication
  - AC-4: Replication lag monitored and alerted

### FR-005: Backup Retention & Lifecycle Management
- **Mô tả**: Automated backup lifecycle management theo retention policy.
- **Retention Policy**:
  | Backup Type      | Retention     | Storage Tier          |
  |------------------|---------------|-----------------------|
  | WAL archives     | 7 days        | Hot (SSD/S3 Standard) |
  | Daily full       | 30 days       | Hot → Warm after 7d   |
  | Weekly snapshot   | 90 days       | Warm (S3 IA)          |
  | Monthly snapshot | 1 year        | Cold (S3 Glacier)     |
  | Yearly snapshot  | 7 years       | Archive (Glacier Deep)|

- **Logic**:
  ```
  BackupLifecycleManager:
      DAILY at 04:00 UTC:
          FOR each backup IN backup_registry:
              IF backup.age > retention[backup.type]:
                  IF backup.hasLegalHold:
                      SKIP  // regulatory hold
                  archive_or_delete(backup)
                  updateRegistry(backup, DELETED)

          // Compliance report
          generateRetentionReport({
              totalBackups: count,
              oldestBackup: date,
              newestBackup: date,
              storageUsed: bytes,
              retentionCompliance: percentage
          })
  ```
- **Acceptance Criteria**:
  - AC-1: Expired backups auto-deleted per retention policy
  - AC-2: Legal hold prevents deletion regardless of retention
  - AC-3: Monthly retention compliance report generated
  - AC-4: Storage costs tracked and reported

### FR-006: RPO/RTO Monitoring & Compliance Dashboard
- **Mô tả**: Continuous monitoring của RPO/RTO compliance.
- **Metrics**:
  ```
  RPO Monitoring:
      actual_rpo = now() - latest_successful_backup.timestamp
      IF actual_rpo > target_rpo (15 min):
          alert("rpo_violation", actual_rpo)

  RTO Monitoring:
      estimated_rto = estimateRecoveryTime(latestBackup, currentWALLag)
      IF estimated_rto > target_rto (30 min):
          alert("rto_at_risk", estimated_rto)

  Dashboard:
      - Current RPO status (actual vs target)
      - Current RTO estimate (based on latest backup)
      - Backup success/failure trend (30 days)
      - Storage usage trend
      - Last successful verification
      - Last DR drill results
      - Compliance score (% SLA met)
  ```
- **Acceptance Criteria**:
  - AC-1: RPO violation detected within 5 minutes
  - AC-2: RTO estimation updated every 15 minutes
  - AC-3: Compliance dashboard accessible to auditors
  - AC-4: Monthly compliance report auto-generated

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|---------------------------------------------------|
| Backup Scheduler             | `backend/runner/backup/scheduler.go`      | Cron-based backup orchestration                   |
| Backup Executor              | `backend/runner/backup/executor.go`       | pg_dump wrapper with encryption                   |
| WAL Archiver                 | `backend/runner/backup/wal_archiver.go`   | WAL-G integration for continuous archiving        |
| PITR Engine                  | `backend/component/recovery/pitr.go`      | Point-in-time recovery orchestration              |
| Backup Verifier              | `backend/runner/backup/verifier.go`       | Automated restore testing                         |
| Backup Replicator            | `backend/runner/backup/replicator.go`     | Cross-region backup replication                   |
| Lifecycle Manager            | `backend/runner/backup/lifecycle.go`      | Retention policy enforcement                      |
| RPO/RTO Monitor              | `backend/runner/backup/compliance.go`     | Continuous RPO/RTO monitoring                     |
| Backup API                   | `backend/api/v1/backup_service.go`        | Backup management API endpoints                   |
| Recovery API                 | `backend/api/v1/recovery_service.go`      | Recovery/PITR API endpoints                       |
| Backup Store                 | `backend/store/backup.go`                | Backup registry CRUD                              |
| Backup Encryption            | `backend/component/crypto/backup_enc.go`  | AES-256-GCM encryption for backups               |
| Storage Adapter              | `backend/component/storage/s3.go`        | S3-compatible storage adapter                     |
| Backup Metrics               | `backend/metrics/backup_metrics.go`       | Backup Prometheus metrics                         |

### 3.2 Configuration

| Environment Variable          | Default         | Mô tả                                                |
|-------------------------------|-----------------|-------------------------------------------------------|
| `BACKUP_ENABLED`             | `false`         | Enable automated backup system                       |
| `BACKUP_SCHEDULE_FULL`       | `0 2 * * *`     | Full backup cron schedule (daily 02:00 UTC)          |
| `BACKUP_SCHEDULE_SNAPSHOT`   | `0 3 * * 0`     | Snapshot cron schedule (weekly Sunday 03:00)          |
| `BACKUP_ENCRYPTION_KEY`      | _(required)_    | AES-256 encryption key (base64)                      |
| `BACKUP_LOCAL_PATH`          | `/data/backups`  | Local backup storage path                            |
| `BACKUP_S3_ENDPOINT`        | _(empty)_       | S3-compatible endpoint for remote storage            |
| `BACKUP_S3_BUCKET`          | _(empty)_       | S3 bucket name                                        |
| `BACKUP_S3_ACCESS_KEY`      | _(empty)_       | S3 access key                                         |
| `BACKUP_S3_SECRET_KEY`      | _(empty)_       | S3 secret key                                         |
| `BACKUP_S3_REGION`          | _(empty)_       | S3 region                                             |
| `BACKUP_DR_S3_ENDPOINT`     | _(empty)_       | DR region S3 endpoint                                 |
| `BACKUP_DR_S3_BUCKET`       | _(empty)_       | DR region S3 bucket                                   |
| `BACKUP_RETENTION_DAILY`    | `30`            | Daily backup retention in days                        |
| `BACKUP_RETENTION_WEEKLY`   | `90`            | Weekly snapshot retention in days                      |
| `BACKUP_RETENTION_MONTHLY`  | `365`           | Monthly snapshot retention in days                     |
| `BACKUP_RETENTION_YEARLY`   | `2555`          | Yearly snapshot retention in days (7 years)           |
| `BACKUP_WAL_RETENTION_DAYS` | `7`             | WAL archive retention in days                         |
| `TARGET_RPO_MINUTES`        | `15`            | RPO target in minutes                                 |
| `TARGET_RTO_MINUTES`        | `30`            | RTO target in minutes                                 |
| `BACKUP_VERIFY_ENABLED`     | `true`          | Enable automated backup verification                 |
| `WAL_ARCHIVE_TOOL`          | `wal-g`         | WAL archiving tool (wal-g, pgbackrest)               |

### 3.3 Database Changes

```sql
-- Backup registry
CREATE TABLE IF NOT EXISTS backup_registry (
    id              BIGSERIAL   PRIMARY KEY,
    backup_id       TEXT        UNIQUE NOT NULL,
    backup_type     TEXT        NOT NULL,
    -- Types: FULL, INCREMENTAL, SNAPSHOT, WAL
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    -- Status: IN_PROGRESS, COMPLETED, FAILED, VERIFIED, DELETED
    size_bytes      BIGINT      NOT NULL DEFAULT 0,
    checksum_sha256 TEXT,
    storage_path    TEXT        NOT NULL,
    storage_type    TEXT        NOT NULL DEFAULT 'LOCAL',
    -- Storage: LOCAL, S3_PRIMARY, S3_DR
    encryption      TEXT        NOT NULL DEFAULT 'AES-256-GCM',
    tables_included TEXT[]      NOT NULL DEFAULT '{}',
    row_counts      JSONB       NOT NULL DEFAULT '{}',
    
    -- RPO/RTO tracking
    data_timestamp  TIMESTAMPTZ NOT NULL,  -- point-in-time of backup data
    
    -- Verification
    verified_at     TIMESTAMPTZ,
    verified_result TEXT,
    
    -- Replication
    replicated_to   TEXT[]      NOT NULL DEFAULT '{}',
    replicated_at   TIMESTAMPTZ,
    
    -- Retention
    legal_hold      BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at      TIMESTAMPTZ NOT NULL,
    
    -- Metadata
    initiated_by    TEXT        NOT NULL DEFAULT 'SYSTEM',
    node_id         TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backup_registry_type_status
    ON backup_registry (backup_type, status);
CREATE INDEX idx_backup_registry_data_time
    ON backup_registry (data_timestamp DESC);
CREATE INDEX idx_backup_registry_expires
    ON backup_registry (expires_at) WHERE status != 'DELETED';

-- Recovery event log
CREATE TABLE IF NOT EXISTS recovery_event (
    id              BIGSERIAL   PRIMARY KEY,
    recovery_type   TEXT        NOT NULL,
    -- Types: FULL_RESTORE, PITR, PARTIAL_RESTORE, DR_DRILL
    source_backup   TEXT        REFERENCES backup_registry(backup_id),
    target_timestamp TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    initiated_by    TEXT        NOT NULL,
    approved_by     TEXT[],
    
    -- Timing
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    actual_rto_min  FLOAT,
    actual_rpo_min  FLOAT,
    
    -- Verification
    verification    JSONB       NOT NULL DEFAULT '{}',
    -- { tableCount, rowCountMatch, fkValid, checksumValid }
    
    details         JSONB       NOT NULL DEFAULT '{}',
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Compliance snapshot (daily)
CREATE TABLE IF NOT EXISTS backup_compliance (
    id              BIGSERIAL   PRIMARY KEY,
    date            DATE        NOT NULL UNIQUE,
    current_rpo_min FLOAT       NOT NULL,
    target_rpo_min  FLOAT       NOT NULL,
    rpo_compliant   BOOLEAN     NOT NULL,
    estimated_rto   FLOAT       NOT NULL,
    target_rto_min  FLOAT       NOT NULL,
    rto_compliant   BOOLEAN     NOT NULL,
    total_backups   INT         NOT NULL DEFAULT 0,
    verified_backups INT        NOT NULL DEFAULT 0,
    failed_backups  INT         NOT NULL DEFAULT 0,
    storage_used_gb FLOAT       NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3.4 Prometheus Metrics

```prometheus
# Backup
bytebase_backup_last_success_timestamp
bytebase_backup_last_duration_seconds
bytebase_backup_size_bytes{type="full"}
bytebase_backup_total{status="completed"}
bytebase_backup_total{status="failed"}
bytebase_backup_verification_status  # 0=failed, 1=passed

# RPO/RTO
bytebase_rpo_current_minutes
bytebase_rpo_target_minutes
bytebase_rpo_compliant  # 0=violation, 1=compliant
bytebase_rto_estimated_minutes
bytebase_rto_target_minutes

# Replication
bytebase_backup_replication_lag_seconds
bytebase_backup_replication_status  # 0=failed, 1=synced

# WAL
bytebase_wal_archive_lag_seconds
bytebase_wal_archive_total
bytebase_wal_archive_failed_total

# Storage
bytebase_backup_storage_used_bytes{tier="hot"}
bytebase_backup_storage_used_bytes{tier="warm"}
bytebase_backup_storage_used_bytes{tier="cold"}
```

### 3.5 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| Backup management page       | Admin page for backup status, history, actions    |
| Recovery page                | PITR interface, recovery wizard                   |
| Compliance dashboard         | RPO/RTO status, compliance score, audit reports   |
| Backup alerts                | Alert notifications for backup failures           |

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-AVAIL-002            | DR infrastructure — backups feed DR procedures                  |
| CR-AVAIL-006            | Multi-Region — cross-region backup replication                  |
| WAL-G / pgBackRest      | WAL archiving tool                                              |
| S3-compatible storage    | MinIO, AWS S3, GCS for remote backup                            |
| PostgreSQL 14+           | pg_dump, WAL archiving, PITR capabilities                      |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | Full backup execution                                          | Backup completed, encrypted, registered |
| TC-002     | WAL archiving continuous                                       | WAL lag ≤ 60 seconds                    |
| TC-003     | Backup verification (restore test)                             | All integrity checks pass               |
| TC-004     | PITR to specific timestamp                                     | Data restored to exact point-in-time    |
| TC-005     | Cross-region backup replication                                | Backup replicated, checksum verified    |
| TC-006     | RPO monitoring — backup delayed                                | Alert fired when RPO > target           |
| TC-007     | RTO estimation accuracy                                        | Estimate within 20% of actual           |
| TC-008     | Backup retention — expired backup                              | Auto-deleted per retention policy       |
| TC-009     | Legal hold prevents deletion                                   | Backup retained despite expiry          |
| TC-010     | Backup failure — disk full                                     | P1 alert within 5 minutes              |
| TC-011     | Recovery drill — full restore                                  | Service operational within RTO          |
| TC-012     | Compliance report generation                                   | Report generated with all metrics       |
| TC-013     | Backup encryption verification                                 | Encrypted backup unreadable without key |
| TC-014     | Concurrent backup during high load                             | Backup completes without service impact |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Full backup with pg_dump + encryption            | Sprint 1       |
| Phase 2 | WAL archiving with WAL-G                        | Sprint 1-2     |
| Phase 3 | Backup verification automation                   | Sprint 2       |
| Phase 4 | PITR implementation                              | Sprint 3       |
| Phase 5 | Cross-region replication                         | Sprint 3-4     |
| Phase 6 | RPO/RTO monitoring + compliance dashboard        | Sprint 4       |
| Phase 7 | Retention lifecycle management                   | Sprint 4-5     |
| Phase 8 | DR drill + regulatory documentation              | Sprint 5-6     |

---

## 7. Risks & Mitigations

| Risk                                         | Impact | Mitigation                                                |
|----------------------------------------------|--------|-----------------------------------------------------------|
| Backup storage costs at scale                | MEDIUM | Tiered storage, compression (9x), lifecycle policies      |
| WAL archiving lag during heavy write load    | HIGH   | Parallel WAL shipping, larger archive_timeout             |
| PITR recovery time exceeds RTO               | HIGH   | Regular basebackups reduce WAL replay time                |
| Encryption key loss = total data loss         | CRITICAL| Key stored in HSM/KMS, key escrow with dual control      |
| Backup corruption undetected                 | HIGH   | Checksums + automated restore verification               |
| Cross-region bandwidth costs                 | MEDIUM | Incremental replication, compression, off-peak scheduling |

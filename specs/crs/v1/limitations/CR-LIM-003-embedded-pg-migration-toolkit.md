# Change Request: Embedded PostgreSQL Production Migration Toolkit

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-003                                               |
| **Limitation ID**  | LIM-003                                                  |
| **Title**          | Embedded PostgreSQL Production Migration Toolkit         |
| **Category**       | Deployment / Infrastructure                              |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Cung cấp **migration toolkit** tự động hóa việc chuyển đổi từ embedded PostgreSQL sang external PostgreSQL, cải thiện **health monitoring** cho embedded PG, và thêm **production readiness checks** để cảnh báo người dùng khi chạy embedded PG trong production.

### 1.2 Bối cảnh
Embedded PostgreSQL phù hợp cho evaluation/demo nhưng không hỗ trợ HA, PITR, WAL archiving, hoặc performance tuning. Nhiều users bắt đầu với embedded PG rồi cần migrate khi đưa lên production — quá trình này hiện hoàn toàn thủ công.

### 1.3 Mục tiêu
- Automated migration tool: embedded → external PostgreSQL
- Production readiness warnings khi phát hiện embedded PG trong production-like usage
- Health dashboard cho embedded PG (connections, storage, performance)
- Clear documentation cho migration path

---

## 2. Yêu cầu chức năng

### FR-001: Production Readiness Check & Warning
- **Mô tả**: Tự động phát hiện khi embedded PG đang được sử dụng trong điều kiện production-like.
- **Detection Criteria**:
  - Instance count > 5
  - User count > 10
  - Database change history > 100 entries
  - Uptime > 30 days
  - Environment có "prod" hoặc "production" tier
- **Warning Display**: Banner trên dashboard + audit log entry
- **Acceptance Criteria**:
  - AC-1: Warning banner hiển thị khi ≥ 2 criteria thỏa mãn
  - AC-2: Banner chứa link tới migration guide
  - AC-3: Warning có thể dismiss tạm (30 ngày) nhưng sẽ xuất hiện lại
  - AC-4: API endpoint trả về production readiness status

### FR-002: Automated Migration Tool (embedded → external PG)
- **Mô tả**: CLI tool và Web UI wizard để migrate dữ liệu từ embedded PG sang external PG.
- **Migration Steps**:
  1. Validate external PG connectivity và version compatibility
  2. Put Bytebase vào maintenance mode
  3. `pg_dump` từ embedded PG
  4. Create schema + `pg_restore` vào external PG
  5. Verify data integrity (row counts, checksums)
  6. Update `PG_URL` configuration
  7. Restart Bytebase pointing to external PG
  8. Validate all services operational
- **Acceptance Criteria**:
  - AC-1: CLI: `bytebase migrate-db --target-url <PG_URL> [--dry-run]`
  - AC-2: Web UI wizard với step-by-step progress
  - AC-3: Dry-run mode chỉ validate mà không thực hiện migration
  - AC-4: Rollback capability nếu migration fails
  - AC-5: Data integrity verification sau migration (row count comparison)

### FR-003: Embedded PG Health Monitoring
- **Mô tả**: Dashboard hiển thị health metrics cho embedded PostgreSQL.
- **Metrics**:
  | Metric                    | Source                  | Warning Threshold   |
  |---------------------------|-------------------------|---------------------|
  | Active connections        | `pg_stat_activity`      | > 80% max_conn     |
  | Database size             | `pg_database_size()`    | > 80% disk space   |
  | Transaction rate          | `pg_stat_database`      | N/A (info only)    |
  | Longest running query     | `pg_stat_activity`      | > 60 seconds       |
  | Replication status        | N/A (embedded)          | Always "N/A"       |
  | WAL size                  | `pg_wal_lsn_diff()`     | > 1GB              |
- **Acceptance Criteria**:
  - AC-1: Health widget trên Settings page
  - AC-2: Metrics refresh every 30 seconds
  - AC-3: Alert khi metric vượt warning threshold

### FR-004: Embedded PG Backup Enhancement
- **Mô tả**: Cung cấp scheduled backup cho embedded PG.
- **Features**:
  - Scheduled pg_dump (daily, configurable)
  - Backup rotation (keep last N backups, default 7)
  - Backup tới local directory hoặc S3-compatible storage
  - One-click restore from backup
- **Acceptance Criteria**:
  - AC-1: Backup schedule configurable qua Settings UI
  - AC-2: Backup files compressed (gzip)
  - AC-3: Backup status visible trên Health dashboard
  - AC-4: Restore validates backup integrity trước khi apply

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| Migration CLI                | `backend/cmd/migrate.go`                  | CLI command cho embedded → external migration      |
| Migration Service            | `backend/component/dbmigrate/service.go`  | Core migration logic                               |
| Readiness Checker            | `backend/component/dbmigrate/readiness.go`| Production readiness detection                     |
| PG Health Monitor            | `backend/component/pghealth/monitor.go`   | Embedded PG health metrics collection              |
| Backup Scheduler             | `backend/component/pgbackup/scheduler.go` | Scheduled backup for embedded PG                   |
| Health API                   | `backend/api/v1/actuator_service.go`      | Expose PG health + readiness endpoints             |
| Settings UI                  | `frontend/src/views/Settings.vue`         | Health dashboard + migration wizard                |

### 3.2 Configuration

| Environment Variable       | Default          | Mô tả                                      |
|----------------------------|------------------|--------------------------------------------|
| `PG_BACKUP_SCHEDULE`       | `0 2 * * *`      | Cron expression cho backup schedule        |
| `PG_BACKUP_RETENTION`      | `7`              | Number of backups to retain                |
| `PG_BACKUP_DIR`            | `{dataDir}/backup`| Backup storage directory                  |
| `PG_BACKUP_S3_URL`         | _(empty)_        | S3-compatible URL for remote backup        |

### 3.3 Database Changes
Không cần schema migration — sử dụng pg_dump/pg_restore.

---

## 4. Phụ thuộc

| Dependency          | Mô tả                                                    |
|---------------------|-----------------------------------------------------------|
| `pg_dump`           | Bundled với embedded PG                                   |
| `pg_restore`        | Bundled với embedded PG                                   |
| External PG 14+     | Target cho migration                                      |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                        |
|------------|----------------------------------------------------------|----------------------------------------|
| TC-001     | Migration embedded → external PG (happy path)           | All data migrated, checksums match     |
| TC-002     | Migration dry-run mode                                   | Validation only, no data moved         |
| TC-003     | Migration with invalid target PG URL                     | Clear error, no data loss              |
| TC-004     | Migration rollback on failure                            | Embedded PG unchanged, error logged    |
| TC-005     | Production readiness check with 3+ criteria met          | Warning banner displayed               |
| TC-006     | Health monitor with high connection count                | Warning threshold triggered            |
| TC-007     | Scheduled backup creation and rotation                   | Backup created, old ones cleaned       |
| TC-008     | Restore from backup                                      | Data restored, integrity verified      |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Production readiness checker + warnings          | Sprint 1       |
| Phase 2 | PG health monitoring dashboard                   | Sprint 2       |
| Phase 3 | Migration CLI tool                               | Sprint 3       |
| Phase 4 | Web UI migration wizard                          | Sprint 4       |
| Phase 5 | Backup scheduler                                 | Sprint 4       |
| Phase 6 | Documentation + testing                          | Sprint 5       |

---

## 7. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                          |
|-----------------------------------------|--------|------------------------------------------------------|
| Data loss during migration              | HIGH   | Mandatory backup before migration, dry-run first     |
| Embedded PG version mismatch            | MEDIUM | Version check before pg_dump/restore                 |
| Large database migration time           | MEDIUM | Progress indicator, maintenance mode                 |
| Backup storage exhaustion               | LOW    | Rotation policy, size monitoring                     |

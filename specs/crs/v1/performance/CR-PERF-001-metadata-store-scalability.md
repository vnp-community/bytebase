# Change Request: Metadata Store Scalability for 200K+ Databases

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-001                                              |
| **Title**          | Metadata Store — PostgreSQL Partitioning & Index Optimization |
| **Category**       | Performance / Database Scalability                       |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-09, DCM-12, ADM-01, ADM-04                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Tối ưu PostgreSQL metadata store để hỗ trợ >200K databases trên >100 bank tenants. Bảng `db` và `db_schema` hiện không có partitioning, gây full table scan khi query cross-tenant.

### 1.2 Bối cảnh
- Bảng `db` sử dụng `UNIQUE(instance, name)` và JOIN `instance` qua `resource_id`
- `ListDatabases` luôn JOIN `instance` để lấy `workspace` và `engine` — overhead lớn
- Schema syncer gọi `ListDatabases(ctx, &FindDatabaseMessage{})` — load toàn bộ DB vào memory
- `ORDER BY db.project, db.instance, db.name` trên 200K rows thiếu composite index
- `COALESCE(db.environment, instance.environment)` trong WHERE không sargable

### 1.3 Mục tiêu
- Hỗ trợ >200K databases trên single deployment
- `ListDatabases` với filter < 50ms P99 cho page ≤ 1000
- Giảm I/O amplification 80%+ qua partitioning và covering indexes

---

## 2. Yêu cầu chức năng

### FR-001: Table Partitioning theo Workspace
- **Mô tả**: Denormalize `workspace` vào bảng `db`, partition by HASH(workspace)
- **Partitions**: 16 hash partitions (tối ưu cho >100 tenants)
- **AC**:
  - AC-1: Query có workspace filter chỉ scan partition tương ứng
  - AC-2: Zero data loss trong migration
  - AC-3: Rollback plan: giữ bảng gốc 72h

### FR-002: Composite Index Optimization
- **Indexes**:
  - `idx_db_workspace_project_instance_name ON db(workspace, project, instance, name) WHERE deleted=false`
  - `idx_db_environment ON db(environment) WHERE environment IS NOT NULL AND deleted=false`
  - `idx_instance_workspace_resource_id ON instance(workspace, resource_id) WHERE deleted=false`
- **AC**:
  - AC-1: ListDatabases với workspace+project filter → Index Only Scan
  - AC-2: Không seq scan trên bảng >100K rows

### FR-003: Materialized Effective Environment
- **Mô tả**: Pre-compute `effective_environment` thay vì runtime COALESCE
- **AC**:
  - AC-1: Direct index lookup thay vì COALESCE
  - AC-2: Instance environment change propagate <5s

### FR-004: Connection Pool Auto-tuning
- **Logic**: `pool_size = min(50 + dbCount/1000 + tenantCount*2, PG_MAX_CONNECTIONS)`
- **AC**:
  - AC-1: Pool auto-adjust khi DB count thay đổi
  - AC-2: Separate read/write pool (80/20)
  - AC-3: Alert khi utilization >80%

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                              | Thay đổi                              |
|--------------------|-----------------------------------|----------------------------------------|
| DB Migration       | `backend/migrator/migration/`     | Partition migration script             |
| Store — Database   | `backend/store/database.go`       | Use denormalized workspace, remove COALESCE |
| Store Constructor  | `backend/store/store.go`          | Auto-tune pool size                    |
| Pool Manager       | `backend/store/pool.go`           | Separate read/write pool               |

### 3.2 Database Changes

```sql
ALTER TABLE db ADD COLUMN workspace TEXT;
UPDATE db SET workspace = (SELECT workspace FROM instance WHERE resource_id = db.instance);
ALTER TABLE db ALTER COLUMN workspace SET NOT NULL;

ALTER TABLE db ADD COLUMN effective_environment TEXT;
UPDATE db SET effective_environment = COALESCE(db.environment,
    (SELECT environment FROM instance WHERE resource_id = db.instance));

CREATE INDEX CONCURRENTLY idx_db_workspace_project ON db(workspace, project, instance, name) WHERE deleted=false;
CREATE INDEX CONCURRENTLY idx_db_effective_env ON db(effective_environment) WHERE deleted=false;
```

---

## 4. Performance Targets

| Metric                  | Current (10K) | Target (200K+) |
|-------------------------|---------------|----------------|
| ListDatabases P99       | ~45ms         | ≤ 50ms         |
| GetDatabase (uncached)  | ~5ms          | ≤ 8ms          |
| Full scan size          | 10K rows      | ≤ 12.5K/partition |
| Schema sync full cycle  | ~2 min        | ≤ 5 min        |

---

## 5. Test Cases

| Test ID | Mô tả                                          | Expected Result                 |
|---------|--------------------------------------------------|----------------------------------|
| TC-001  | ListDatabases 200K records, workspace filter     | P99 < 50ms, partition pruning   |
| TC-002  | ListDatabases project+env filter                 | Index only scan                  |
| TC-003  | CreateDatabase auto-set workspace                | Column populated correctly       |
| TC-004  | Instance env change → effective_environment      | Updated <5s                      |
| TC-005  | Partition migration zero data loss               | COUNT match pre/post             |
| TC-006  | Concurrent writes across partitions              | No lock contention               |

---

## 6. Rollout Plan

| Phase   | Mô tả                                    | Timeline   |
|---------|-------------------------------------------|------------|
| Phase 1 | Denormalize workspace + indexes           | Sprint 1   |
| Phase 2 | Effective environment materialization     | Sprint 1   |
| Phase 3 | Table partitioning (staging)              | Sprint 2   |
| Phase 4 | Connection pool auto-tuning               | Sprint 2   |
| Phase 5 | Production partition migration            | Sprint 3   |
| Phase 6 | Load testing 200K benchmark               | Sprint 3   |

---

## 7. Risks & Mitigations

| Risk                              | Impact | Mitigation                                   |
|-----------------------------------|--------|----------------------------------------------|
| Partition migration downtime      | HIGH   | Online migration via shadow table swap       |
| Workspace denorm data drift       | MEDIUM | Trigger-based sync + consistency check       |
| Over-partitioning                 | LOW    | Start 16, expand to 32 if needed             |
| Index bloat                       | MEDIUM | REINDEX CONCURRENTLY + autovacuum tuning     |

# Change Request: Database Performance Metrics Collector

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-005                                               |
| **Gap ID**         | G5                                                       |
| **Title**          | Database Performance Metrics Collector                   |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Module thu thập và hiển thị performance metrics cơ bản từ tất cả DB instances trực tiếp trong Bytebase. Mục tiêu không thay thế Grafana/Prometheus stack nhưng cung cấp **quick-glance performance view** cho DBA ngay trong Bytebase UI, đặc biệt sau deployment changes.

### 1.2 Mục tiêu
- Quick performance overview per instance trong Bytebase
- Post-deployment performance comparison (before/after change)
- Slow query detection từ DB stats views
- Performance alerts cho anomalies sau change rollout

---

## 2. Yêu cầu chức năng

### FR-001: Metrics Collector
Collect key metrics từ mỗi engine:

| Category | Oracle | PostgreSQL | MySQL | MSSQL | MongoDB |
|---|---|---|---|---|---|
| Connections | `v$session` count | `pg_stat_activity` | `SHOW STATUS 'Threads_connected'` | `sys.dm_exec_sessions` | `db.serverStatus().connections` |
| Query Rate | `v$sysstat` | `pg_stat_statements` | `SHOW STATUS 'Queries'` | `sys.dm_exec_query_stats` | `db.serverStatus().opcounters` |
| Cache Hit | `v$sga_stat` | `pg_stat_database.blks_hit` | `Innodb_buffer_pool_read_requests` | Buffer cache hit ratio | `db.serverStatus().wiredTiger.cache` |
| Slow Queries | `v$sql` (elapsed_time) | `pg_stat_statements` (mean_time) | `slow_query_log` | Wait stats | `db.currentOp({secs_running:{$gt:5}})` |
| Locks/Waits | `v$lock` | `pg_locks` | `SHOW ENGINE INNODB STATUS` | `sys.dm_tran_locks` | `db.currentOp({waitingForLock:true})` |

- Polling interval: 60s (configurable)
- Retention: 7 days in Bytebase DB, older data summarized

### FR-002: Instance Health Dashboard
- **Health Score**: 0-100 composite score per instance
- **Time-series charts**: Connections, QPS, cache hit ratio, latency
- **Change Impact View**: Overlay deployment markers on charts
  - Before vs After comparison for each rollout
- **Top Slow Queries**: List with execution count and avg time

### FR-003: Performance Alerts
- Threshold-based alerts:
  - Connection count > N% of max
  - Cache hit ratio < threshold
  - Slow query count spike
  - Lock wait time increase
- Post-change monitoring: automatic 1-hour intensive monitoring after rollout

### FR-004: Prometheus Metrics Export
- Expose collected metrics as Prometheus endpoint (`/metrics`)
- Allow external Grafana to scrape Bytebase for DB health
- Standard metric naming: `bytebase_db_connections_active{instance="...", engine="..."}`

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Metrics Collector | `backend/component/metrics/db_collector.go` | Polling engine |
| Metric Plugins | `backend/plugin/db/*/metrics_query.go` | Engine-specific queries |
| Metrics Store | `backend/store/db_metrics.go` | Time-series storage |
| Metrics API | `backend/api/v1/metrics_service.go` | API endpoints |
| Prometheus Exporter | `backend/component/metrics/prometheus.go` | /metrics endpoint |
| Health Dashboard | `frontend/src/views/InstanceHealth/` | Charts & scores |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Collect PG metrics → dashboard shows charts | Correct time-series |
| TC-002 | Deploy change → before/after overlay | Change marker visible |
| TC-003 | Connection spike → alert triggered | Notification sent |
| TC-004 | Prometheus scrape `/metrics` | Valid metric format |
| TC-005 | 7-day retention → old data summarized | Hourly averages kept |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Metrics collector + basic dashboard | Sprint 1-2 |
| Phase 2 | Change impact view + alerts | Sprint 3 |
| Phase 3 | Prometheus export + slow query detail | Sprint 4 |

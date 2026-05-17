# Change Request: Active Session Monitoring Dashboard

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-003                                               |
| **Gap ID**         | G3                                                       |
| **Title**          | Active Session Monitoring Dashboard                      |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Dashboard giám sát active sessions trên tất cả DB instances, theo dõi kết nối user cũ/mới real-time. Hiện tại Bytebase SQL Editor cho phép DBA query `pg_stat_activity` thủ công. CR này xây dựng **automated, real-time session monitoring** tích hợp vào Bytebase UI.

### 1.2 Bối cảnh
Sau chuyển đổi user (Phase triển khai password policy), cần theo dõi sessions user cũ vẫn active để đảm bảo migration hoàn tất. Hiện cần PMM hoặc pganalyze — tool bên ngoài.

### 1.3 Mục tiêu
- Real-time session monitoring cross-engine
- Track user cũ vs user mới sau migration
- Alert khi user cũ vẫn có active sessions sau deadline
- Kill session capability từ UI (với approval)

---

## 2. Yêu cầu chức năng

### FR-001: Session Collector
Polling-based session collection từ mỗi engine:

| Engine | Source | Key Metrics |
|---|---|---|
| Oracle | `v$session`, `v$process` | SID, serial#, username, status, machine, program, sql_id, logon_time |
| PostgreSQL | `pg_stat_activity` | pid, usename, client_addr, state, query, query_start, backend_start |
| MySQL | `SHOW PROCESSLIST`, `information_schema.processlist` | id, user, host, db, command, time, state, info |
| SQL Server | `sys.dm_exec_sessions`, `sys.dm_exec_requests` | session_id, login_name, host_name, program_name, status |
| MongoDB | `db.currentOp()` | client, connectionId, opid, active, secs_running |

- Polling interval: configurable (default 30s)
- Lightweight — chỉ collect metadata, không collect query content (configurable)

### FR-002: Session Dashboard
- **Overview Panel**: Tổng sessions per instance, per user, per state
- **User Migration Tracker**: So sánh sessions old user vs new user
  - Configurable "old users" watchlist
  - Timeline chart showing session count over time
- **Session Detail View**: Click vào user → xem all active sessions
- **Filters**: By engine, instance, user, state, duration

### FR-003: Session Alerts
- Alert khi old user vẫn có sessions sau X ngày (configurable)
- Alert khi session count vượt threshold
- Alert khi long-running sessions (> N hours)
- Delivery: Slack, Email, In-app notification

### FR-004: Session Management (with Approval)
- **Kill Session**: DBA request kill session → tạo approval Issue
- **Bulk Kill**: Kill tất cả sessions của specific user
- Auto-generate engine-specific kill commands:
  - Oracle: `ALTER SYSTEM KILL SESSION 'sid,serial#'`
  - PG: `SELECT pg_terminate_backend(pid)`
  - MySQL: `KILL CONNECTION id`

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Session Collector | `backend/component/session/collector.go` | Polling & caching |
| Session Plugins | `backend/plugin/db/*/session_query.go` | Engine-specific queries |
| Session API | `backend/api/v1/session_service.go` | REST/gRPC endpoints |
| Session Dashboard | `frontend/src/views/SessionMonitor/` | Dashboard UI |
| Migration Tracker | `frontend/src/components/SessionMigrationTracker.vue` | Old vs New user chart |
| Session Alerts | `backend/component/session/alerter.go` | Alert rules |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Collect sessions từ PG instance | Sessions listed in dashboard |
| TC-002 | Old user có 5 sessions → alert triggered | Notification sent |
| TC-003 | Kill session via UI → approval flow | Issue created, session killed after approve |
| TC-004 | Timeline chart: old user sessions decrease over time | Correct visualization |
| TC-005 | MongoDB currentOp collection | Sessions displayed correctly |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Session collector + basic dashboard | Sprint 1-2 |
| Phase 2 | Migration tracker + alerts | Sprint 3 |
| Phase 3 | Session kill with approval | Sprint 4 |

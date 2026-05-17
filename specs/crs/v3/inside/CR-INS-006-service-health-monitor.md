# Change Request: Service Health Check Monitor

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-006                                               |
| **Gap ID**         | G6                                                       |
| **Title**          | Service Health Check Monitor                             |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Module giám sát service health sau khi thực hiện database changes (đặc biệt sau chuyển đổi user). Thực hiện TCP check, SQL query check, và HTTP health check cho các dịch vụ liên quan đến database instance vừa thay đổi.

### 1.2 Bối cảnh
Gap G6 yêu cầu service uptime monitoring sau chuyển user. Hiện đề xuất Uptime Kuma. Tuy nhiên, Bytebase đã có connection tới DB — mở rộng để check application-level health tạo closed-loop monitoring.

### 1.3 Mục tiêu
- Post-change health verification tự động
- DB connectivity check qua new credentials
- Application endpoint monitoring sau rollout
- Auto-rollback trigger khi health check fail

---

## 2. Yêu cầu chức năng

### FR-001: Health Check Registry
- Register health checks cho mỗi database/application pair:
  - **DB TCP Check**: Kết nối TCP tới DB port (1521, 5432, 3306, 1433, 27017)
  - **DB SQL Check**: Execute `SELECT 1` hoặc custom query qua new user
  - **HTTP Check**: GET/POST tới application health endpoint
  - **Custom Script**: Chạy script kiểm tra tùy chỉnh
- Associate health checks với specific databases/instances

### FR-002: Post-Rollout Monitoring
- Tự động kích hoạt intensive monitoring sau mỗi rollout:
  - Duration: configurable (default 30 minutes)
  - Interval: every 30 seconds
  - Check types: all registered checks cho impacted databases
- Dashboard: real-time health status during monitoring window
- Traffic light status: 🟢 Healthy / 🟡 Degraded / 🔴 Down

### FR-003: Health Check Alerts & Auto-Rollback
- Alert khi health check fails liên tục (configurable threshold: 3 consecutive fails)
- Optional auto-rollback: revert last change if health check fails
  - Requires pre-configured rollback plan in Issue
  - Auto-create rollback Issue with pre-populated SQL
- Alert channels: Slack, Email, PagerDuty

### FR-004: Historical Health Data
- Track uptime percentage per service/database
- Correlate health events with database changes
- SLA reporting: uptime % over configurable period

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Health Check Registry | `backend/store/health_check.go` | CRUD health checks |
| Health Executor | `backend/component/healthcheck/executor.go` | Execute checks |
| Post-Rollout Monitor | `backend/component/healthcheck/post_rollout.go` | Auto monitoring |
| Health API | `backend/api/v1/health_check_service.go` | API endpoints |
| Health Dashboard | `frontend/src/views/HealthMonitor/` | Real-time status |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | DB TCP check to PG port | 🟢 Healthy |
| TC-002 | SQL check via new user credentials | SELECT 1 succeeds |
| TC-003 | HTTP check fails → alert | Notification sent |
| TC-004 | 3 consecutive fails → auto-rollback | Rollback Issue created |
| TC-005 | Post-rollout 30min window monitoring | Timeline visible |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Health check registry + executor | Sprint 1-2 |
| Phase 2 | Post-rollout monitoring | Sprint 3 |
| Phase 3 | Auto-rollback + SLA reporting | Sprint 4 |

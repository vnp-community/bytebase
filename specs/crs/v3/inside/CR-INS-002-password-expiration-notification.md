# Change Request: Password Expiration Notification Engine

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-002                                               |
| **Gap ID**         | G2                                                       |
| **Title**          | Password Expiration Notification Engine                  |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Depends On**     | CR-INS-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Engine thông báo hết hạn mật khẩu tích hợp vào Bytebase. Quét password expiry date trên tất cả DB instances, gửi notification qua Slack/Email/Webhook/In-app với escalation logic.

### 1.2 Bối cảnh
Không có tool open source dedicated cho cross-DB password expiry notification. Tích hợp vào Bytebase leverage existing DB connections và webhook infrastructure.

### 1.3 Mục tiêu
- Proactive notification (14d, 7d, 3d, 1d trước hết hạn)
- Multi-channel delivery
- Escalation logic (DBA → Manager → CISO)
- Auto-create renewal Issue khi gần deadline

---

## 2. Yêu cầu chức năng

### FR-001: Password Expiry Scanner
Scheduled job quét expiry trên tất cả instances:

| Engine | Query |
|---|---|
| Oracle | `SELECT username, expiry_date FROM dba_users` |
| PostgreSQL | `SELECT usename, valuntil FROM pg_user` |
| MySQL | `SELECT user, password_last_changed, password_lifetime FROM mysql.user` |
| SQL Server | `SELECT name, LOGINPROPERTY(name,'DaysUntilExpiration') FROM sys.sql_logins` |
| MongoDB | Custom metadata collection scan |

- Scan interval: configurable (default 6h)
- Differential scan — chỉ report changes

### FR-002: Notification Rules Engine
- Rule Builder: days until expiry, engine type, user type, environment tier
- Notification templates với variables: `{{username}}`, `{{db_instance}}`, `{{expiry_date}}`, `{{days_remaining}}`
- Deduplication logic
- Snooze capability (max 7 days)

### FR-003: Escalation Matrix
| Trigger | L1 (DBA) | L2 (Manager) | L3 (CISO) |
|---|---|---|---|
| 14 days | ✅ | | |
| 7 days | ✅ | ✅ CC | |
| 3 days | ✅ Urgent | ✅ | |
| 1 day | ✅ Critical | ✅ Urgent | ✅ CC |
| Expired | ✅ + Auto-Issue | ✅ Critical | ✅ |

### FR-004: Auto-Remediation
- Auto-Create Issue khi ≤ 3 days: pre-populated renewal SQL
- Optional auto-lock expired accounts sau grace period (default 7 days)
- Exclude service accounts

### FR-005: Expiry Dashboard Widget
- Countdown timers, color-coded: Green(>14d), Yellow(7-14d), Orange(3-7d), Red(<3d)
- Quick action: "Create Renewal Issue"

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Expiry Scanner | `backend/component/dbpolicy/expiry_scanner.go` | Cross-engine detection |
| Rules Engine | `backend/component/notification/rules_engine.go` | Rule evaluation & dedup |
| Escalation | `backend/component/notification/escalation.go` | Multi-level escalation |
| Auto-Remediation | `backend/component/dbpolicy/auto_remediation.go` | Auto issue creation |
| Scanner Plugins | `backend/plugin/db/*/expiry_query.go` | Engine-specific queries |
| Dashboard Widget | `frontend/src/components/PasswordExpiry/` | Countdown widget |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Oracle user expiry 10 days → notification | Sent |
| TC-002 | PG user expiry 3 days → auto-create Issue | Issue with ALTER ROLE SQL |
| TC-003 | Disabled rule → no notification | Zero notifications |
| TC-004 | Escalation 1 day → L2+L3 notified | Manager + CISO notified |
| TC-005 | Service account UNLIMITED → excluded | No notification |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Expiry scanner + basic notification | Sprint 1-2 |
| Phase 2 | Rules engine + escalation | Sprint 3 |
| Phase 3 | Auto-remediation + dashboard | Sprint 4 |

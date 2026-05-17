# Change Request: Pipeline Credential Sharing Workflow

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-003                                                   |
| **Title**          | Pipeline Credential Sharing Workflow                         |
| **Priority**       | P1 — High                                                    |
| **Status**         | Draft                                                        |
| **PRD Refs**       | DCM-01, SEC-09, SEC-18                                       |
| **Arch Layers**    | L4 (Service), L6 (Runner)                                    |
| **Dependencies**   | CR-SHR-001, CR-SHR-002                                       |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Tích hợp credential sharing vào workflow **Issue → Plan → Rollout** của Bytebase. Khi DBA tạo database user hoặc thay đổi credentials qua pipeline, hệ thống tự động tạo secure share và deliver cho recipients đã chỉ định trong approval workflow.

### 1.1 Tại sao cần

Hiện tại quy trình cấp phát DB/User (DCM-01):

```
1. Developer tạo Issue (yêu cầu cấp DB/User)
2. DBA phê duyệt → thực thi CREATE USER SQL
3. DBA copy password → gửi manual cho Developer (INSECURE!)
```

Target flow:

```
1. Developer tạo Issue (yêu cầu cấp DB/User)
2. DBA phê duyệt → thực thi CREATE USER SQL
3. System tự động detect credential → create secure share
4. System notify Developer với access URL (SECURE!)
5. Credential auto-expires sau 24h
```

---

## 2. Requirements

### 2.1 Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Credential detection in SQL | Detect CREATE USER, ALTER USER, SET PASSWORD statements |
| FR-2 | Auto-share after rollout | TaskRun executor triggers share creation post-migration |
| FR-3 | Recipient from issue | Recipients inferred from Issue creator + project members |
| FR-4 | Manual share trigger | DBA can manually create share from Issue detail page |
| FR-5 | Share link in issue comment | Access URL posted as issue comment (URL only, no credential) |
| FR-6 | Multi-DB support | Detect credential patterns for all 22 DB engines |
| FR-7 | Approval-gated sharing | Share only created after approval complete |

### 2.2 SQL Pattern Detection

| Engine | Pattern | Example |
|---|---|---|
| PostgreSQL | `CREATE ROLE ... PASSWORD '...'` | `CREATE ROLE app_user WITH PASSWORD 'xxx'` |
| MySQL | `CREATE USER ... IDENTIFIED BY '...'` | `CREATE USER 'app'@'%' IDENTIFIED BY 'xxx'` |
| Oracle | `CREATE USER ... IDENTIFIED BY ...` | `CREATE USER app IDENTIFIED BY xxx` |
| SQL Server | `CREATE LOGIN ... WITH PASSWORD = '...'` | `CREATE LOGIN app WITH PASSWORD = 'xxx'` |
| MongoDB | `db.createUser({pwd: '...'})` | `db.createUser({user: "app", pwd: "xxx"})` |

---

## 3. Technical Design

### 3.1 Credential Detector

```go
// backend/component/sharing/detector.go
type CredentialDetector struct {
    patterns map[storepb.Engine][]CredentialPattern
}

type DetectedCredential struct {
    Username       string
    Password       string
    Engine         storepb.Engine
    StatementIndex int
    DatabaseName   string
}

func (d *CredentialDetector) Detect(engine storepb.Engine, statements []string) []DetectedCredential
```

### 3.2 TaskRun Integration

```go
// In database_migrate_executor.go — post-migration hook
func (e *DatabaseMigrateExecutor) RunOnce(...) (*storepb.TaskRunResult, error) {
    // ... existing migration logic ...
    
    // Post-migration: detect and share credentials
    if e.sharingManager != nil {
        credentials := e.credentialDetector.Detect(engine, statements)
        for _, cred := range credentials {
            e.createAutoShare(ctx, task, cred)
        }
    }
    
    return result, nil
}
```

### 3.3 SharingService (gRPC)

```protobuf
// proto/v1/sharing_service.proto
service SharingService {
    rpc CreateShare(CreateShareRequest) returns (CreateShareResponse);
    rpc GetShare(GetShareRequest) returns (Share);
    rpc RevokeShare(RevokeShareRequest) returns (google.protobuf.Empty);
    rpc ListShares(ListSharesRequest) returns (ListSharesResponse);
}

message CreateShareRequest {
    string project = 1;
    int64 issue_uid = 2;
    string name = 3;
    bytes credential_payload = 4;  // Encrypted client-side
    string credential_type = 5;
    int32 max_access_count = 6;
    google.protobuf.Timestamp expires_at = 7;
    repeated int64 recipient_uids = 8;
}
```

### 3.4 Workflow Diagram

```
Issue Created (CREATE USER request)
  │
  ├─► Approval Workflow (SEC-09)
  │     └─ Approved by DBA
  │
  ├─► Plan → Rollout → TaskRun
  │     └─ DatabaseMigrateExecutor
  │         ├─ Execute CREATE USER SQL
  │         ├─ Credential Detector finds password
  │         ├─ SharingManager.CreateShare()
  │         │   └─ Vaultwarden Send API
  │         └─ Post issue comment with access URL
  │
  └─► Notification
        ├─ Webhook → IM (Slack/Teams): "Credentials ready"
        └─ Email notification with access link
```

### 3.5 Issue Comment Format

```
🔐 Credentials shared securely

Database: production-orders
Username: app_service_user
Share URL: https://vault.company.com/#/send/abc123/key/xyz
Expires: 2026-05-18 12:00 UTC (24h)
Max access: 3 times

⚠️ This link will expire automatically. Save credentials in your team's vault.
```

---

## 4. Security Considerations

| Concern | Design Decision |
|---|---|
| Password in SQL statement | Detected and extracted; original SQL NOT stored in logs |
| Share URL in issue comment | URL visible only to issue participants (ACL) |
| Credential in TaskRun result | Masked in TaskRunResult; credential NOT in audit log |
| Auto-share bypass | Only triggers when credential detected AND auto-share enabled |

---

## 5. Configuration

```yaml
# Workspace Setting
sharing:
  auto_share:
    enabled: true
    trigger_on:
      - CREATE_USER
      - ALTER_USER_PASSWORD
    default_ttl: "24h"
    default_max_access: 3
    recipients: "issue_creator"  # or "project_developers", "explicit"
```

---

## 6. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Credential detector (PG, MySQL, Oracle) | Sprint 3 |
| 2 | TaskRun post-migration hook | Sprint 3 |
| 3 | SharingService gRPC + REST gateway | Sprint 4 |
| 4 | Issue comment integration | Sprint 4 |
| 5 | Frontend UI (share button, share list) | Sprint 5 |
| 6 | Remaining engines + tests | Sprint 5 |

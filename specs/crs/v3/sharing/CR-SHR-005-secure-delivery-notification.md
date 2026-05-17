# Change Request: Secure Credential Delivery Notification

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-005                                                   |
| **Title**          | Secure Credential Delivery Notification                      |
| **Priority**       | P2 — Medium                                                  |
| **Status**         | Draft                                                        |
| **PRD Refs**       | ADM-02 (IM Notifications)                                    |
| **Arch Layers**    | L5 (Component)                                               |
| **Dependencies**   | CR-SHR-001, CR-SHR-003                                       |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Mở rộng hệ thống notification (Webhook, Slack, DingTalk, Feishu, Teams) để deliver secure share access URLs cho recipients. Notification chứa link tới shared credential, KHÔNG chứa credential content. Hỗ trợ multi-channel delivery với fallback.

### 1.1 Delivery Channels

| Channel | Method | Security |
|---|---|---|
| Bytebase Issue Comment | In-app | ACL-protected (project members only) |
| Email | SMTP via Mailer plugin | TLS encrypted |
| Slack | Webhook / DM | Workspace-scoped |
| DingTalk | Webhook | Signature verified |
| Feishu/Lark | Webhook | Signature verified |
| Microsoft Teams | Webhook | Connector-based |

---

## 2. Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Multi-channel notification | Deliver share URL via ≥2 channels simultaneously |
| FR-2 | Channel preference per user | User configures preferred notification channel |
| FR-3 | Expiry reminder | Notify recipients 2h before share expiry |
| FR-4 | Delivery confirmation | Track whether notification was delivered |
| FR-5 | Split delivery | URL via Channel A, decryption key via Channel B |
| FR-6 | Retry on failure | Retry notification delivery with exponential backoff |

---

## 3. Technical Design

### 3.1 Notification Template

```go
// backend/component/webhook/sharing_notifier.go

type ShareNotification struct {
    Type           string    // "CREDENTIAL_SHARED", "CREDENTIAL_EXPIRING"
    ShareName      string    // "prod-db-password"
    AccessURL      string    // Vaultwarden Send URL
    ExpiresAt      time.Time
    MaxAccess      int32
    ProjectName    string
    IssueName      string
    IssueURL       string
    SenderName     string
}
```

### 3.2 Split Delivery (Defense-in-Depth)

```
Channel 1 (Slack/Email): "Credentials for prod-db. Access: https://vault.co/send/abc"
Channel 2 (In-app/SMS):  "Decryption key for prod-db share: K3y-X7z-9Mn"
```

Recipient needs both parts to access credential — compromise of single channel insufficient.

### 3.3 Integration Points

- **WebhookManager** (`component/webhook/`) — existing IM dispatch
- **Mailer** (`plugin/mailer/`) — existing email
- **Bus** (`component/bus/`) — new `ShareNotificationChan`
- **Issue Comment** — via IssueService (L4)

---

## 4. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Notification templates, channel config | Sprint 5 |
| 2 | Issue comment integration | Sprint 5 |
| 3 | Split delivery implementation | Sprint 6 |
| 4 | Expiry reminder runner | Sprint 6 |

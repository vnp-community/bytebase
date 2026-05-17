# Change Request: Sharing Audit & Compliance Engine

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-004                                                   |
| **Title**          | Sharing Audit & Compliance Engine                            |
| **Priority**       | P1 — High                                                    |
| **Status**         | Draft                                                        |
| **PRD Refs**       | SEC-10 (Audit Log Full)                                      |
| **Arch Layers**    | L3 (Security), L8 (Store)                                    |
| **Dependencies**   | CR-SHR-001                                                   |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Xây dựng hệ thống audit toàn diện cho tất cả hoạt động chia sẻ thông tin nhạy cảm. Đảm bảo traceability và compliance (PCI-DSS, ISO27001) bằng cách ghi log mọi hành động: create, access, revoke, expire. Tích hợp vào AuditInterceptor hiện tại (L3).

### 1.1 Compliance Requirements

| Standard | Requirement | Implementation |
|---|---|---|
| PCI-DSS 8.2.6 | Track credential distribution | Audit log mọi share creation |
| PCI-DSS 10.2 | Monitor access to sensitive data | Log mọi share access event |
| ISO27001 A.9.4 | Credential lifecycle management | Track create → access → expire/revoke |
| SOX | Segregation of duties | DBA creates, Developer receives — different actors |

---

## 2. Requirements

### 2.1 Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Log share creation | Who, what, when, for whom, TTL, issue reference |
| FR-2 | Log share access | Who accessed, when, from which IP, remaining accesses |
| FR-3 | Log share revocation | Who revoked, when, reason |
| FR-4 | Log share expiration | Auto-expire events logged |
| FR-5 | Compliance dashboard | Filter shares by project, time, status, user |
| FR-6 | Export audit report | CSV/PDF export for compliance audits |
| FR-7 | Alert on anomalies | Alert when: > N shares/day, share accessed from unusual IP |

### 2.2 Audit Event Types

| Event | Level | Data Logged |
|---|---|---|
| `SHARE_CREATED` | INFO | creator, credential_type, recipients, TTL, issue_uid |
| `SHARE_ACCESSED` | INFO | accessor_uid, share_id, access_count, source_ip |
| `SHARE_REVOKED` | WARN | revoker_uid, share_id, reason |
| `SHARE_EXPIRED` | INFO | share_id, final_access_count |
| `SHARE_FAILED` | ERROR | creator_uid, error, provider |
| `SHARE_POLICY_VIOLATION` | WARN | violator_uid, violation_type |

---

## 3. Technical Design

### 3.1 Audit Log Extension

```go
// Extend existing AuditInterceptor (backend/api/v1/audit.go)
// Add sharing-specific audit methods

type SharingAuditEntry struct {
    BaseAuditLog                      // Existing audit fields
    ShareID         string            // Provider share ID
    CredentialType  string            // Type of shared credential
    RecipientUIDs   []int64           // Who can access
    TTL             time.Duration     // Configured TTL
    MaxAccessCount  int32             // Configured max accesses
    ProjectID       string            // Associated project
    IssueUID        int64             // Associated issue
    ProviderType    string            // Sharing provider
}
```

### 3.2 Store Schema

```sql
-- Extend audit_log or create dedicated table
CREATE TABLE sharing_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    event_type      TEXT NOT NULL,        -- SHARE_CREATED, SHARE_ACCESSED, etc.
    share_id        TEXT NOT NULL,
    project         TEXT,
    issue_uid       BIGINT,
    actor_uid       BIGINT NOT NULL,
    actor_ip        INET,
    credential_type TEXT,
    recipient_uids  JSONB,
    details         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sharing_audit_project ON sharing_audit_log(project, created_at DESC);
CREATE INDEX idx_sharing_audit_actor ON sharing_audit_log(actor_uid, created_at DESC);
CREATE INDEX idx_sharing_audit_share ON sharing_audit_log(share_id);
```

### 3.3 Compliance Report Query

```sql
-- PCI-DSS Credential Distribution Report
SELECT
    sa.created_at,
    sa.event_type,
    p.email AS actor,
    sa.credential_type,
    sa.project,
    sa.details->>'reason' AS reason
FROM sharing_audit_log sa
JOIN principal p ON p.id = sa.actor_uid
WHERE sa.workspace_id = $1
  AND sa.created_at BETWEEN $2 AND $3
ORDER BY sa.created_at DESC;
```

### 3.4 Anomaly Detection Runner

```go
// backend/runner/sharing/anomaly_detector.go
// Periodic runner (every 15 min) checks for anomalies

type AnomalyRule struct {
    MaxSharesPerDay     int  // Alert if > N shares created by one user
    MaxAccessesPerHour  int  // Alert if > N accesses to one share
    UnusualIPAccess     bool // Alert if access from new IP
}
```

---

## 4. Security

| Concern | Decision |
|---|---|
| Credential content in audit | NEVER logged — only metadata |
| PII in audit | Recipient UIDs only, resolved at display time |
| Audit log tampering | Append-only table, PG advisory lock |
| Long-term retention | Configurable retention (default 3 years for PCI-DSS) |

---

## 5. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Audit event types, store schema | Sprint 3 |
| 2 | Integration with SharingManager | Sprint 3 |
| 3 | AuditInterceptor extension | Sprint 4 |
| 4 | Compliance report API + frontend | Sprint 5 |
| 5 | Anomaly detection runner | Sprint 6 |

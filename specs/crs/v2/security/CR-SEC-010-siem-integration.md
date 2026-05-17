# Change Request: Security Event Monitoring & Alerting (SIEM)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-010                                               |
| **Feature ID**     | SEC-10                                                   |
| **Title**          | Security Event Monitoring & Alerting (SIEM Integration)  |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống Security Event Monitoring tích hợp: real-time security event detection, anomaly detection, SIEM integration (Splunk, ELK, Datadog), automated alerting, và security dashboard cho SOC team.

### 1.2 Bối cảnh
Bytebase Audit Log (SEC-07/SEC-10, CR-ENT-003) ghi nhận activities nhưng thiếu: real-time threat detection, anomaly detection, SIEM forwarding, và security-specific alerting.

---

## 2. Yêu cầu chức năng

### FR-001: Security Event Classification
- **Event Categories**:

| Category                  | Events                                                    | Severity    |
|---------------------------|-----------------------------------------------------------|-------------|
| Authentication            | Failed login, brute-force, impossible travel               | Critical    |
| Authorization             | Permission denied, privilege escalation attempt            | High        |
| Data Access               | Sensitive data query, bulk export, masking bypass          | High        |
| Configuration             | Policy change, role change, SSO config change             | Medium      |
| Schema Changes            | DDL on production, schema drift, unauthorized change      | Critical    |
| System                    | Service restart, certificate expiry, key rotation         | Medium      |

- **Acceptance Criteria**:
  - AC-1: Event taxonomy covering all security-relevant actions
  - AC-2: Severity levels: Critical, High, Medium, Low, Informational
  - AC-3: Events enriched with context (user, IP, timestamp, resource)
  - AC-4: Events in structured format (CEF, JSON, OCSF)

### FR-002: Real-Time Anomaly Detection
- **Detection Rules**:
  - Impossible travel: login from geographically impossible locations
  - Access pattern anomaly: unusual query patterns, bulk data access
  - Off-hours activity: sensitive operations outside business hours
  - Role change spike: multiple role changes in short period
- **Acceptance Criteria**:
  - AC-1: Rule-based detection engine with configurable thresholds
  - AC-2: ML-based baseline learning (optional, future phase)
  - AC-3: Alert suppression for known patterns (reduce noise)
  - AC-4: Detection latency < 30 seconds

### FR-003: SIEM Integration
- **Supported Targets**:
  - Syslog (RFC 5424)
  - Splunk HEC (HTTP Event Collector)
  - Elasticsearch / OpenSearch
  - Datadog Log Management
  - AWS CloudWatch Logs
  - Custom webhook
- **Acceptance Criteria**:
  - AC-1: Multiple SIEM targets simultaneously
  - AC-2: Configurable event filtering per target
  - AC-3: Guaranteed delivery (at-least-once) with retry
  - AC-4: TLS encryption for log transport
  - AC-5: Back-pressure handling when SIEM is unavailable

### FR-004: Security Dashboard
- **Acceptance Criteria**:
  - AC-1: Real-time security event feed
  - AC-2: Top threats summary (last 24h/7d/30d)
  - AC-3: User risk scoring based on activities
  - AC-4: Geographical login map
  - AC-5: Alert statistics and resolution tracking

### FR-005: Automated Response Actions
- **Acceptance Criteria**:
  - AC-1: Auto-lock user on critical security event
  - AC-2: Auto-revoke sessions on impossible travel
  - AC-3: Auto-block IP on brute-force detection
  - AC-4: Configurable playbooks per event type
  - AC-5: Manual override by admin

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Security Event Engine (new)  | `backend/component/security_event/`         | Event classification, anomaly detection     |
| SIEM Forwarder (new)         | `backend/component/siem/`                   | Multi-target log forwarding                 |
| Audit Interceptor            | `backend/api/v1/audit.go`                   | Security event enrichment                   |
| Security Dashboard           | `frontend/src/views/SecurityDashboard.vue`  | Real-time security overview                 |
| Alert Service                | `backend/runner/security_alert/`            | Automated response actions                  |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Impossible travel detected                           | Alert generated, session revoked |
| TC-002  | Bulk data export by non-admin                        | High severity event logged       |
| TC-003  | SIEM integration with Splunk                         | Events received in Splunk        |
| TC-004  | Brute-force detected                                 | IP auto-blocked, admin notified  |
| TC-005  | Security dashboard loads                             | Real-time feed populated         |
| TC-006  | SIEM target unavailable                              | Events queued, retry on recovery |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Event classification + enrichment    | Sprint 1       |
| Phase 2 | SIEM integration (syslog + webhook)  | Sprint 2       |
| Phase 3 | Anomaly detection rules              | Sprint 3       |
| Phase 4 | Security dashboard + auto-response   | Sprint 4       |

# Change Request: Extended Audit Trail & SIEM Integration for Shared Secrets

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SHR-105                                               |
| **Title**          | Extended Audit Trail & SIEM Integration for Shared Secrets |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Extends**        | CR-SHR-004 (Sharing Audit & Compliance)                  |

---

## 1. Mô tả

Mở rộng CR-SHR-004 (audit cơ bản) thành hệ thống **Enterprise Audit & Compliance** với:
- **Anomaly detection engine** — Rule-based + behavioral baseline
- **SIEM integration** — Syslog, Webhook, Kafka, Elasticsearch, Splunk
- **Compliance reporting** — SOC 2, ISO 27001, PCI DSS templates
- **HMAC event chain** — Tamper-proof audit log
- **Data retention & archival** — Configurable policies

---

## 2. Requirements

### 2.1 Anomaly Detection Rules

| Rule ID | Pattern | Risk | Action |
|---|---|---|---|
| ANM-001 | >5 share links in 10 min | HIGH | Alert admin |
| ANM-002 | Access from >3 countries in 1h | CRITICAL | Auto-revoke |
| ANM-003 | >10 failed accesses in 5 min | HIGH | Lock link |
| ANM-004 | Distribute to unregistered target | CRITICAL | Block |
| ANM-005 | Bulk credential export (>10) | HIGH | Require approval |
| ANM-006 | Off-hours credential access | MEDIUM | Alert |

### 2.2 SIEM Exporters

| Exporter | Protocol | Use Case |
|---|---|---|
| Syslog | RFC 5424 | Traditional SIEM |
| Webhook | HTTP POST | Custom integrations |
| Kafka | Producer API | Event streaming |
| Elasticsearch | Bulk API | Log aggregation |
| Splunk | HEC | Enterprise SIEM |

### 2.3 Compliance Reports

| Report | Standard | Content |
|---|---|---|
| Secret Access Audit | SOC 2 CC6.1 | Who accessed what, when, from where |
| Key Lifecycle | SOC 2 CC6.6 | Key rotation history |
| Share Activity | ISO 27001 A.9.4.1 | All sharing events |
| Credential Distribution | PCI DSS 8.2 | Rotation evidence |

---

## 3. Technical Design

### 3.1 HMAC Event Chain

```
Event N: hash_N = HMAC-SHA256(key, event_data + hash_{N-1})
Tamper detection: re-compute chain → mismatch = tampering
```

### 3.2 Schema

```sql
CREATE TABLE sharing_audit_event (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_ip INET,
    action TEXT NOT NULL,
    category TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    details JSONB NOT NULL,
    result TEXT NOT NULL,
    risk_level TEXT DEFAULT 'low',
    compliance_tags TEXT[],
    previous_event_hash TEXT,
    event_hash TEXT NOT NULL
);

CREATE TABLE anomaly_alert (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id TEXT NOT NULL,
    risk_level TEXT NOT NULL,
    description TEXT NOT NULL,
    related_events TEXT[],
    status TEXT DEFAULT 'open',
    resolved_at TIMESTAMPTZ,
    resolution_note TEXT
);

CREATE TABLE siem_config (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    exporter_type TEXT NOT NULL,
    config JSONB NOT NULL,
    event_filter JSONB,
    enabled BOOLEAN DEFAULT TRUE
);
```

### 3.3 Components

| Component | File/Package |
|---|---|
| Audit Emitter | `backend/component/audit/sharing_emitter.go` |
| Anomaly Detector | `backend/component/audit/anomaly/detector.go` |
| Compliance Reporter | `backend/component/audit/compliance/reporter.go` |
| SIEM Exporter | `backend/component/audit/siem/exporter.go` |
| HMAC Chain | `backend/component/audit/integrity.go` |
| Retention Manager | `backend/runner/audit/retention.go` |

### 3.4 Event Flow

```
Sharing Operation
  → Emit structured audit event (HMAC chain)
  → Anomaly Detector evaluates (rules + baseline)
    → [If anomaly] Create alert + auto-action
  → SIEM Exporter forwards (filtered)
```

---

## 4. Security

| Concern | Mitigation |
|---|---|
| Audit tampering | HMAC chain + append-only storage |
| PII in audit | Configurable masking for exports |
| Storage exhaustion | Retention policies + auto-archival |
| SIEM creds | Encrypted with BEE envelope |

---

## 5. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Create share → audit event | Event with correct action/risk |
| TC-002 | >5 shares in 10 min → anomaly | ANM-001 triggered |
| TC-003 | Multi-country access → revoke | ANM-002, link revoked |
| TC-004 | Generate SOC2 report | PDF with findings |
| TC-005 | Tamper audit event → detect | HMAC chain broken |
| TC-006 | SIEM webhook delivery | Target receives events |

---

## 6. Rollout Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Extended audit emitter + HMAC chain | Sprint 7-8 |
| 2 | Anomaly detection rules | Sprint 8-9 |
| 3 | Compliance reporting | Sprint 9-10 |
| 4 | SIEM integration | Sprint 10-11 |
| 5 | Dashboard UI + retention | Sprint 11-12 |

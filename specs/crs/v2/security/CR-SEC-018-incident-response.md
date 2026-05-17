# Change Request: Security Incident Response Automation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-018                                               |
| **Feature ID**     | SEC-10                                                   |
| **Title**          | Security Incident Response Automation                    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai automated security incident response framework: incident detection → classification → containment → notification → remediation → post-mortem. Bao gồm playbook automation, escalation chains, và forensic data preservation.

### 1.2 Bối cảnh
Bytebase Audit Log (SEC-10) và SIEM Integration (CR-SEC-010) phát hiện threats. CR này xây dựng response automation layer để giảm MTTR (Mean Time To Respond) từ hours xuống minutes.

---

## 2. Yêu cầu chức năng

### FR-001: Incident Classification
- **Severity Levels**:

| Severity  | Criteria                                           | Response SLA |
|-----------|---------------------------------------------------|-------------|
| P0 - SEV1 | Active data breach, system compromise              | 15 minutes  |
| P1 - SEV2 | Credential leak, privilege escalation              | 1 hour      |
| P2 - SEV3 | Suspicious activity, policy violation              | 4 hours     |
| P3 - SEV4 | Informational, false positive review               | 24 hours    |

- **Acceptance Criteria**:
  - AC-1: Auto-classification based on event type and context
  - AC-2: Manual severity override by security admin
  - AC-3: Severity escalation on SLA breach
  - AC-4: Incident timeline auto-generated

### FR-002: Automated Playbooks
- **Pre-built Playbooks**:

| Playbook                  | Trigger                           | Actions                                     |
|---------------------------|-----------------------------------|---------------------------------------------|
| Account Compromise        | Impossible travel + data access   | Lock account, revoke sessions, notify admin |
| Credential Leak           | API key in public repo            | Rotate credential, block old key, notify    |
| Brute Force               | >50 failed logins in 5 min       | Block IP, lock account, notify SOC          |
| Data Exfiltration         | Bulk export from production       | Block export, freeze account, preserve logs |
| Unauthorized Schema Change| DDL on prod without approval      | Rollback if possible, freeze pipeline       |

- **Acceptance Criteria**:
  - AC-1: Playbooks execute automatically on trigger
  - AC-2: Each playbook action logged for audit
  - AC-3: Human-in-the-loop option (require confirmation for destructive actions)
  - AC-4: Custom playbook creation by security admin
  - AC-5: Playbook dry-run mode for testing

### FR-003: Escalation Chain
- **Acceptance Criteria**:
  - AC-1: Multi-level escalation: on-call → team lead → CISO
  - AC-2: Escalation channels: email, SMS, PagerDuty, Slack
  - AC-3: Auto-escalate on SLA breach
  - AC-4: On-call rotation schedule integration
  - AC-5: Acknowledgment tracking per escalation level

### FR-004: Forensic Data Preservation
- **Acceptance Criteria**:
  - AC-1: Auto-snapshot of relevant logs on incident trigger
  - AC-2: Preserve session data, query history, access patterns
  - AC-3: Evidence package exportable for investigation
  - AC-4: Tamper-proof evidence storage (link CR-SEC-011)
  - AC-5: Chain of custody tracking for evidence

### FR-005: Post-Incident Review
- **Acceptance Criteria**:
  - AC-1: Incident timeline auto-generated
  - AC-2: Root cause analysis template
  - AC-3: Remediation action tracking
  - AC-4: Lessons learned documentation
  - AC-5: Recurring incident pattern detection

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Incident Engine (new)        | `backend/component/incident/`               | Classification, playbook execution          |
| Playbook Runner (new)        | `backend/runner/incident_playbook/`         | Automated response actions                  |
| Escalation Service (new)     | `backend/component/escalation/`             | Multi-channel escalation                    |
| Forensic Store (new)         | `backend/store/incident_evidence.go`        | Evidence preservation                       |
| Incident Dashboard           | `frontend/src/views/IncidentManager.vue`    | Incident management UI                      |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Account compromise playbook triggers                 | Account locked, sessions revoked |
| TC-002  | SLA breach on P0 incident                            | Auto-escalation triggered        |
| TC-003  | Forensic evidence preserved                          | Evidence package exportable      |
| TC-004  | Custom playbook dry-run                              | Actions simulated, not executed  |
| TC-005  | Escalation acknowledgment                            | Timer stops, status updated      |
| TC-006  | Post-incident report generated                       | Timeline + root cause template   |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Incident classification + timeline   | Sprint 1       |
| Phase 2 | Pre-built playbooks                  | Sprint 2       |
| Phase 3 | Escalation chain                     | Sprint 3       |
| Phase 4 | Forensic preservation + review       | Sprint 4       |

# Change Request: Compliance Dashboard & Reporting Engine

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-008                                               |
| **Gap ID**         | G8                                                       |
| **Title**          | Compliance Dashboard & Reporting Engine                  |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Depends On**     | CR-INS-001, CR-INS-003, CR-INS-004, CR-INS-007          |

---

## 1. Tổng quan

### 1.1 Mô tả
Dashboard tổng hợp compliance status theo PCI-DSS và ISO27001 cho toàn bộ hệ thống database. Tập hợp dữ liệu từ tất cả CR-INS modules (policy compliance, session monitoring, access review, credential scan) vào một unified view.

### 1.2 Bối cảnh
Gap G8 đề xuất Grafana custom panels. Xây dựng compliance dashboard trực tiếp trong Bytebase đảm bảo single pane of glass cho DBA và compliance team, không cần maintain dashboard bên ngoài.

### 1.3 Mục tiêu
- Unified compliance view cho PCI-DSS & ISO27001
- Automated compliance scoring
- Scheduled compliance reports (PDF/CSV)
- Audit-ready evidence collection

---

## 2. Yêu cầu chức năng

### FR-001: Compliance Score Engine
Tính compliance score composite từ multiple signals:

| Domain | Weight | Source | Metrics |
|---|---|---|---|
| Password Policy | 30% | CR-INS-001 | % users with valid profile, % compliant passwords |
| Access Control | 25% | CR-INS-004 | % users matching permission matrix, orphaned count |
| Session Security | 15% | CR-INS-003 | Old user session count, unauthorized access attempts |
| Credential Safety | 15% | CR-INS-007 | Hardcoded credentials found, remediation rate |
| Change Management | 15% | Bytebase native | Changes with approval %, SQL review pass rate |

- Overall score: 0-100 per instance, per project, overall
- Score trend over time (weekly/monthly)

### FR-002: PCI-DSS Compliance Mapping
Map Bytebase metrics → PCI-DSS requirements:

| PCI-DSS Req | Description | Bytebase Evidence |
|---|---|---|
| 2.1 | Change vendor defaults | Password policy compliance |
| 7.1 | Limit access by business need | Access review results |
| 7.2 | Access control system | Approval workflow audit |
| 8.1 | Unique user IDs | DB user inventory |
| 8.2 | Authentication mechanisms | Password complexity check |
| 8.5 | No shared credentials | Credential scan results |
| 10.1 | Audit trails | Bytebase audit log |
| 10.2 | Automated audit trails | Change pipeline logs |

### FR-003: ISO27001 Controls Mapping
Map metrics → ISO27001 Annex A controls (A.9 Access Control, A.12 Operations Security).

### FR-004: Compliance Report Generator
- **Scheduled reports**: Weekly/Monthly/Quarterly auto-generation
- **Formats**: PDF (audit-ready), CSV (data analysis), JSON (API)
- **Report sections**:
  - Executive summary with overall score
  - Per-domain breakdown
  - Violation details with remediation status
  - Evidence references (audit log entries)
  - Trend analysis
- **Distribution**: Email to compliance team, archive in Bytebase

### FR-005: Dashboard UI
- **Executive View**: Overall score donut chart, trend line, top violations
- **Domain Deep-dive**: Drill-down per compliance domain
- **Instance Map**: Heatmap of compliance scores across instances
- **Timeline**: Compliance events correlated with changes
- **Filters**: By engine, environment, project, date range

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Score Engine | `backend/component/compliance/score_engine.go` | Composite scoring |
| PCI Mapper | `backend/component/compliance/pci_mapper.go` | PCI-DSS mapping |
| ISO Mapper | `backend/component/compliance/iso_mapper.go` | ISO27001 mapping |
| Report Generator | `backend/component/compliance/reporter.go` | PDF/CSV/JSON |
| Compliance API | `backend/api/v1/compliance_service.go` | API endpoints |
| Dashboard UI | `frontend/src/views/Compliance/` | Charts & reports |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | All instances compliant → score 100 | Green overall |
| TC-002 | Policy violation → score drops | Score reflects violation |
| TC-003 | Monthly report generated | PDF with all sections |
| TC-004 | PCI-DSS mapping renders correctly | All 8 requirements mapped |
| TC-005 | Trend shows improvement over 4 weeks | Upward score trend |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Score engine + basic dashboard | Sprint 1-2 |
| Phase 2 | PCI/ISO mapping + report generator | Sprint 3-4 |
| Phase 3 | Scheduled reports + distribution | Sprint 5 |

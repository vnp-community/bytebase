# Change Request: Risk Assessment

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-006                                               |
| **Feature ID**     | SEC-08                                                   |
| **Title**          | Risk Assessment for Database Changes                     |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Tự động đánh giá mức độ rủi ro cho mỗi database change request dựa trên nhiều yếu tố: loại SQL statement, environment tier, affected rows estimate, table importance, và thời gian thực hiện. Risk level quyết định approval workflow cần thiết.

### 1.2 Mục tiêu
- Tự động phân loại risk level cho mỗi change (LOW / MODERATE / HIGH / CRITICAL)
- Risk level drive approval workflow (CR-ENT-007)
- Cung cấp risk breakdown chi tiết cho reviewers
- Cho phép custom risk rules theo organization needs

---

## 2. Yêu cầu chức năng

### FR-001: Risk Level Classification
- **Mô tả**: Hệ thống tự động phân loại risk cho mỗi database change.
- **Risk Levels**:

| Level        | Score Range | Mô tả                                         | Default Action        |
|--------------|-------------|------------------------------------------------|-----------------------|
| **LOW**      | 0-25        | Routine changes, read-only, dev environment    | Auto-approve          |
| **MODERATE** | 26-50       | Schema changes on staging, data updates         | Single approval       |
| **HIGH**     | 51-75       | Production schema changes, large data updates   | Multi-level approval  |
| **CRITICAL** | 76-100      | Production DDL, data deletion, privilege changes| DBA + Manager approval|

### FR-002: Risk Factors & Scoring
- **Mô tả**: Hệ thống tính risk score dựa trên weighted factors.
- **Default Risk Factors**:

| Factor                     | Weight | Scoring Logic                                          |
|---------------------------|--------|--------------------------------------------------------|
| Environment Tier          | 30%    | DEV=0, STAGING=15, PROD=30                             |
| Statement Type            | 25%    | SELECT=0, INSERT=10, UPDATE=15, ALTER=20, DROP=25      |
| Estimated Affected Rows   | 20%    | <100=0, <1K=5, <10K=10, <100K=15, ≥100K=20            |
| Table Classification      | 15%    | INTERNAL=0, STANDARD=5, SENSITIVE=10, CRITICAL=15      |
| Off-Hours Execution       | 10%    | Business hours=0, Off-hours=5, Weekend=10              |

- **Acceptance Criteria**:
  - AC-1: Risk score tính chính xác dựa trên weighted factors
  - AC-2: Admin có thể customize weights và scoring thresholds
  - AC-3: Risk assessment hiển thị breakdown chi tiết
  - AC-4: Risk score tính trước khi issue transition sang review

### FR-003: Custom Risk Rules
- **Mô tả**: Admin có thể define custom risk rules dùng CEL expressions.
- **Ví dụ**:
  ```cel
  // Rule: DROP TABLE trên production luôn CRITICAL
  statement.type == "DROP" && environment.tier == "PRODUCTION"
    ? risk.CRITICAL
    : risk.DEFAULT

  // Rule: Changes trên tables có PII data luôn ≥ HIGH
  table.classification == "PII"
    ? max(risk.calculated, risk.HIGH)
    : risk.calculated
  ```
- **Acceptance Criteria**:
  - AC-1: Custom rules override default scoring
  - AC-2: Rules validated trước khi save (CEL syntax check)
  - AC-3: Rule evaluation order configurable (priority-based)

### FR-004: Risk Assessment UI
- **Mô tả**: Hiển thị risk assessment trên Issue/Plan detail page.
- **UI Elements**:
  - Risk level badge (color-coded: green/yellow/orange/red)
  - Risk score number
  - Factor breakdown (collapsible)
  - Risk history (nếu re-assessed)
  - Override option cho workspace admin
- **Acceptance Criteria**:
  - AC-1: Risk badge visible trên issue list và detail
  - AC-2: Breakdown expandable để xem chi tiết từng factor
  - AC-3: Admin có thể override risk level với reason

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                             | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| Risk Assessment Engine       | `backend/component/risk/`                 | New component: risk scoring engine                 |
| Plan Check Executor          | `backend/runner/plancheck/`               | Integrate risk assessment into plan checks         |
| Issue Service                | `backend/api/v1/issue_service.go`         | Expose risk info in issue response                 |
| Risk Policy Service          | `backend/api/v1/risk_policy_service.go`   | CRUD risk rules                                    |
| Feature Gate                 | `enterprise/feature.go`                   | Define `FeatureRiskAssessment`                     |

### 3.2 Database Changes

```sql
CREATE TABLE risk_policy (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    name TEXT NOT NULL,
    expression TEXT NOT NULL,  -- CEL expression
    risk_level TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Risk assessment results stored in issue/plan payload (JSONB)
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | SELECT trên DEV environment                             | Risk: LOW (score ~0)                  |
| TC-002     | ALTER TABLE trên PRODUCTION                             | Risk: HIGH (score ~70)                |
| TC-003     | DROP TABLE trên PRODUCTION                              | Risk: CRITICAL (score ~85)            |
| TC-004     | Custom rule: PII table → HIGH                           | Override applied correctly            |
| TC-005     | Admin override risk level                               | Override saved with reason            |
| TC-006     | Non-ENTERPRISE plan                                     | Feature hidden, no risk assessment    |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Risk scoring engine                  | Sprint 1       |
| Phase 2 | PlanCheck integration                | Sprint 1       |
| Phase 3 | Risk assessment UI                   | Sprint 2       |
| Phase 4 | Custom risk rules (CEL)              | Sprint 3       |
| Phase 5 | Approval workflow integration        | Sprint 3       |

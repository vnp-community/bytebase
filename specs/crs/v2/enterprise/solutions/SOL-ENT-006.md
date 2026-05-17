# Solution: CR-ENT-006 — Risk Assessment

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-006                |
| **Solution**   | SOL-ENT-006               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Xây dựng **Risk Assessment Engine** (L5 Component) tính weighted risk score cho database changes. Tích hợp vào PlanCheck pipeline (L6), kết quả drive Approval Workflow (CR-ENT-007). Hỗ trợ custom CEL rules.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/risk/` (NEW) | Risk scoring engine |
| **L6 — Runner** | `runner/plancheck/` | Risk assessment executor |
| **L7 — Plugin** | SQL Parser | Statement type analysis |
| **L8 — Store** | `store/risk_policy.go` (NEW) | Custom rules persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureRiskAssessment` gate |

---

## 3. Chi tiết Implementation

### 3.1 Risk Scoring Engine

```go
// component/risk/engine.go
func (e *RiskEngine) Assess(ctx context.Context, input *RiskInput) (*RiskResult, error) {
    factors := []RiskFactor{
        e.assessEnvironmentTier(input.Environment),    // 30%
        e.assessStatementType(input.StatementType),    // 25%
        e.assessAffectedRows(input.EstimatedRows),     // 20%
        e.assessTableClassification(input.Tables),     // 15%
        e.assessExecutionTime(input.ScheduledAt),      // 10%
    }
    totalScore := sumWeighted(factors)
    totalScore = e.applyCustomRules(ctx, input, totalScore)
    return &RiskResult{Level: classifyRisk(totalScore), TotalScore: totalScore, Factors: factors}, nil
}
```

### 3.2 Classification: 0-25=LOW, 26-50=MODERATE, 51-75=HIGH, 76-100=CRITICAL

### 3.3 Schema Migration

```sql
CREATE TABLE risk_policy (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    name TEXT NOT NULL,
    expression TEXT NOT NULL,  -- CEL
    risk_level TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.4 PlanCheck Integration

Register `RiskAssessmentExecutor` in PlanCheck scheduler. Results stored in `plan_check_run.result` JSONB.

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-007 | Risk level drives approval template matching |
| CR-ENT-013 | Table classification feeds risk scoring |
| CR-ENT-019 | Environment tier = 30% weight factor |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Risk scoring engine | Sprint 1 |
| 2 | PlanCheck integration | Sprint 1 |
| 3 | Risk UI (badge + breakdown) | Sprint 2 |
| 4 | Custom CEL rules | Sprint 3 |
| 5 | Approval workflow integration | Sprint 3 |

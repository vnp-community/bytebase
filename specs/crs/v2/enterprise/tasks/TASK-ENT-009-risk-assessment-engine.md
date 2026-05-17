# TASK-ENT-009 — Risk Assessment Engine

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-009                               |
| **Source**       | SOL-ENT-006 (CR-ENT-006)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Xây dựng Risk Assessment Engine (L5 Component) tính weighted risk score cho database changes, tích hợp vào PlanCheck pipeline.

## Scope

### Phase 1 — Sprint 1: Scoring Engine + PlanCheck
1. **L5 — Risk Engine (NEW)**: `component/risk/engine.go`
   - Environment Tier (30%) — CR-ENT-019
   - Statement Type (25%) — via SQL Parser (L7)
   - Affected Rows (20%) — estimated row count
   - Table Classification (15%) — CR-ENT-013
   - Execution Time (10%) — scheduled time analysis
2. **Classification**: 0-25=LOW, 26-50=MODERATE, 51-75=HIGH, 76-100=CRITICAL
3. **L6 — PlanCheck Integration**: Register `RiskAssessmentExecutor`, results in `plan_check_run.result` JSONB
4. **L9 — Feature Gate**: `FeatureRiskAssessment`

### Phase 2 — Sprint 2: Risk UI
5. **L1 — Frontend**: Risk badge + factor breakdown display on issues/plans

### Phase 3 — Sprint 3: Custom Rules + Approval Integration
6. **Schema Migration**: `risk_policy` table — custom CEL rules
7. **Custom CEL Rules**: `applyCustomRules()` — user-defined risk modifiers
8. **Approval Integration**: Risk level drives approval template matching (CR-ENT-007)

## Acceptance Criteria

- [x] Risk scoring engine produces accurate weighted scores
- [x] All 5 scoring factors implemented
- [x] Classification levels correct (LOW/MODERATE/HIGH/CRITICAL)
- [x] PlanCheck integration functional
- [x] Risk results stored in `plan_check_run.result`
- [x] Custom CEL rules evaluate correctly
- [x] Risk UI shows badge + breakdown
- [x] Risk level feeds approval workflow routing

## Dependencies

- CR-ENT-007 (Approval Workflow) — risk level drives template matching
- CR-ENT-013 (Data Classification) — table classification feeds scoring
- CR-ENT-019 (Environment Tiers) — tier = 30% weight

## Definition of Done

- [x] Engine accuracy validated with test scenarios
- [x] PlanCheck pipeline tested end-to-end
- [x] Custom rules framework functional

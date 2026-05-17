# Solution: CR-ENT-019 — Environment Tiers

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-019                |
| **Solution**   | SOL-ENT-019               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Thêm `tier` classification vào bảng `environment` hiện có. Production tier auto-applies stricter policies: mandatory approval, enhanced audit, enforced masking, restricted copy. Tier classification drives Risk Assessment (CR-ENT-006) scoring (30% weight).

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `environment_service.go` | Tier classification CRUD |
| **L4 — Service** | `org_policy_service.go` | Tier-based policy resolution |
| **L6 — Runner** | `runner/approval/` | Tier-based approval routing |
| **L5 — Component** | `component/risk/` | Tier feeds risk scoring |
| **L8 — Store** | `environment` table | `tier` column |
| **L9 — Enterprise** | `feature.go` | `FeatureEnvironmentTiers` gate |

---

## 3. Chi tiết Implementation

### 3.1 Schema Migration

```sql
ALTER TABLE environment ADD COLUMN tier TEXT NOT NULL DEFAULT 'NON_PRODUCTION'
    CHECK (tier IN ('PRODUCTION', 'NON_PRODUCTION'));
```

### 3.2 Tier-Based Auto-Policies

When environment is classified as `PRODUCTION`, system auto-enforces:

| Policy | PRODUCTION | NON_PRODUCTION |
|--------|-----------|----------------|
| Approval workflow | Mandatory (no auto-approve) | Optional |
| Audit logging | All SQL queries logged | Standard only |
| Data masking | Enforced | Optional |
| Copy restriction | RESTRICT default | ALLOW default |
| Access mode | Read-only default | Read-write |

### 3.3 Deployment Gates

```go
func (s *RolloutService) validateDeploymentOrder(ctx context.Context, plan *store.PlanMessage) error {
    stages := plan.Stages
    for i, stage := range stages {
        env, _ := s.store.GetEnvironment(ctx, stage.EnvironmentID)
        if env.Tier == "PRODUCTION" && i > 0 {
            // Check all previous non-prod stages completed
            for _, prevStage := range stages[:i] {
                if prevStage.Status != "DONE" {
                    return status.Errorf(codes.FailedPrecondition,
                        "must complete non-production stages before production deployment")
                }
            }
        }
    }
    return nil
}
```

### 3.4 UI Indicators

- Production environments: Red badge/border
- Warning dialog when deploying to production tier
- Environment cards show tier classification

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-006 | Environment tier = 30% risk score weight |
| CR-ENT-007 | Production tier triggers mandatory approval |
| CR-ENT-012 | Production tier enforces data masking |
| CR-ENT-005 | Production tier enforces copy restriction |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Tier classification + migration | Sprint 1 |
| 2 | Tier-based policy enforcement | Sprint 1 |
| 3 | Deployment gates | Sprint 2 |
| 4 | UI indicators + warnings | Sprint 2 |

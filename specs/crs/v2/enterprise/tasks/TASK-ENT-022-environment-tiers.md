# TASK-ENT-022 — Environment Tiers & Deployment Gates

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-022                               |
| **Source**       | SOL-ENT-019 (CR-ENT-019)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Thêm `tier` classification vào environments. Production tier auto-applies stricter policies: mandatory approval, enhanced audit, enforced masking, restricted copy.

## Scope

### Phase 1 — Sprint 1: Tier Classification + Policy Enforcement
1. **Schema Migration**: `ALTER TABLE environment ADD COLUMN tier TEXT` — `PRODUCTION` | `NON_PRODUCTION`
2. **L4 — EnvironmentService**: Tier classification CRUD
3. **Tier-Based Auto-Policies**:
   | Policy | PRODUCTION | NON_PRODUCTION |
   |--------|-----------|----------------|
   | Approval workflow | Mandatory | Optional |
   | Audit logging | All SQL queries | Standard only |
   | Data masking | Enforced | Optional |
   | Copy restriction | RESTRICT default | ALLOW default |
   | Access mode | Read-only default | Read-write |
4. **L4 — OrgPolicyService**: Tier-based policy resolution
5. **L9 — Feature Gate**: `FeatureEnvironmentTiers`

### Phase 2 — Sprint 2: Deployment Gates + UI
6. **Deployment Gates**: `validateDeploymentOrder()` — must complete non-prod stages before production
7. **L1 — Frontend**: 
   - Production environments: Red badge/border
   - Warning dialog when deploying to production tier
   - Environment cards show tier classification

## Acceptance Criteria

- [x] `tier` column added to environment table
- [x] Tier classification CRUD functional
- [x] PRODUCTION tier auto-enforces all 5 policy categories
- [x] Deployment gates block production deployment before non-prod completion
- [x] Frontend red badge/border for production environments
- [x] Warning dialog on production deployment
- [x] Risk Assessment receives tier info (30% weight factor)

## Dependencies

- CR-ENT-006 (Risk Assessment) — tier = 30% risk weight
- CR-ENT-007 (Approval Workflow) — production mandatory approval
- CR-ENT-012 (Data Masking) — production enforced masking
- CR-ENT-005 (Copy Restriction) — production enforced copy restrict

## Definition of Done

- [x] Tier classification + auto-policies tested
- [x] Deployment gates verified
- [x] Frontend indicators functional

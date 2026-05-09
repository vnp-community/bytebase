# Change Request: Environment Tiers

| Field | Value |
|---|---|
| **CR ID** | CR-ENT-019 |
| **Feature ID** | ADM-05 |
| **Title** | Environment Tiers (Production Tier) |
| **Plan** | ENTERPRISE |
| **Priority** | P1 — High |
| **Status** | Draft |
| **Created** | 2026-05-08 |

---

## 1. Tổng quan

Phân tầng environments thành **production tier** và **non-production tier**, áp dụng policies khác nhau cho mỗi tier. Production tier có stricter controls: mandatory approval, restricted access, enhanced audit.

## 2. Yêu cầu chức năng

### FR-001: Environment Tier Classification
- **Tiers**:
  - `PRODUCTION` — Strict controls, mandatory approval, full audit
  - `NON_PRODUCTION` — Relaxed controls, optional approval
- Admin classify mỗi environment vào tier
- Default: `NON_PRODUCTION`

### FR-002: Tier-Based Policy Enforcement
- **Production Tier** auto-applies:
  - Mandatory approval workflow (no auto-approve)
  - Enhanced audit logging (all SQL queries logged)
  - Data masking enforced
  - Copy data restricted
  - Read-only access default (query only)
- **Non-Production Tier**:
  - Auto-approve cho low-risk changes
  - Relaxed masking policies
  - Read-write access default

### FR-003: Tier Indicators
- Visual tier badge trên environment cards
- Production environments highlighted (e.g., red border/badge)
- Warning khi deploying to production tier

### FR-004: Tier-Based Deployment Gates
- Enforce deployment order: non-production → production
- Block direct-to-production deployments unless emergency bypass
- Progressive rollout requires all non-prod stages complete

## 3. Backend Changes

| Component | Thay đổi |
|---|---|
| `backend/api/v1/environment_service.go` | Tier classification CRUD |
| `backend/runner/approval/` | Tier-based approval routing |
| `backend/api/v1/org_policy_service.go` | Tier-based policy resolution |
| `enterprise/feature.go` | `FeatureEnvironmentTiers` |

## 4. Database Changes

```sql
ALTER TABLE environment ADD COLUMN tier TEXT NOT NULL DEFAULT 'NON_PRODUCTION'
    CHECK (tier IN ('PRODUCTION', 'NON_PRODUCTION'));
```

## 5. Test Cases

| TC | Mô tả | Expected |
|---|---|---|
| TC-001 | Deploy to PRODUCTION tier without approval | Blocked |
| TC-002 | Deploy to NON_PRODUCTION tier | Auto-approve (if low risk) |
| TC-003 | Direct-to-production deployment | Warning + require explicit override |
| TC-004 | Production tier visual indicator | Red badge visible |
| TC-005 | Non-ENTERPRISE | Tier classification hidden |

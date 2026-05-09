# Change Request: Feature Gate Rebalancing & Pricing Optimization

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-006                                               |
| **Limitation ID**  | LIM-006                                                  |
| **Title**          | Feature Gate Rebalancing & Pricing Optimization          |
| **Category**       | Licensing / Business                                     |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Điều chỉnh feature gating strategy để cân bằng giữa **security accessibility** và **enterprise monetization**. Tăng FREE plan limits, đưa security-baseline features (2FA) vào TEAM plan, và mở rộng TEAM audit log retention để đáp ứng compliance cơ bản.

### 1.2 Bối cảnh
Hiện tại nhiều tính năng security cơ bản (2FA, full audit log, approval workflow) bị khóa sau Enterprise license. FREE plan giới hạn 10 instances không đủ cho multi-env setup (dev/staging/prod × 3-4 services = 9-12 instances). TEAM audit log chỉ 7 ngày — không đủ cho compliance tiêu chuẩn (90 ngày - 1 năm).

### 1.3 Mục tiêu
- Tăng FREE instance limit: 10 → 25
- Đưa 2FA vào TEAM plan (security baseline)
- Tăng TEAM audit log retention: 7 ngày → 90 ngày
- Thêm TEAM password restrictions (basic policy)
- Flexible instance counting (exclude inactive/archived)
- Transparent upgrade path UI

---

## 2. Yêu cầu chức năng

### FR-001: FREE Plan Instance Limit Increase (10 → 25)
- **Mô tả**: Tăng giới hạn instances cho FREE plan từ 10 lên 25.
- **Rationale**: Typical multi-env setup (dev/staging/prod) × 5-8 services = 15-24 instances. Giới hạn 10 buộc users upgrade quá sớm trước khi trải nghiệm đủ value.
- **Implementation**:
  ```go
  // enterprise/plan.go
  case api.FREE:
      return PlanLimit{
          MaxInstances: 25,  // was 10
          MaxSeats:     20,
      }
  ```
- **Acceptance Criteria**:
  - AC-1: FREE workspace cho phép tạo tới 25 instances
  - AC-2: Existing FREE workspaces tự động được nâng limit
  - AC-3: Terraform Provider reflect new limit

### FR-002: 2FA Downgrade to TEAM Plan
- **Mô tả**: Chuyển Two-Factor Authentication từ ENTERPRISE-only sang TEAM plan.
- **Rationale**: 2FA là security baseline (NIST SP 800-63b). Khóa sau Enterprise tạo rủi ro cho organizations dùng TEAM plan.
- **Implementation**:
  ```go
  // enterprise/feature.go
  Feature2FA: {
      MinimumPlan: api.TEAM,  // was api.ENTERPRISE
  }
  ```
- **Acceptance Criteria**:
  - AC-1: TEAM users có thể enable/configure 2FA
  - AC-2: 2FA enforcement policy available cho TEAM workspace admins
  - AC-3: TOTP setup flow accessible cho TEAM users

### FR-003: TEAM Audit Log Retention Extension (7 → 90 days)
- **Mô tả**: Tăng audit log retention cho TEAM plan từ 7 ngày lên 90 ngày.
- **Rationale**: SOC 2, ISO 27001, GDPR đều yêu cầu ≥ 90 ngày retention. 7 ngày không đủ cho compliance cơ bản.
- **Implementation**:
  ```go
  // enterprise/audit.go
  case api.TEAM:
      return AuditConfig{
          Retention: 90 * 24 * time.Hour,  // was 7 days
          ExportEnabled: true,              // allow export
      }
  ```
- **Acceptance Criteria**:
  - AC-1: TEAM audit logs retained for 90 days
  - AC-2: Audit log export (CSV/JSON) available for TEAM
  - AC-3: Cleanup job respects new retention period
  - AC-4: Storage impact documented (estimated growth per workspace)

### FR-004: TEAM Password Restrictions (Basic)
- **Mô tả**: Cung cấp basic password policy cho TEAM plan.
- **TEAM Policy Features**:
  - Minimum length: 8-32 characters (configurable)
  - Require mixed case: yes/no
  - Require numbers: yes/no
  - Password expiry: 90/180/365 days
- **ENTERPRISE-only Features** (unchanged):
  - Password history enforcement
  - Custom regex patterns
  - Breach database check
- **Acceptance Criteria**:
  - AC-1: TEAM workspace admins can configure basic password policy
  - AC-2: Password validation enforced on user creation and password change
  - AC-3: Policy configuration UI accessible for TEAM admins

### FR-005: Smart Instance Counting
- **Mô tả**: Cải thiện instance counting logic — chỉ đếm ACTIVE instances.
- **Excluded from Count**:
  - ARCHIVED instances
  - DELETED instances
  - Instances in MAINTENANCE mode (temporarily)
- **Acceptance Criteria**:
  - AC-1: Instance quota chỉ đếm instances ở trạng thái ACTIVE
  - AC-2: Archive instance → quota freed
  - AC-3: Unarchive instance → quota re-consumed (check limit)
  - AC-4: API và UI reflect accurate counting

### FR-006: Upgrade Path Transparency
- **Mô tả**: Clear UI cho users hiểu feature differences và upgrade path.
- **Components**:
  - Plan comparison page (FREE vs TEAM vs ENTERPRISE)
  - Feature lock indicators (lock icon + "Available in TEAM/ENTERPRISE" tooltip)
  - Usage dashboard (instance count, seat count, audit log age)
  - In-context upgrade prompts (non-intrusive)
- **Acceptance Criteria**:
  - AC-1: Plan comparison page accessible from Settings
  - AC-2: Locked features show informative tooltip (not just disabled)
  - AC-3: Usage metrics dashboard accurate
  - AC-4: Upgrade prompts appear at 80% quota usage

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| Plan Limits                  | `backend/enterprise/plan.go`              | Update FREE instance limit to 25                   |
| Feature Gate — 2FA           | `backend/enterprise/feature.go`           | Move 2FA minimum plan to TEAM                      |
| Audit Retention              | `backend/enterprise/audit.go`             | Update TEAM retention to 90 days                   |
| Password Policy              | `backend/enterprise/password.go`          | Split basic/advanced policy by plan                |
| Instance Counter             | `backend/store/instance.go`               | Count only ACTIVE instances for quota              |
| Plan Comparison API          | `backend/api/v1/subscription_service.go`  | Expose plan comparison data                        |
| License JWT Validation       | `backend/enterprise/license.go`           | Validate new limits                                |

### 3.2 Frontend Changes

| Component           | File                                    | Thay đổi                                    |
|---------------------|-----------------------------------------|----------------------------------------------|
| Plan Comparison     | `frontend/src/pages/PlanComparison`     | New page with feature matrix                 |
| Feature Lock UI     | `frontend/src/components/FeatureLock`   | Lock indicator component                     |
| Usage Dashboard     | `frontend/src/pages/Settings/Usage`     | Quota usage visualization                    |
| 2FA Settings        | `frontend/src/pages/Settings/Security`  | Enable 2FA UI for TEAM users                 |
| Password Policy     | `frontend/src/pages/Settings/Security`  | Basic password policy config for TEAM        |

### 3.3 Database Changes
Không cần schema migration — thay đổi business logic trong feature gate layer.

---

## 4. Phụ thuộc

| Dependency          | Mô tả                                                    |
|---------------------|-----------------------------------------------------------|
| License Service     | Feature gate decisions                                    |
| Stripe Integration  | Update plan definitions for SaaS billing (nếu applicable) |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                        |
|------------|----------------------------------------------------------|----------------------------------------|
| TC-001     | FREE plan: create 25th instance                          | Success                                |
| TC-002     | FREE plan: create 26th instance                          | Error: RESOURCE_EXHAUSTED              |
| TC-003     | TEAM plan: enable 2FA                                    | Success, TOTP setup completes          |
| TC-004     | FREE plan: enable 2FA                                    | Blocked, upgrade prompt shown          |
| TC-005     | TEAM plan: audit log query at 89 days                    | Records returned                       |
| TC-006     | TEAM plan: audit log query at 91 days                    | Records expired/cleaned                |
| TC-007     | TEAM plan: set password min length to 12                 | Policy enforced on next password set   |
| TC-008     | Instance archive → quota freed                           | Instance count decremented             |
| TC-009     | Instance unarchive when at quota limit                   | Error: RESOURCE_EXHAUSTED              |
| TC-010     | Plan comparison page loads                               | All features listed with plan badges   |

---

## 6. Updated Pricing Matrix

| Dimension                | FREE (Updated) | TEAM (Updated) | ENTERPRISE     |
|--------------------------|----------------|----------------|----------------|
| **Maximum Instances**    | **25** (was 10)| 10             | Unlimited      |
| **Maximum Seats**        | 20             | Unlimited      | Unlimited      |
| **2FA**                  | —              | **✅** (was ENT)| ✅             |
| **Audit Log**            | —              | **90 days** (was 7) | Unlimited  |
| **Password Policy**      | —              | **Basic** (was ENT) | Full       |
| **SSO**                  | —              | Google/GitHub   | Full           |
| **Data Masking**         | —              | —              | ✅             |
| **Approval Workflow**    | —              | —              | ✅             |
| **Custom Roles**         | —              | —              | ✅             |
| **SCIM/Directory Sync**  | —              | —              | ✅             |

---

## 7. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | FREE instance limit increase (25)                | Sprint 1       |
| Phase 2 | 2FA downgrade to TEAM                            | Sprint 1       |
| Phase 3 | TEAM audit log retention (90 days)               | Sprint 2       |
| Phase 4 | TEAM basic password policy                       | Sprint 2       |
| Phase 5 | Smart instance counting                          | Sprint 3       |
| Phase 6 | Upgrade path UI + plan comparison                | Sprint 3-4     |
| Phase 7 | Documentation + Stripe plan update               | Sprint 4       |

---

## 8. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                           |
|-----------------------------------------|--------|------------------------------------------------------|
| Revenue impact from feature downgrade   | HIGH   | Compensate with better conversion funnel              |
| Audit log storage growth (TEAM 90d)     | MEDIUM | Estimate per-workspace growth, plan DB capacity       |
| Existing TEAM users expect 7d behavior  | LOW    | Auto-extend — no negative impact                     |
| FREE 25 instances reduces upgrade need  | MEDIUM | More users → better funnel → more upgrades long-term |

# Solution: CR-LIM-006 — Feature Gate Rebalancing & Pricing Optimization

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-006                               |
| **Solution ID**| SOL-LIM-006                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L9 (Enterprise Layer), L4 (Service Layer), L8 (Store) |
| **TDD Refs**   | §14 Trade-offs (Feature-gated licensing), Architecture §10 |

---

## 1. Solution Overview

### 1.1 Approach Summary

**Proposed Architecture Enhancement**: Refactor L9 Enterprise Layer from hardcoded `plan.yaml` to **dynamic Plan Policy Engine** — enabling plan-level configuration changes without code deployment.

3-phase approach:

1. **Phase A — Plan Limits Tuning** (FREE 25 instances, TEAM audit 90d)
2. **Phase B — Feature Tier Reshuffling** (2FA → TEAM, basic password policy → TEAM)
3. **Phase C — Dynamic Plan Policy Engine + Upgrade UX** (architecture change)

### 1.2 Architectural Change Proposal

> **⚠️ PROPOSED ARCHITECTURE CHANGE — L9 Enterprise Layer Refactoring**

**Current Architecture** (Architecture §10, TDD §14):
```yaml
# backend/enterprise/plan.yaml — STATIC file, requires code deploy to change
FREE:
  maxInstances: 10
  maxSeats: 20
  features: [...]
TEAM:
  maxInstances: 10
  features: [...]
ENTERPRISE:
  maxInstances: -1  # unlimited
  features: [...]
```

```go
// backend/enterprise/license.go — Hardcoded feature checks
func (s *LicenseService) IsFeatureEnabled(ctx, workspaceID, feature) error {
    plan := s.getPlan(workspaceID)
    // Static lookup from plan.yaml
    if !planFeatureMatrix[plan][feature] {
        return status.Errorf(codes.PermissionDenied, "requires %s plan", requiredPlan)
    }
}
```

**Problem**: Changing any limit/feature-tier requires:
1. Edit `plan.yaml`
2. Rebuild Go binary
3. Redeploy all instances

**Proposed Architecture** — Dynamic Plan Policy:
```go
// Plan configuration loaded from DB setting (overridable at runtime)
// Falls back to compiled defaults in plan.yaml if no DB override exists.
type PlanPolicy struct {
    Plan         api.PlanType
    Limits       PlanLimits
    Features     []api.Feature
    AuditConfig  AuditConfig
    PasswordConfig PasswordConfig
}

// DB-stored, hot-reloadable via setting table
// Admins can override defaults without code change
```

**Benefits**:
- Plan changes deployable via API/UI (no binary rebuild)
- Self-hosted operators can customize feature gates
- A/B testing plan configurations for SaaS
- Audit trail for plan policy changes

---

## 2. Detailed Technical Design

### 2.1 Phase A — Plan Limits Tuning

#### 2.1.1 FREE Instance Limit: 10 → 25

**File**: `backend/enterprise/plan.yaml` (modify)

```yaml
# BEFORE:
FREE:
  maxInstances: 10
  maxSeats: 20

# AFTER:
FREE:
  maxInstances: 25  # Supports typical multi-env: dev/staging/prod × 8 services
  maxSeats: 20
```

**File**: `backend/enterprise/license.go` (modify instance check)

```go
func (s *LicenseService) GetInstanceLimit(ctx context.Context) int {
    plan := s.GetCurrentPlan(ctx)
    switch plan {
    case api.FREE:
        return 25  // was 10
    case api.TEAM:
        return 10
    case api.ENTERPRISE:
        return -1  // unlimited
    }
    return 10
}
```

#### 2.1.2 Smart Instance Counting (ACTIVE only)

**File**: `backend/store/instance.go` (modify)

```go
// CountActiveInstances counts only ACTIVE instances for quota enforcement.
// ARCHIVED and DELETED instances do not consume quota.
func (s *Store) CountActiveInstances(ctx context.Context) (int, error) {
    var count int
    err := s.GetDB().QueryRowContext(ctx, `
        SELECT COUNT(*) FROM instance
        WHERE deleted = false AND archived = false
    `).Scan(&count)
    return count, err
}
```

**File**: `backend/api/v1/instance_service.go` (modify quota check)

```go
func (s *InstanceService) CreateInstance(ctx context.Context, req *connect.Request[v1pb.CreateInstanceRequest]) (*connect.Response[v1pb.Instance], error) {
    // Check quota using ACTIVE count only
    activeCount, err := s.store.CountActiveInstances(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "count instances: %v", err)
    }

    limit := s.licenseService.GetInstanceLimit(ctx)
    if limit > 0 && activeCount >= limit {
        return nil, status.Errorf(codes.ResourceExhausted,
            "instance limit reached (%d/%d active instances). Archive unused instances or upgrade to Enterprise plan.",
            activeCount, limit)
    }
    // ... existing create logic ...
}
```

#### 2.1.3 TEAM Audit Log Retention: 7 → 90 days

**File**: `backend/runner/cleaner/data_cleaner.go` (modify)

```go
func (c *DataCleaner) getAuditLogRetention(ctx context.Context) time.Duration {
    plan := c.licenseService.GetCurrentPlan(ctx)
    switch plan {
    case api.FREE:
        return 0  // No audit log access
    case api.TEAM:
        return 90 * 24 * time.Hour  // was 7 days
    case api.ENTERPRISE:
        return 0  // Unlimited (no cleanup)
    }
    return 7 * 24 * time.Hour
}

func (c *DataCleaner) cleanAuditLogs(ctx context.Context) {
    retention := c.getAuditLogRetention(ctx)
    if retention == 0 {
        return // No cleanup needed
    }
    cutoff := time.Now().Add(-retention)
    result, _ := c.store.GetDB().ExecContext(ctx, `
        DELETE FROM audit_log WHERE created_ts < $1
    `, cutoff)
    if n, _ := result.RowsAffected(); n > 0 {
        slog.Info("Cleaned audit logs", "deleted", n, "retention", retention)
    }
}
```

### 2.2 Phase B — Feature Tier Reshuffling

#### 2.2.1 2FA → TEAM Plan

**File**: `backend/enterprise/plan.yaml` (modify)

```yaml
# BEFORE:
TEAM:
  features:
    - FEATURE_SSO_GOOGLE_GITHUB
    - FEATURE_AUDIT_LOG
    - FEATURE_BATCH_QUERY
    # 2FA is NOT here — it's ENTERPRISE only

# AFTER:
TEAM:
  features:
    - FEATURE_SSO_GOOGLE_GITHUB
    - FEATURE_AUDIT_LOG
    - FEATURE_BATCH_QUERY
    - FEATURE_2FA               # NEW — moved from ENTERPRISE
    - FEATURE_PASSWORD_BASIC    # NEW — basic password policy
```

**File**: `backend/enterprise/license.go` (modify feature check)

```go
// Feature minimum plan mapping
var featureMinPlan = map[v1pb.PlanFeature]api.PlanType{
    // ... existing ...
    v1pb.PlanFeature_FEATURE_2FA:              api.TEAM,       // was api.ENTERPRISE
    v1pb.PlanFeature_FEATURE_PASSWORD_BASIC:   api.TEAM,       // NEW
    v1pb.PlanFeature_FEATURE_PASSWORD_ADVANCED: api.ENTERPRISE, // History, regex, breach check
}
```

#### 2.2.2 Basic Password Policy for TEAM

**File**: `backend/api/v1/setting_service.go` (extend)

```go
// Password policy configuration
type PasswordPolicy struct {
    MinLength       int    `json:"minLength"`       // 8-32
    RequireMixedCase bool  `json:"requireMixedCase"`
    RequireNumbers   bool  `json:"requireNumbers"`
    RequireSpecial   bool  `json:"requireSpecial"`  // ENTERPRISE only
    ExpiryDays       int   `json:"expiryDays"`      // 0 = no expiry
    HistoryCount     int   `json:"historyCount"`    // ENTERPRISE only
}

func (s *SettingService) UpdatePasswordPolicy(ctx context.Context, req *connect.Request[v1pb.UpdatePasswordPolicyRequest]) (*connect.Response[v1pb.PasswordPolicy], error) {
    plan := s.licenseService.GetCurrentPlan(ctx)

    policy := req.Msg.Policy

    // TEAM: basic password settings only
    if plan == api.TEAM {
        // Enforce TEAM limits
        policy.RequireSpecial = false  // ENTERPRISE only
        policy.HistoryCount = 0        // ENTERPRISE only
    }

    // Validate
    if policy.MinLength < 8 || policy.MinLength > 32 {
        return nil, status.Errorf(codes.InvalidArgument, "minLength must be 8-32")
    }

    // Store as setting
    data, _ := protojson.Marshal(policy)
    err := s.store.UpsertSetting(ctx, &store.SetSettingMessage{
        Name:  "workspace.password-policy",
        Value: string(data),
    })
    return connect.NewResponse(policy), err
}
```

**File**: `backend/api/v1/auth_service.go` (extend password validation)

```go
func (s *AuthService) validatePassword(ctx context.Context, password string) error {
    setting, err := s.store.GetSetting(ctx, "workspace.password-policy")
    if err != nil || setting == nil {
        return nil // No policy configured
    }

    var policy PasswordPolicy
    protojson.Unmarshal([]byte(setting.Value), &policy)

    if len(password) < policy.MinLength {
        return status.Errorf(codes.InvalidArgument,
            "password must be at least %d characters", policy.MinLength)
    }
    if policy.RequireMixedCase && !hasMixedCase(password) {
        return status.Errorf(codes.InvalidArgument,
            "password must contain both upper and lower case letters")
    }
    if policy.RequireNumbers && !hasNumber(password) {
        return status.Errorf(codes.InvalidArgument,
            "password must contain at least one number")
    }
    return nil
}
```

### 2.3 Phase C — Dynamic Plan Policy Engine

> **Architecture Change**: Move plan configuration from static `plan.yaml` to DB-stored settings with runtime override capability.

#### 2.3.1 Plan Policy Store

**File**: `backend/enterprise/policy_engine.go` (new)

```go
// PlanPolicyEngine provides dynamic, hot-reloadable plan configuration.
// Priority: DB setting override > plan.yaml defaults > hardcoded fallback.
type PlanPolicyEngine struct {
    store          *store.Store
    defaultPolicies map[api.PlanType]*PlanPolicy  // From plan.yaml
    cache          atomic.Value                    // Cached active policies
    mu             sync.RWMutex
}

type PlanPolicy struct {
    MaxInstances    int             `json:"maxInstances"`
    MaxSeats        int             `json:"maxSeats"`
    Features        []string        `json:"features"`
    AuditRetention  string          `json:"auditRetention"`  // duration string
    PasswordTier    string          `json:"passwordTier"`    // "none", "basic", "advanced"
}

func NewPlanPolicyEngine(store *store.Store) *PlanPolicyEngine {
    engine := &PlanPolicyEngine{
        store:           store,
        defaultPolicies: loadPlanYAML(), // Load from embedded plan.yaml
    }
    engine.reload(context.Background())
    return engine
}

// GetPolicy returns the effective policy for a plan.
// Checks DB override first, then falls back to plan.yaml defaults.
func (e *PlanPolicyEngine) GetPolicy(ctx context.Context, plan api.PlanType) *PlanPolicy {
    // Check DB override
    setting, err := e.store.GetSetting(ctx, fmt.Sprintf("plan.policy.%s", plan))
    if err == nil && setting != nil {
        var override PlanPolicy
        if err := json.Unmarshal([]byte(setting.Value), &override); err == nil {
            return e.mergeWithDefaults(plan, &override)
        }
    }
    // Fallback to compiled defaults
    return e.defaultPolicies[plan]
}

// mergeWithDefaults applies override values while keeping defaults for unset fields
func (e *PlanPolicyEngine) mergeWithDefaults(plan api.PlanType, override *PlanPolicy) *PlanPolicy {
    defaults := e.defaultPolicies[plan]
    merged := *defaults // copy

    if override.MaxInstances > 0 {
        merged.MaxInstances = override.MaxInstances
    }
    if override.MaxSeats > 0 {
        merged.MaxSeats = override.MaxSeats
    }
    if len(override.Features) > 0 {
        merged.Features = override.Features
    }
    if override.AuditRetention != "" {
        merged.AuditRetention = override.AuditRetention
    }
    return &merged
}

// UpdatePolicy allows workspace admins (self-hosted) to override plan policy.
// This is a self-hosted-only feature for customizing feature gates.
func (e *PlanPolicyEngine) UpdatePolicy(ctx context.Context, plan api.PlanType, policy *PlanPolicy) error {
    data, err := json.Marshal(policy)
    if err != nil {
        return err
    }
    return e.store.UpsertSetting(ctx, &store.SetSettingMessage{
        Name:  fmt.Sprintf("plan.policy.%s", plan),
        Value: string(data),
    })
}
```

#### 2.3.2 Refactor LicenseService to use PolicyEngine

**File**: `backend/enterprise/license.go` (modify)

```go
type LicenseService struct {
    store        *store.Store
    policyEngine *PlanPolicyEngine  // NEW — replaces direct plan.yaml reads
}

func (s *LicenseService) GetInstanceLimit(ctx context.Context) int {
    plan := s.GetCurrentPlan(ctx)
    policy := s.policyEngine.GetPolicy(ctx, plan)
    return policy.MaxInstances // Dynamic from DB or defaults
}

func (s *LicenseService) IsFeatureEnabled(ctx context.Context, workspaceID string, feature v1pb.PlanFeature) error {
    plan := s.GetCurrentPlan(ctx)
    policy := s.policyEngine.GetPolicy(ctx, plan)

    featureName := feature.String()
    for _, f := range policy.Features {
        if f == featureName {
            return nil // Feature enabled
        }
    }
    return status.Errorf(codes.PermissionDenied,
        "feature %s requires a higher plan (current: %s)", featureName, plan)
}
```

#### 2.3.3 Plan Comparison API

**File**: `backend/api/v1/subscription_service.go` (extend)

```go
func (s *SubscriptionService) ComparePlans(ctx context.Context, req *connect.Request[v1pb.ComparePlansRequest]) (*connect.Response[v1pb.ComparePlansResponse], error) {
    plans := []api.PlanType{api.FREE, api.TEAM, api.ENTERPRISE}
    var comparisons []*v1pb.PlanComparison

    for _, plan := range plans {
        policy := s.policyEngine.GetPolicy(ctx, plan)
        comparisons = append(comparisons, &v1pb.PlanComparison{
            Plan:           string(plan),
            MaxInstances:   int32(policy.MaxInstances),
            MaxSeats:       int32(policy.MaxSeats),
            Features:       policy.Features,
            AuditRetention: policy.AuditRetention,
        })
    }

    // Add current workspace usage
    currentPlan := s.licenseService.GetCurrentPlan(ctx)
    activeInstances, _ := s.store.CountActiveInstances(ctx)
    activeUsers, _ := s.store.CountActiveUsers(ctx)

    return connect.NewResponse(&v1pb.ComparePlansResponse{
        Plans:           comparisons,
        CurrentPlan:     string(currentPlan),
        UsedInstances:   int32(activeInstances),
        UsedSeats:       int32(activeUsers),
    }), nil
}
```

#### 2.3.4 Upgrade Prompt Logic

**File**: `backend/api/v1/actuator_service.go` (extend)

```go
// GetUpgradeHints returns upgrade suggestions based on current usage.
func (s *ActuatorService) GetUpgradeHints(ctx context.Context, req *connect.Request[v1pb.GetUpgradeHintsRequest]) (*connect.Response[v1pb.UpgradeHintsResponse], error) {
    plan := s.licenseService.GetCurrentPlan(ctx)
    policy := s.policyEngine.GetPolicy(ctx, plan)

    hints := &v1pb.UpgradeHintsResponse{}

    // Instance quota warning
    activeInstances, _ := s.store.CountActiveInstances(ctx)
    if policy.MaxInstances > 0 {
        ratio := float64(activeInstances) / float64(policy.MaxInstances)
        if ratio >= 0.8 {
            hints.Hints = append(hints.Hints, &v1pb.UpgradeHint{
                Type:    "QUOTA_WARNING",
                Message: fmt.Sprintf("You're using %d/%d instances (%.0f%%)", activeInstances, policy.MaxInstances, ratio*100),
                Action:  "UPGRADE_PLAN",
            })
        }
    }

    // Feature lock hints (based on attempted feature access)
    // Tracked via audit log - recent permission denied errors
    recentDenials, _ := s.store.ListRecentFeatureDenials(ctx, 7*24*time.Hour)
    for _, denial := range recentDenials {
        hints.Hints = append(hints.Hints, &v1pb.UpgradeHint{
            Type:    "FEATURE_LOCKED",
            Feature: denial.Feature,
            Message: fmt.Sprintf("Feature '%s' was requested but requires %s plan", denial.Feature, denial.RequiredPlan),
            Action:  "UPGRADE_PLAN",
        })
    }

    return connect.NewResponse(hints), nil
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| **L9 (Enterprise)** | **HIGH** | PlanPolicyEngine, feature reshuffling, dynamic config |
| **L4 (Service)** | **MEDIUM** | Password policy, plan comparison, upgrade hints |
| L8 (Store) | **LOW** | Smart instance counting, settings storage |
| L6 (Runner) | **LOW** | Audit retention config change in DataCleaner |
| L1 (Frontend) | **MEDIUM** | Plan comparison page, upgrade prompts, feature locks |

### 3.1 Architecture Layer Change Summary

```
BEFORE (L9):
  plan.yaml (static) → LicenseService (hardcoded checks)

AFTER (L9):
  plan.yaml (defaults) → PlanPolicyEngine → DB settings (overrides)
                                ↓
                        LicenseService (dynamic checks)
```

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
Phase A (Sprint 1):
  1. Update plan.yaml: FREE maxInstances=25
  2. Smart instance counting (ACTIVE only)
  3. Audit retention: TEAM 7d → 90d
  4. Test: existing plans unaffected, new limits work

Phase B (Sprint 2):
  5. Move 2FA to TEAM plan
  6. Add basic password policy for TEAM
  7. Test: TEAM users can enable 2FA, set password policy

Phase C (Sprint 3-4):
  8. Implement PlanPolicyEngine
  9. Refactor LicenseService to use PolicyEngine
  10. Plan comparison API
  11. Upgrade hints API
  12. Frontend: comparison page, lock indicators, prompts
```

### 4.2 Rollback Plan

```
Phase A: Revert plan.yaml values (instant, no data loss)
Phase B: Revert featureMinPlan map (instant)
Phase C: PlanPolicyEngine falls back to plan.yaml if no DB override
```

---

## 5. Updated Plan Matrix (After All Phases)

| Dimension                | FREE (New)       | TEAM (New)          | ENTERPRISE       |
|--------------------------|------------------|---------------------|------------------|
| **Maximum Instances**    | **25** (was 10)  | 10                  | Unlimited        |
| **Maximum Seats**        | 20               | Unlimited           | Unlimited        |
| **2FA**                  | —                | **✅** (was ENT)    | ✅               |
| **Password Policy**      | —                | **Basic** (new)     | Advanced         |
| **Audit Log**            | —                | **90 days** (was 7) | Unlimited        |
| **SSO**                  | —                | Google/GitHub        | Full OIDC/SAML   |
| **Data Masking**         | —                | —                   | ✅               |
| **Approval Workflow**    | —                | —                   | ✅               |
| **Custom Roles**         | —                | —                   | ✅               |
| **Instance Counting**    | **ACTIVE only**  | ACTIVE only         | ACTIVE only      |

---

## 6. Performance Considerations

- **Audit log storage** (TEAM 90d): Estimated ~10MB/workspace/month for typical usage. Monitor via `bytebase_audit_log_size_bytes` metric.
- **PlanPolicyEngine cache**: Settings cached in L8 settingCache (1,024 entries, TTL from cache layer). Policy lookup adds < 0.1ms overhead.
- **Instance counting**: `CountActiveInstances()` is indexed query — sub-millisecond on typical deployments.

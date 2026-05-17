# Solution: CR-SEC-004 — ABAC Enhancement

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-004                |
| **Solution**   | SOL-SEC-004               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng CEL-based IAM Manager (L5: `component/iam/`) để inject context attributes (time, IP, environment tier, risk level) vào CEL evaluation context. Reuse existing CEL infrastructure (TDD Section 7.2) — Bytebase đã sử dụng CEL cho permission conditions. Thêm emergency access override endpoint (L4) với mandatory MFA re-authentication.

---

## 2. Architectural Alignment

```
L3 ACL Interceptor ──► L5 IAM Manager (CEL engine)
       │                      │
       │ Context Injection     │ Extend CEL Variables:
       │ (IP, time, env)      │   request.time
       │                      │   request.source_ip
       │                      │   request.environment.tier
       │                      │   change.risk_level
       │                      │
L4 Service ──► EmergencyAccessService (new)
       │
L8 Store ──► store/abac_policy.go (extend policy.go)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3** | `acl.go` (19.3KB) | Inject request context attributes |
| **L5** | `component/iam/` | CEL engine extension with ABAC variables |
| **L4** | `emergency_access.go` (new) | Break-glass procedure |
| **L8** | `store/policy.go` | ABAC policy storage (reuse existing policy table) |
| **L9** | `enterprise/feature.go` | `FeatureABAC` gate |

---

## 3. Chi tiết Implementation

### 3.1 L5 — CEL Context Extension

**File**: `backend/component/iam/manager.go`

Bytebase already uses CEL for IAM conditions. Extend the CEL environment:

```go
func (m *Manager) buildCELEnv(ctx context.Context, req *RequestContext) *cel.Env {
    env, _ := cel.NewEnv(
        // Existing variables
        cel.Variable("user", cel.ObjectType("User")),
        cel.Variable("resource", cel.ObjectType("Resource")),

        // NEW: ABAC context attributes
        cel.Variable("request", cel.ObjectType("RequestContext")),
    )
    return env
}

type RequestContext struct {
    Time          time.Time          // Current server time
    SourceIP      string             // Client IP
    UserAgent     string
    Environment   *EnvironmentContext
    Change        *ChangeContext
}

type EnvironmentContext struct {
    Tier     string // "PRODUCTION", "STAGING", "DEVELOPMENT"
    Name     string
}

type ChangeContext struct {
    RiskLevel    string // "HIGH", "MEDIUM", "LOW" (from SEC-08 Risk Assessment)
    StatementType string // "DDL", "DML", "DQL"
}
```

### 3.2 L3 — ACL Interceptor Context Injection

**File**: `backend/api/v1/acl.go`

```go
func (a *ACLInterceptor) buildRequestContext(ctx context.Context, method string) *RequestContext {
    reqCtx := &RequestContext{
        Time:     time.Now(),
        SourceIP: extractClientIP(ctx),
    }

    // Resolve environment tier from request resource
    if envName := extractEnvironmentFromMethod(ctx, method); envName != "" {
        env, _ := a.store.GetEnvironment(ctx, envName)
        reqCtx.Environment = &EnvironmentContext{
            Tier: env.GetTier().String(), // from CR-ENT-019 Environment Tiers
            Name: envName,
        }
    }

    return reqCtx
}
```

### 3.3 L8 — ABAC Policy Storage

Reuse existing `policy` table (JSONB payload) with new policy type:

```go
// Policy type enum extension
const PolicyType_ABAC PolicyType = "ABAC"

// ABAC policy payload stored as JSONB in policy.payload
type ABACPolicy struct {
    Rules []ABACRule `json:"rules"`
}

type ABACRule struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Condition   string `json:"condition"`   // CEL expression
    Effect      string `json:"effect"`      // "DENY" or "ALLOW"
    Priority    int    `json:"priority"`    // Lower = higher priority
}
```

### 3.4 L4 — Emergency Access

**File**: `backend/api/v1/emergency_access.go` (new)

```go
func (s *EmergencyAccessService) RequestEmergencyAccess(ctx context.Context, req *v1pb.EmergencyAccessRequest) (*v1pb.EmergencyAccessResponse, error) {
    user := getUserFromContext(ctx)

    // Require MFA re-authentication
    if !s.verify2FA(ctx, user, req.TotpCode) {
        return nil, status.Errorf(codes.Unauthenticated, "MFA required for emergency access")
    }

    // Create time-limited override (max 4h)
    duration := min(req.DurationMinutes, 240)
    override := &store.EmergencyOverride{
        UserUID:       user.UID,
        Justification: req.Justification, // mandatory
        ExpiresAt:     time.Now().Add(time.Duration(duration) * time.Minute),
    }
    s.store.CreateEmergencyOverride(ctx, override)

    // Notify all workspace admins
    s.webhookManager.NotifyEmergencyAccess(ctx, user, override)

    // Audit log with special category
    s.auditEmergencyAccess(ctx, user, override)

    return &v1pb.EmergencyAccessResponse{ExpiresAt: override.ExpiresAt}, nil
}
```

---

## 4. Database Changes

```sql
CREATE TABLE emergency_override (
    id            BIGSERIAL PRIMARY KEY,
    user_uid      INT NOT NULL REFERENCES principal(id),
    justification TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_ts    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked       BOOLEAN NOT NULL DEFAULT FALSE
);
```

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-011 (Custom Roles) | ABAC extends custom role permissions |
| CR-ENT-019 (Env Tiers) | Environment tier used as ABAC attribute |
| CR-ENT-006 (Risk Assessment) | Risk level used as ABAC attribute |
| CR-ENT-009 (2FA) | MFA required for emergency access |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | CEL environment extension with request context | Sprint 1 |
| 2 | ACL interceptor context injection | Sprint 2 |
| 3 | ABAC policy storage + admin UI | Sprint 3 |
| 4 | Emergency access + MFA integration | Sprint 4 |

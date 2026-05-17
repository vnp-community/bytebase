# Solution: CR-SEC-006 — IP Allowlisting & Geo-Restriction

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-006                |
| **Solution**   | SOL-SEC-006               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Thêm IP Policy middleware (L2) vào Echo middleware stack trước Auth Interceptor. Reuse GeoIP component từ SOL-SEC-003. IP policies lưu trong existing `policy` table (L8) via `OrgPolicyService` (L4). Hỗ trợ CIDR matching, country-based blocking, proxy/reverse-proxy X-Forwarded-For parsing.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L2** | `echo_routes.go` → new `ip_policy_middleware` | First-line IP validation |
| **L4** | `setting_service.go` | IP policy configuration API |
| **L5** | `component/geoip/` (from SOL-SEC-003) | IP → country resolution |
| **L8** | `store/policy.go` | IP policy storage (reuse policy table) |

---

## 3. Chi tiết Implementation

### 3.1 L2 — IP Policy Middleware

**File**: `backend/server/echo_routes.go`

Insert into middleware stack (Architecture L2) **after** `recoverMiddleware` but **before** `securityHeadersMiddleware`:

```go
func ipPolicyMiddleware(policyStore *store.Store, geoIP *geoip.Service) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            clientIP := resolveClientIP(c) // X-Forwarded-For with trusted proxy chain

            policy, _ := policyStore.GetWorkspaceIPPolicy(c.Request().Context())
            if policy == nil || policy.Mode == "disabled" {
                return next(c)
            }

            // Check IP allowlist/denylist
            allowed := evaluateIPPolicy(clientIP, policy)
            if !allowed {
                if policy.OnViolation == "mfa_challenge" {
                    c.Response().Header().Set("X-BB-MFA-Required", "true")
                    return c.JSON(403, map[string]string{"error": "MFA required from this IP"})
                }
                // Log blocked attempt
                return c.JSON(403, map[string]string{"error": "access denied from this IP"})
            }

            // Check geo-restriction
            if len(policy.AllowedCountries) > 0 || len(policy.DeniedCountries) > 0 {
                geo, _ := geoIP.Lookup(clientIP)
                if !isCountryAllowed(geo.Country, policy) {
                    return c.JSON(403, map[string]string{"error": "access denied from this region"})
                }
            }

            return next(c)
        }
    }
}

func resolveClientIP(c echo.Context) string {
    // Parse X-Forwarded-For with trusted proxy validation
    xff := c.Request().Header.Get("X-Forwarded-For")
    trustedProxies := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
    return extractRealIP(xff, c.RealIP(), trustedProxies)
}
```

### 3.2 L8 — IP Policy Storage

Reuse existing `policy` table with new policy type:

```go
const PolicyType_IP_POLICY PolicyType = "IP_POLICY"

type IPPolicy struct {
    Mode             string   `json:"mode"`             // "allowlist", "denylist", "monitor"
    Rules            []IPRule `json:"rules"`
    AllowedCountries []string `json:"allowedCountries"` // ISO 3166-1 alpha-2
    DeniedCountries  []string `json:"deniedCountries"`
    OnViolation      string   `json:"onViolation"`      // "block", "mfa_challenge", "log_only"
    ServiceAccountExempt bool `json:"serviceAccountExempt"`
}

type IPRule struct {
    CIDR  string `json:"cidr"`
    Label string `json:"label"`
}
```

### 3.3 L4 — Database Connection IP Restriction

**File**: `backend/component/dbfactory/factory.go`

Extend DBFactory to check per-instance IP allowlist:

```go
func (f *Factory) GetDriver(ctx context.Context, instance *store.InstanceMessage) (db.Driver, error) {
    // NEW: Check IP restriction for this instance
    if instance.IPAllowlist != nil {
        clientIP := extractClientIP(ctx)
        if !matchCIDR(clientIP, instance.IPAllowlist) {
            return nil, status.Errorf(codes.PermissionDenied,
                "database access denied from IP %s", clientIP)
        }
    }
    // Existing driver creation logic...
    return db.Open(ctx, instance.Engine, connectionConfig)
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-003 (Brute-Force) | Reuses GeoIP component |
| CR-SEC-010 (SIEM) | IP violation events forwarded to SIEM |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | IP policy middleware + CIDR matching | Sprint 1 |
| 2 | GeoIP country restriction | Sprint 2 |
| 3 | Per-instance DB connection restriction | Sprint 2 |
| 4 | IP policy management UI | Sprint 3 |

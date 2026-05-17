# Solution: CR-ENT-016 — Workload Identity (OIDC Federation)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-016                |
| **Solution**   | SOL-ENT-016               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Enhance `WorkloadIdentityService` hiện có (`backend/api/v1/workload_identity_service.go`, 14KB) và OIDC plugin (L7) để hỗ trợ token exchange từ CI/CD providers (GitHub Actions, GitLab CI). Passwordless authentication cho pipelines.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `workload_identity_service.go` (existing, 14KB) | Trust config CRUD + token exchange |
| **L4 — Service** | `auth/` | Handle workload identity tokens |
| **L7 — Plugin** | `plugin/idp/oidc/` | OIDC token validation (JWKS) |
| **L3 — Security** | ACL interceptor | Workload identity principal type |
| **L9 — Enterprise** | `feature.go` | `FeatureWorkloadIdentity` gate |

---

## 3. Chi tiết Implementation

### 3.1 Token Exchange Flow

```
CI/CD Pipeline → Get OIDC token from CI provider
  → POST /v1/auth:exchangeToken {
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: "<oidc_token>"
    }
  → Validate: issuer, signature (JWKS), claims, audience
  → Evaluate attribute_condition (CEL)
  → Issue short-lived Bytebase access token (1h default)
```

### 3.2 Trust Configuration

```go
type WorkloadIdentityConfig struct {
    Name              string
    Issuer            string   // e.g., "https://token.actions.githubusercontent.com"
    Audiences         []string
    AttributeMapping  map[string]string  // CEL expressions
    AttributeCondition string            // CEL: restrict repo/branch
}
```

### 3.3 Principal Type

Workload identity maps to existing principal type: `workloadIdentities/{email}`
- Distinct from service accounts
- Assignable roles via IAM policy (same mechanism)
- All actions audited

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-008 | Shares OIDC validation infrastructure |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | OIDC trust config CRUD | Sprint 1 |
| 2 | Token exchange endpoint | Sprint 1 |
| 3 | GitHub Actions integration | Sprint 2 |
| 4 | GitLab CI integration | Sprint 2 |
| 5 | IAM permission mapping | Sprint 3 |

# Change Request: Workload Identity (OIDC Federation)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-016                                               |
| **Feature ID**     | SEC-19                                                   |
| **Title**          | Workload Identity — OIDC Federation                      |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **Workload Identity** cho phép CI/CD pipelines và automated workloads xác thực với Bytebase mà **không cần static credentials** (API keys). Sử dụng OIDC token federation từ CI/CD providers (GitHub Actions, GitLab CI, Jenkins).

### 1.2 Mục tiêu
- Passwordless authentication cho CI/CD pipelines
- OIDC token exchange (CI/CD OIDC token → Bytebase access token)
- Short-lived tokens, no static credentials to manage
- Configurable trust policies (restrict by repo, branch, etc.)

---

## 2. Yêu cầu chức năng

### FR-001: OIDC Identity Provider Trust
- **Mô tả**: Configure trusted OIDC providers cho workload identity.
- **Supported Providers**:
  - GitHub Actions (`token.actions.githubusercontent.com`)
  - GitLab CI (`gitlab.com`)
  - Jenkins (custom OIDC plugin)
  - Custom OIDC provider
- **Configuration**:
  ```yaml
  workload_identity:
    name: "github-ci"
    issuer: "https://token.actions.githubusercontent.com"
    audiences: ["bytebase.example.com"]
    attribute_mapping:
      email: "repository_owner + '@github.com'"
    attribute_condition: |
      assertion.repository == "org/repo" &&
      assertion.ref == "refs/heads/main"
  ```
- **Acceptance Criteria**:
  - AC-1: OIDC discovery (fetch JWKS from issuer)
  - AC-2: Token validation (signature, claims, audience)
  - AC-3: Attribute condition restricts which workloads can authenticate
  - AC-4: Multiple trust configurations supported

### FR-002: Token Exchange Flow
- **Mô tả**: Exchange external OIDC token for Bytebase access token.
- **Flow**:
  ```
  CI/CD Pipeline → Get OIDC token from CI provider
    → POST /v1/auth:exchangeToken
       { "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
         "subject_token": "<oidc_token>",
         "subject_token_type": "urn:ietf:params:oauth:token-type:jwt" }
    → Validate OIDC token against trust config
    → Issue short-lived Bytebase access token (1 hour)
    → Pipeline uses Bytebase API with access token
  ```
- **Acceptance Criteria**:
  - AC-1: Access token lifetime configurable (default 1h, max 24h)
  - AC-2: Token scoped to specific permissions (based on workload identity mapping)
  - AC-3: No refresh token issued (one-time exchange)
  - AC-4: Rate limiting on token exchange endpoint

### FR-003: Workload Identity Permissions
- **Mô tả**: Map workload identity to Bytebase permissions.
- **Acceptance Criteria**:
  - AC-1: Workload identity principal type: `workloadIdentities/{email}`
  - AC-2: Assignable roles via IAM policy (same as users)
  - AC-3: Audit log captures workload identity actions
  - AC-4: Distinct from service accounts in principal hierarchy

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                             | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| Workload Identity Service    | `backend/api/v1/workload_identity.go`     | Trust config CRUD + token exchange                 |
| Auth Service                 | `backend/api/auth/`                       | Handle workload identity tokens                    |
| OIDC Validator               | `backend/plugin/idp/oidc/`               | OIDC token validation for workloads                |
| Feature Gate                 | `enterprise/feature.go`                   | Define `FeatureWorkloadIdentity`                   |

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Exchange GitHub Actions OIDC token                      | Bytebase access token issued          |
| TC-002     | Exchange token from untrusted issuer                    | Rejected: unknown issuer              |
| TC-003     | Token with failed attribute condition                   | Rejected: condition not met           |
| TC-004     | Expired OIDC token                                      | Rejected: token expired               |
| TC-005     | Use exchanged token for API call                        | API call succeeds                     |
| TC-006     | Access token expired (>1h)                              | API call rejected: token expired      |
| TC-007     | Non-ENTERPRISE: workload identity hidden                | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | OIDC trust config CRUD               | Sprint 1       |
| Phase 2 | Token exchange endpoint              | Sprint 1       |
| Phase 3 | GitHub Actions integration           | Sprint 2       |
| Phase 4 | GitLab CI integration                | Sprint 2       |
| Phase 5 | IAM permission mapping               | Sprint 3       |

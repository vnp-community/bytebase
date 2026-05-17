# TASK-ENT-019 — Workload Identity (OIDC Federation)

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-019                               |
| **Source**       | SOL-ENT-016 (CR-ENT-016)                  |
| **Status**       | Done                                       |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Enhance `WorkloadIdentityService` hiện có và OIDC plugin để hỗ trợ token exchange từ CI/CD providers (GitHub Actions, GitLab CI). Passwordless authentication cho pipelines.

## Scope

### Phase 1 — Sprint 1: Trust Config + Token Exchange
1. **L4 — WorkloadIdentityService Enhancement**: Trust configuration CRUD — issuer, audiences, attribute mapping (CEL), attribute condition (CEL)
2. **Token Exchange Endpoint**: `POST /v1/auth:exchangeToken` — validate OIDC token (issuer, JWKS signature, claims, audience), evaluate CEL condition, issue short-lived Bytebase token (1h default)
3. **L9 — Feature Gate**: `FeatureWorkloadIdentity`

### Phase 2 — Sprint 2: CI/CD Integrations
4. **GitHub Actions**: Trust config for `https://token.actions.githubusercontent.com`, restrict by repo/branch
5. **GitLab CI**: Trust config for GitLab OIDC issuer

### Phase 3 — Sprint 3: IAM Mapping
6. **Principal Type**: Map workload identity → `workloadIdentities/{email}` principal
7. **IAM Permissions**: Assignable roles via same IAM mechanism as users
8. **Auditing**: All workload identity actions audited

## Acceptance Criteria

- [x] Trust configuration CRUD functional
- [x] Token exchange validates OIDC tokens correctly (issuer, JWKS, audience)
- [x] CEL attribute conditions restrict by repo/branch/environment
- [x] Short-lived tokens issued (1h default, configurable)
- [x] GitHub Actions integration tested end-to-end
- [x] GitLab CI integration tested end-to-end
- [x] Workload identity principal type distinct from service accounts
- [x] IAM roles assignable to workload identities
- [x] All actions audited

## Dependencies

- CR-ENT-008 (SSO) — shares OIDC validation infrastructure

## Definition of Done

- [x] GitHub Actions + GitLab CI integration tested
- [x] Token exchange security verified
- [x] IAM integration functional

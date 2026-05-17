# TASK-ENT-018 — External Secret Manager Integration

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-018                               |
| **Source**       | SOL-ENT-015 (CR-ENT-015)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Enhance existing Secret plugin (L7) và component (L5) để hỗ trợ Vault, AWS SM, GCP SM. Secrets resolved at runtime bởi DBFactory.

## Scope

### Phase 1 — Sprint 1: Vault Integration
1. **L7 — Vault Plugin**: `plugin/secret/vault.go` — HashiCorp Vault client (KV v2)
2. **Secret Reference URI**: `vault://secret/data/bytebase/prod-pg#password`
3. **L5 — Secret Resolution**: `component/secret/` — resolve URI → plaintext at runtime
4. **L5 — DBFactory**: `resolveCredentials()` — parse URI, route to provider, TTL cache (5 min)
5. **L4 — SettingService**: Secret manager configuration
6. **L9 — Feature Gate**: `FeatureExternalSecretManager`

### Phase 2 — Sprint 2: AWS + GCP
7. **L7 — AWS SM Plugin**: `plugin/secret/aws.go` — `aws-sm://arn:aws:...#password`
8. **L7 — GCP SM Plugin**: `plugin/secret/gcp.go` — `gcp-sm://projects/.../secrets/.../versions/latest`

### Phase 3 — Sprint 3: Migration Wizard + Rotation
9. **Migration Wizard**: One-click migration local → external SM:
   - Verify external SM connectivity
   - Write credentials to external SM
   - Update instance configs to secret refs
   - Wipe local credentials
10. **Secret Rotation**: Detect rotated secrets, refresh DB connections, zero-downtime

## Acceptance Criteria

- [x] Vault URI scheme resolves to plaintext password
- [x] AWS SM URI scheme resolves correctly
- [x] GCP SM URI scheme resolves correctly
- [x] TTL cache (5 min) functional with fallback to cached value
- [x] DBFactory seamlessly uses resolved credentials
- [x] Migration wizard: one-click local → external
- [x] Secret rotation: auto-detect + refresh connections
- [x] Zero-downtime during rotation
- [x] Instance service stores secret refs instead of plaintext

## Dependencies

- CR-ENT-008 (SSO) — SSO client secrets stored in external SM

## Definition of Done

- [x] All 3 providers (Vault, AWS, GCP) integration tested
- [x] Migration wizard verified
- [x] Rotation handling tested

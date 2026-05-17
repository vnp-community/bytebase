# Change Request: Multi-Provider Vault Registry

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-002                                               |
| **Title**          | Multi-Provider Vault Registry                            |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-VLT-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **Multi-Provider Vault Registry** — hệ thống đăng ký và quản lý nhiều vault provider implementations. Hiện tại Bytebase hỗ trợ 4 providers (Vault KV V2, AWS SM, GCP SM, Azure KV), CR này mở rộng thêm **Vaultwarden/Bitwarden**, **CyberArk Conjur**, **Kubernetes Secrets**, và thiết kế pattern cho community providers.

### 1.2 Bối cảnh
VNPAY sử dụng **Vaultwarden** (self-hosted Bitwarden) làm password manager cho toàn bộ tổ chức. Cần tích hợp Bytebase với Vaultwarden để:
- Lưu DB credentials trong Vaultwarden
- Tận dụng access control của Vaultwarden (organizations, collections)
- Phù hợp với security policy hiện có của VNPAY

Ngoài ra, các doanh nghiệp khác có thể sử dụng CyberArk, K8s Secrets, hoặc các giải pháp khác → cần plugin registry.

### 1.3 Mục tiêu
- Plugin-based provider registration (theo mô hình DB driver plugin hiện tại)
- Implement 7 providers: Local, Vault KV V2, AWS SM, GCP SM, Azure KV, Vaultwarden, CyberArk
- Kubernetes Secrets integration cho cloud-native deployment
- Provider factory pattern cho extensibility

---

## 2. Yêu cầu chức năng

### FR-001: Provider Registry Pattern

Áp dụng cùng pattern với `plugin/db/` — providers tự register qua `init()`:

```go
// Registry singleton
var registry = &ProviderRegistry{
    factories: make(map[string]ProviderFactory),
}

type ProviderFactory func(config map[string]interface{}) (SecretProvider, error)

func Register(providerType string, factory ProviderFactory) {
    registry.factories[providerType] = factory
}

func NewProvider(providerType string, config map[string]interface{}) (SecretProvider, error) {
    factory, ok := registry.factories[providerType]
    if !ok {
        return nil, fmt.Errorf("unknown provider: %s", providerType)
    }
    return factory(config)
}
```

### FR-002: HashiCorp Vault KV V2 Provider (Refactor)

Refactor `vault.go` hiện tại thành SecretProvider interface:

| Config Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | ✅ | Vault server URL |
| `auth_type` | enum | ✅ | TOKEN, APPROLE, KUBERNETES, LDAP |
| `token` | string | conditional | Static token |
| `role_id` / `secret_id` | string | conditional | AppRole credentials |
| `k8s_role` | string | conditional | K8s auth role |
| `engine_name` | string | ✅ | KV engine mount path |
| `namespace` | string | ❌ | Vault namespace (Enterprise) |
| `tls_ca` | string | ❌ | CA cert PEM |
| `tls_cert` / `tls_key` | string | ❌ | Client cert mTLS |
| `skip_tls_verify` | bool | ❌ | Dev mode only |

**Thêm auth methods mới**:
- `KUBERNETES` — Pod-based auth cho K8s deployments
- `LDAP` — LDAP-based auth cho enterprise environments

### FR-003: Vaultwarden/Bitwarden Provider (NEW)

Tích hợp qua **Bitwarden API** (compatible với Vaultwarden):

| Config Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | ✅ | Vaultwarden server URL |
| `client_id` | string | ✅ | API key client ID |
| `client_secret` | string | ✅ | API key client secret |
| `organization_id` | string | ❌ | Organization ID for collection-based access |
| `collection_id` | string | ❌ | Collection to scope secrets |
| `device_type` | string | ❌ | Device identifier (default: "Bytebase") |

**Mapping logic**:
```
SecretRef.Path  → Bitwarden Item name (e.g., "bytebase/instances/pg-prod")
SecretRef.Key   → Bitwarden Item field name (e.g., "password")
SecretRef.Scope → Bitwarden Collection (per workspace)
```

**Implementation notes**:
- Sử dụng Bitwarden API v1 (REST): `/api/logins`, `/api/ciphers`
- Auth via API key flow: `POST /identity/connect/token` with `client_credentials`
- Support both Bitwarden Cloud và self-hosted Vaultwarden
- Cache decrypt key in memory (per session)

### FR-004: AWS Secrets Manager Provider (Refactor)

Refactor `aws.go` hiện tại:

| Config Field | Type | Required | Description |
|---|---|---|---|
| `region` | string | ✅ | AWS region |
| `access_key_id` | string | conditional | Static credentials |
| `secret_access_key` | string | conditional | Static credentials |
| `role_arn` | string | ❌ | IAM role to assume |
| `use_instance_profile` | bool | ❌ | Use EC2/ECS instance profile |
| `prefix` | string | ❌ | Secret name prefix (default: "bytebase/") |
| `kms_key_id` | string | ❌ | Custom KMS key for encryption |

**Thêm features mới**:
- `SetSecret` support (hiện tại chỉ có read)
- IAM role assumption cho cross-account access
- Instance profile auth cho EC2/ECS deployment

### FR-005: GCP Secret Manager Provider (Refactor)

Refactor `gcp.go` hiện tại:

| Config Field | Type | Required | Description |
|---|---|---|---|
| `project_id` | string | ✅ | GCP project ID |
| `credentials_json` | string | conditional | Service account key |
| `use_workload_identity` | bool | ❌ | Use GKE Workload Identity |
| `prefix` | string | ❌ | Secret name prefix (default: "bytebase-") |

### FR-006: Azure Key Vault Provider (Refactor)

Refactor `azure.go` hiện tại:

| Config Field | Type | Required | Description |
|---|---|---|---|
| `vault_url` | string | ✅ | Azure Key Vault URL |
| `tenant_id` | string | ✅ | Azure AD tenant |
| `client_id` | string | conditional | App registration client ID |
| `client_secret` | string | conditional | App registration secret |
| `use_managed_identity` | bool | ❌ | Use Azure Managed Identity |

### FR-007: CyberArk Conjur Provider (NEW)

| Config Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | ✅ | Conjur server URL |
| `account` | string | ✅ | Conjur account name |
| `login` | string | ✅ | Host or user identity |
| `api_key` | string | ✅ | API key |
| `ssl_cert` | string | ❌ | CA cert for self-signed |
| `policy_branch` | string | ❌ | Policy branch for secrets |

### FR-008: Kubernetes Secrets Provider (NEW)

| Config Field | Type | Required | Description |
|---|---|---|---|
| `namespace` | string | ✅ | K8s namespace |
| `label_selector` | string | ❌ | Label selector for secrets |
| `kubeconfig_path` | string | ❌ | Path to kubeconfig (dev mode) |
| `use_in_cluster` | bool | ❌ | Use in-cluster config (default: true) |

**Constraints**:
- Read-only by default (K8s secrets managed by external tools)
- Write requires RBAC: `create`, `update` on `secrets` resource
- Auto-detect namespace from pod environment

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| Provider Registry                  | `backend/component/secret/registry.go`               | New: factory pattern registry                |
| Vault KV V2 Provider              | `backend/component/secret/providers/vault_kv.go`     | Refactor from existing vault.go              |
| Vaultwarden Provider              | `backend/component/secret/providers/vaultwarden.go`  | New: Bitwarden API integration               |
| AWS SM Provider                   | `backend/component/secret/providers/aws_sm.go`       | Refactor from existing aws.go                |
| GCP SM Provider                   | `backend/component/secret/providers/gcp_sm.go`       | Refactor from existing gcp.go                |
| Azure KV Provider                 | `backend/component/secret/providers/azure_kv.go`     | Refactor from existing azure.go              |
| CyberArk Conjur Provider         | `backend/component/secret/providers/cyberark.go`     | New: Conjur REST API                         |
| K8s Secrets Provider              | `backend/component/secret/providers/k8s.go`          | New: client-go integration                   |
| Local Provider                    | `backend/component/secret/providers/local.go`        | New: wrap existing obfuscation               |
| Provider Tests                    | `backend/component/secret/providers/*_test.go`       | Unit tests cho mỗi provider                 |
| Proto: VaultConfig update         | `proto/store/store/setting.proto`                    | Update ProviderType enum                     |
| UI: Provider Config Form          | `frontend/src/views/Setting/VaultProvider.vue`       | Dynamic config form per provider type        |

### 3.1 Directory Structure

```
backend/component/secret/
├── provider.go          # SecretProvider interface (from CR-VLT-001)
├── manager.go           # SecretManager orchestrator (from CR-VLT-001)
├── registry.go          # ProviderRegistry + factory pattern
├── cache.go             # Secret caching layer
├── ref.go               # SecretRef types
├── metrics.go           # Prometheus metrics
├── providers/
│   ├── local.go         # XOR obfuscation (backward compatible)
│   ├── vault_kv.go      # HashiCorp Vault KV V2
│   ├── vaultwarden.go   # Vaultwarden/Bitwarden
│   ├── aws_sm.go        # AWS Secrets Manager
│   ├── gcp_sm.go        # GCP Secret Manager
│   ├── azure_kv.go      # Azure Key Vault
│   ├── cyberark.go      # CyberArk Conjur
│   ├── k8s.go           # Kubernetes Secrets
│   └── *_test.go        # Per-provider tests
├── secret.go            # Legacy compatibility wrapper (refactored)
└── vault.go             # DEPRECATED → moved to providers/vault_kv.go
```

### 3.2 Vaultwarden API Integration Detail

```go
// VaultwardenProvider implements SecretProvider via Bitwarden API
type VaultwardenProvider struct {
    baseURL     string
    accessToken string
    orgID       string
    collID      string
    httpClient  *http.Client
    mu          sync.RWMutex
    tokenExpiry time.Time
}

// Auth flow:
// POST /identity/connect/token
// Content-Type: application/x-www-form-urlencoded
// grant_type=client_credentials&client_id=...&client_secret=...&scope=api.organization

// GetSecret flow:
// 1. GET /api/ciphers?organizationId={orgID} — list items
// 2. Find item by name matching SecretRef.Path
// 3. Extract field matching SecretRef.Key from item.Fields[]
// 4. Return field value

// SetSecret flow:
// 1. Search existing item by name
// 2. If exists: PUT /api/ciphers/{id} — update field
// 3. If not: POST /api/ciphers — create new login item with custom fields
```

### 3.3 Provider Feature Matrix

| Feature | Local | Vault KV V2 | Vaultwarden | AWS SM | GCP SM | Azure KV | CyberArk | K8s |
|---|---|---|---|---|---|---|---|---|
| GetSecret | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| SetSecret | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| DeleteSecret | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| ListSecrets | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Versioning | ❌ | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Rotation | ❌ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ |
| mTLS | N/A | ✅ | ❌ | N/A | N/A | N/A | ✅ | N/A |
| RBAC | N/A | ✅ | ✅ (org) | ✅ (IAM) | ✅ (IAM) | ✅ (RBAC) | ✅ | ✅ |

⚠️ = Read-only by default, write requires explicit RBAC configuration

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Provider credentials bootstrap   | Initial provider config encrypted with auth_secret; after vault init, re-encrypt with vault-stored key |
| Vaultwarden API token expiry     | Auto-refresh token before expiry; mutex-protected refresh    |
| K8s secret access control        | Use ServiceAccount with minimal RBAC; namespaced access only |
| Provider config in UI            | Sensitive fields masked; never sent back to frontend after save |
| Cross-provider migration         | Secret-by-secret copy with verification; old provider kept until confirmed |
| Provider failure isolation       | Circuit breaker per provider; metrics alert on consecutive failures |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Register all 8 providers                              | All factories registered, no collision     |
| TC-002  | Create Vault KV V2 provider with AppRole auth         | Authenticated, health check passes        |
| TC-003  | Create Vaultwarden provider with API key              | Token obtained, list items works          |
| TC-004  | Vaultwarden GetSecret for existing item               | Correct field value returned              |
| TC-005  | Vaultwarden SetSecret creates new item                | Item created with correct fields          |
| TC-006  | AWS SM provider with IAM role assumption              | Cross-account secret access works         |
| TC-007  | K8s Secrets provider in-cluster                       | Read secrets from configured namespace    |
| TC-008  | Unknown provider type → error                        | Clear error message with available types   |
| TC-009  | Provider health check failure → circuit breaker       | Subsequent calls fast-fail until recovery |
| TC-010  | Concurrent provider initialization                    | Thread-safe, no race conditions           |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Registry pattern + Local provider refactor | Sprint 1       |
| Phase 2 | Vault KV V2 + AWS SM refactor              | Sprint 2       |
| Phase 3 | Vaultwarden + GCP SM + Azure KV            | Sprint 3       |
| Phase 4 | CyberArk + K8s Secrets                     | Sprint 4       |
| Phase 5 | UI config forms + integration testing      | Sprint 5       |

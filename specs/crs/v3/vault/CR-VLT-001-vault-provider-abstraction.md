# Change Request: Vault Provider Abstraction Layer

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-001                                               |
| **Title**          | Vault Provider Abstraction Layer                         |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế và triển khai **Vault Provider Abstraction Layer** — một interface chuẩn hóa cho tất cả các thao tác đọc/ghi secrets, thay thế pattern hiện tại đang hardcode từng provider trong `backend/component/secret/secret.go`. Layer này là nền tảng cho toàn bộ hệ thống vault management.

### 1.2 Bối cảnh
Hiện tại Bytebase sử dụng 2 cơ chế bảo vệ dữ liệu nhạy cảm:
1. **XOR Obfuscation** (`common.Obfuscate`) — dùng `auth_secret` làm seed, XOR với plaintext rồi base64 encode. Lưu trong PostgreSQL. **Không phải encryption** — có thể reverse nếu biết seed.
2. **External Secret** (`component/secret/`) — chỉ hỗ trợ cho DataSource passwords, switch/case 4 providers (Vault KV V2, AWS SM, GCP SM, Azure Key Vault).

**Vấn đề**:
- Obfuscation dùng XOR **không an toàn** cho production — seed lưu cùng DB, nếu DB bị compromise thì tất cả secrets bị lộ.
- External secret chỉ hỗ trợ **DataSource password** — không cover SMTP, IDP, AI API key, webhook tokens.
- Mỗi provider được implement riêng biệt, không có abstraction chung → khó thêm provider mới.
- Không có caching, connection pooling, hay health check cho vault connections.

### 1.3 Mục tiêu
- Unified `SecretProvider` interface cho tất cả vault operations
- Provider lifecycle management (init, health, close)
- Secret caching layer với configurable TTL
- Graceful degradation khi vault unreachable
- Foundation cho tất cả CR-VLT-* khác

---

## 2. Yêu cầu chức năng

### FR-001: SecretProvider Interface

Định nghĩa Go interface chuẩn cho vault providers:

```go
// SecretProvider defines the contract for external secret storage backends.
type SecretProvider interface {
    // Name returns the provider identifier (e.g., "vault-kv-v2", "aws-sm").
    Name() string
    
    // GetSecret retrieves a secret by path and key.
    // path: logical path (e.g., "bytebase/instances/pg-prod")
    // key: specific field within the secret (e.g., "password")
    GetSecret(ctx context.Context, path string, key string) (string, error)
    
    // SetSecret stores or updates a secret.
    SetSecret(ctx context.Context, path string, key string, value string) error
    
    // DeleteSecret removes a secret.
    DeleteSecret(ctx context.Context, path string, key string) error
    
    // ListSecrets returns all keys at a given path.
    ListSecrets(ctx context.Context, path string) ([]string, error)
    
    // Healthy performs a health check on the provider connection.
    Healthy(ctx context.Context) error
    
    // Close cleans up provider resources.
    Close() error
}
```

### FR-002: SecretManager Orchestrator

Central manager xử lý tất cả secret operations:

```go
type SecretManager struct {
    primary     SecretProvider          // Primary vault provider
    fallback    SecretProvider          // Optional fallback (e.g., local obfuscation)
    cache       *SecretCache           // In-memory LRU cache with TTL
    config      *SecretManagerConfig    // Runtime configuration
    metrics     *SecretMetrics         // Prometheus counters
}

// Resolve retrieves a secret, trying cache → primary → fallback.
func (m *SecretManager) Resolve(ctx context.Context, ref SecretRef) (string, error)

// Store persists a secret to the primary provider.
func (m *SecretManager) Store(ctx context.Context, ref SecretRef, value string) error
```

### FR-003: SecretRef — Unified Secret Reference

```go
// SecretRef identifies a secret across any provider.
type SecretRef struct {
    Category    SecretCategory  // DATASOURCE, SETTING, AUTH, IDP, WEBHOOK
    Scope       string          // "workspaces/{id}" or "instances/{id}"
    Path        string          // Logical path in vault
    Key         string          // Field name
    Version     int             // Optional version for rotation tracking
}

type SecretCategory int
const (
    SecretCategoryDataSource  SecretCategory = iota  // DB passwords, SSL certs
    SecretCategorySetting                             // SMTP, AI keys, IM tokens
    SecretCategoryAuth                                // JWT signing secret
    SecretCategoryIDP                                 // OAuth/SAML client secrets
    SecretCategoryWebhook                             // Webhook auth tokens
    SecretCategoryLicense                             // Enterprise license key
)
```

### FR-004: Secret Caching Layer

- LRU cache với configurable TTL (default: 5 phút)
- Cache invalidation khi secret được update
- Metrics: hit/miss ratio, cache size
- Cache bypass option cho security-critical reads

### FR-005: Configuration Model

```go
type SecretManagerConfig struct {
    // Provider type: "local", "vault-kv-v2", "aws-sm", "gcp-sm", "azure-kv",
    //                "vaultwarden", "cyberark", "k8s-secrets"
    ProviderType    string
    
    // Provider-specific configuration (JSONB in setting table)
    ProviderConfig  map[string]interface{}
    
    // Cache settings
    CacheTTL        time.Duration  // Default: 5m
    CacheMaxSize    int            // Default: 1024
    
    // Fallback behavior
    FallbackEnabled bool           // Default: true
    FallbackToLocal bool           // Use obfuscation as fallback
    
    // Namespace prefix for all secrets
    Namespace       string         // Default: "bytebase"
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| SecretProvider interface           | `backend/component/secret/provider.go`               | New: provider interface + types              |
| SecretManager                      | `backend/component/secret/manager.go`                | New: orchestrator with cache + fallback      |
| SecretRef types                    | `backend/component/secret/ref.go`                    | New: unified secret reference model          |
| SecretCache                        | `backend/component/secret/cache.go`                  | New: LRU cache with TTL for secrets          |
| LocalProvider (obfuscation)        | `backend/component/secret/local.go`                  | Refactor: wrap existing obfuscation as provider |
| Existing secret.go                 | `backend/component/secret/secret.go`                 | Refactor: delegate to SecretManager          |
| Proto: SecretManagerConfig         | `proto/store/store/setting.proto`                    | Add: workspace-level vault config            |
| Server bootstrap                   | `backend/server/server.go`                           | Add: SecretManager initialization at step 2.5 |
| Store injection                    | `backend/store/store.go`                             | Replace `Secret string` with `SecretManager` |
| Prometheus metrics                 | `backend/component/secret/metrics.go`                | New: vault operation metrics                 |

### 3.1 Proto Schema Changes

```protobuf
// In setting.proto — add to WorkspaceSetting
message VaultConfig {
  enum ProviderType {
    PROVIDER_UNSPECIFIED = 0;
    LOCAL = 1;           // Default: XOR obfuscation (backward compatible)
    VAULT_KV_V2 = 2;     // HashiCorp Vault KV V2
    AWS_SM = 3;          // AWS Secrets Manager
    GCP_SM = 4;          // GCP Secret Manager
    AZURE_KV = 5;        // Azure Key Vault
    VAULTWARDEN = 6;     // Vaultwarden (Bitwarden API)
    CYBERARK = 7;        // CyberArk Conjur
    K8S_SECRETS = 8;     // Kubernetes Secrets
  }
  ProviderType provider_type = 1;
  
  // Provider-specific config (JSON)
  string provider_config = 2;
  
  // Cache settings
  int32 cache_ttl_seconds = 3;     // Default: 300
  int32 cache_max_size = 4;        // Default: 1024
  
  // Fallback settings
  bool fallback_to_local = 5;      // Default: true
  
  // Namespace prefix
  string namespace = 6;            // Default: "bytebase"
}
```

### 3.2 Bootstrap Integration

```
Server Bootstrap (updated):
  └─ NewServer(ctx, profile)
       ├─ 1. StartMetadataInstance()
       ├─ 2. store.New(pgURL)
       ├─ 2.5. secretManager = secret.NewManager(store, vaultConfig) ← NEW
       ├─ 3. migrator.MigrateSchema()
       ├─ 4. enterprise.NewLicenseService()
       ...
```

### 3.3 Migration Path cho Existing Code

**Phase 1: Adapter Pattern**
```go
// Wrap existing ReplaceExternalSecret to use new SecretManager
func (m *SecretManager) ResolveDataSourceSecret(
    ctx context.Context, 
    secret string, 
    externalSecret *storepb.DataSourceExternalSecret,
) (string, error) {
    // If DataSource has external_secret configured (per-instance), use legacy path
    if externalSecret != nil {
        return m.legacyResolve(ctx, externalSecret)
    }
    // Otherwise, try workspace-level vault if configured
    if m.primary != nil {
        ref := SecretRef{Category: SecretCategoryDataSource, ...}
        return m.Resolve(ctx, ref)
    }
    // Fallback to obfuscation
    return secret, nil
}
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Vault credentials at rest        | Vault auth token/config encrypted with master key derived from auth_secret |
| Cache contains plaintext secrets | Cache lives in-process memory only, purged on shutdown        |
| Vault unavailability             | Configurable fallback to obfuscated local storage with alert  |
| Secret enumeration attack        | ListSecrets requires WORKSPACE_ADMIN permission               |
| Transit encryption               | All vault communications over TLS (enforced, configurable skip for dev) |
| Provider config exposure         | Provider config JSONB is itself stored encrypted in setting table |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Initialize SecretManager with LOCAL provider          | Fallback to obfuscation, functional       |
| TC-002  | Initialize SecretManager with Vault KV V2             | Connected, health check passes            |
| TC-003  | GetSecret with cache hit                              | Return cached value, no vault call        |
| TC-004  | GetSecret with cache miss                             | Call vault, cache result, return          |
| TC-005  | GetSecret when vault unreachable + fallback enabled   | Return obfuscated fallback value          |
| TC-006  | GetSecret when vault unreachable + fallback disabled  | Return error with vault connectivity msg  |
| TC-007  | SetSecret + immediate GetSecret                       | Cache invalidated, fresh value returned   |
| TC-008  | SecretManager.Close() → cache purged                  | All cached secrets wiped from memory      |
| TC-009  | Metrics: vault_secret_get_total counter               | Incremented on each GetSecret call        |
| TC-010  | Concurrent GetSecret for same key                     | Single-flight (no duplicate vault calls)  |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Interface definition + LocalProvider       | Sprint 1       |
| Phase 2 | SecretManager + Cache + Metrics            | Sprint 1-2     |
| Phase 3 | Refactor existing code to use SecretManager| Sprint 2-3     |
| Phase 4 | Proto changes + Setting UI                 | Sprint 3       |
| Phase 5 | Integration testing + documentation        | Sprint 4       |

# SOL-VLT-001: Vault Abstraction & Multi-Provider Registry

| Field | Value |
|---|---|
| **SOL ID** | SOL-VLT-001 |
| **CR References** | CR-VLT-001, CR-VLT-002 |
| **Layer** | L5 (Component) + L7 (Plugin pattern) |
| **Estimated Effort** | 8 sprints |

---

## 1. Phân tích hiện trạng

### 1.1 Code hiện tại

```
backend/component/secret/
├── secret.go    # ReplaceExternalSecret — switch/case 4 providers
├── vault.go     # HashiCorp Vault KV V2 (read-only, 123 lines)
├── aws.go       # AWS Secrets Manager (read-only, 57 lines)
├── gcp.go       # GCP Secret Manager (read-only, 57 lines)
└── azure.go     # Azure Key Vault (read-only, 53 lines)
```

**Vấn đề chính**:
- `ReplaceExternalSecret()` là switch/case hardcoded — thêm provider = sửa function
- Tất cả providers chỉ hỗ trợ **read** (`GetSecret`) — không có write/delete/list
- Mỗi provider tạo **client mới** mỗi lần gọi — không có connection pooling
- Chỉ hỗ trợ `DataSource.external_secret` — không cover Setting/Auth/IDP/Webhook

### 1.2 Pattern tham chiếu từ codebase

**Plugin DB Driver** (`plugin/db/`): 
```go
// Registration pattern (init-time)
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}
```

**Store caching** (`store/store.go`):
```go
settingCache *lru.Cache[string, *SettingMessage]  // 1,024 entries
```

**Server bootstrap** (`server/server.go`):
```
NewServer → store.New → migrator → enterprise → iam → webhook → dbfactory → runners
```

---

## 2. Giải pháp kiến trúc

### 2.1 Component Structure

```
backend/component/secret/
├── provider.go          # SecretProvider interface
├── manager.go           # SecretManager orchestrator
├── registry.go          # ProviderRegistry (factory pattern)
├── cache.go             # LRU cache with TTL
├── ref.go               # SecretRef, SecretCategory types
├── catalog.go           # SensitiveFieldDef registry (SOL-VLT-002)
├── metrics.go           # Prometheus metrics
├── secret.go            # Legacy wrapper (backward compatible)
├── providers/
│   ├── local.go         # Wrap existing obfuscation
│   ├── vault_kv.go      # Refactor from vault.go
│   ├── vaultwarden.go   # NEW: Bitwarden API
│   ├── aws_sm.go        # Refactor from aws.go
│   ├── gcp_sm.go        # Refactor from gcp.go
│   ├── azure_kv.go      # Refactor from azure.go
│   ├── cyberark.go      # NEW: Conjur REST API
│   └── k8s.go           # NEW: client-go
└── providers/*_test.go
```

### 2.2 Bootstrap Integration

```
NewServer(ctx, profile)
  ├─ 1. StartMetadataInstance()
  ├─ 2. store.New(pgURL)
  ├─ 2.5 ← secret.NewManager(store, vaultConfig)   ★ NEW
  ├─ 3. migrator.MigrateSchema()
  ├─ 4. enterprise.NewLicenseService()
  ├─ 5. iam.NewManager()
  ├─ 6. webhook.NewManager()
  ├─ 7. dbfactory.New(secretManager)                ★ MODIFIED: inject SecretManager
  ...
```

**Lý do step 2.5**: SecretManager cần `store` (để đọc config), nhưng phải init trước `dbfactory` (vì dbfactory cần resolve credentials).

---

## 3. Thiết kế chi tiết

### 3.1 SecretProvider Interface

```go
// provider.go
package secret

import "context"

// SecretProvider defines the contract for external secret storage backends.
// Follows the same pattern as plugin/db/Driver interface.
type SecretProvider interface {
    // Name returns provider identifier (e.g., "vault-kv-v2", "aws-sm").
    Name() string

    // GetSecret retrieves a secret value.
    GetSecret(ctx context.Context, path string, key string) (string, error)

    // SetSecret creates or updates a secret.
    SetSecret(ctx context.Context, path string, key string, value string) error

    // DeleteSecret removes a secret.
    DeleteSecret(ctx context.Context, path string, key string) error

    // ListSecrets returns all keys at a given path.
    ListSecrets(ctx context.Context, path string) ([]string, error)

    // Healthy performs provider health check.
    Healthy(ctx context.Context) error

    // Close releases provider resources.
    Close() error
}
```

### 3.2 Provider Registry

```go
// registry.go
package secret

import (
    "fmt"
    "sync"
)

// ProviderFactory creates a provider from config.
// Same pattern as db.Register() in plugin/db/.
type ProviderFactory func(config map[string]any) (SecretProvider, error)

type ProviderRegistry struct {
    mu        sync.RWMutex
    factories map[string]ProviderFactory
}

var globalRegistry = &ProviderRegistry{
    factories: make(map[string]ProviderFactory),
}

// Register is called from provider init() functions.
func Register(providerType string, factory ProviderFactory) {
    globalRegistry.mu.Lock()
    defer globalRegistry.mu.Unlock()
    if _, exists := globalRegistry.factories[providerType]; exists {
        panic(fmt.Sprintf("secret provider %q already registered", providerType))
    }
    globalRegistry.factories[providerType] = factory
}

// NewProvider creates a provider instance by type.
func NewProvider(providerType string, config map[string]any) (SecretProvider, error) {
    globalRegistry.mu.RLock()
    factory, ok := globalRegistry.factories[providerType]
    globalRegistry.mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("unknown secret provider: %s (available: %v)",
            providerType, globalRegistry.Available())
    }
    return factory(config)
}

// Available returns registered provider type names.
func (r *ProviderRegistry) Available() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.factories))
    for name := range r.factories {
        names = append(names, name)
    }
    return names
}
```

### 3.3 SecretManager Orchestrator

```go
// manager.go
package secret

import (
    "context"
    "sync"
    "time"

    lru "github.com/hashicorp/golang-lru/v2/expirable"
)

type SecretManager struct {
    primary   SecretProvider
    fallback  SecretProvider // LocalProvider (obfuscation)
    cache     *lru.LRU[string, string]
    config    *ManagerConfig
    mu        sync.RWMutex

    // singleflight for concurrent same-key requests
    inflight  map[string]*call
    inflightMu sync.Mutex
}

type ManagerConfig struct {
    ProviderType    string
    ProviderConfig  map[string]any
    CacheTTL        time.Duration // Default: 5m
    CacheMaxSize    int           // Default: 1024
    FallbackEnabled bool          // Default: true
    Namespace       string        // Default: "bytebase"
}

// NewManager creates SecretManager — called at bootstrap step 2.5.
func NewManager(store Store, config *ManagerConfig) (*SecretManager, error) {
    m := &SecretManager{
        config:   config,
        inflight: make(map[string]*call),
    }

    // Always create LocalProvider as fallback
    localProvider, err := NewProvider("local", map[string]any{
        "store": store,
    })
    if err != nil {
        return nil, fmt.Errorf("init local provider: %w", err)
    }
    m.fallback = localProvider

    // If vault provider configured, create it as primary
    if config.ProviderType != "" && config.ProviderType != "local" {
        primary, err := NewProvider(config.ProviderType, config.ProviderConfig)
        if err != nil {
            if config.FallbackEnabled {
                // Log warning, fallback to local
                slog.Warn("vault provider init failed, falling back to local",
                    "provider", config.ProviderType, "error", err)
                m.primary = localProvider
            } else {
                return nil, fmt.Errorf("init vault provider %s: %w", config.ProviderType, err)
            }
        } else {
            m.primary = primary
        }
    } else {
        m.primary = localProvider
    }

    // Init cache (same pattern as store.go LRU caches)
    cacheTTL := config.CacheTTL
    if cacheTTL == 0 {
        cacheTTL = 5 * time.Minute
    }
    cacheSize := config.CacheMaxSize
    if cacheSize == 0 {
        cacheSize = 1024
    }
    m.cache = lru.NewLRU[string, string](cacheSize, nil, cacheTTL)

    return m, nil
}

// Resolve retrieves a secret: cache → primary → fallback.
func (m *SecretManager) Resolve(ctx context.Context, ref SecretRef) (string, error) {
    cacheKey := ref.CacheKey()

    // 1. Check cache
    if val, ok := m.cache.Get(cacheKey); ok {
        secretCacheHits.Inc()
        return val, nil
    }
    secretCacheMisses.Inc()

    // 2. Singleflight: prevent duplicate vault calls for same key
    val, err := m.doResolve(ctx, cacheKey, ref)
    if err != nil {
        return "", err
    }

    // 3. Cache result
    m.cache.Add(cacheKey, val)
    return val, nil
}

// Store persists a secret and invalidates cache.
func (m *SecretManager) Store(ctx context.Context, ref SecretRef, value string) error {
    path := ref.VaultPath(m.config.Namespace)
    if err := m.primary.SetSecret(ctx, path, ref.Key, value); err != nil {
        return fmt.Errorf("store secret: %w", err)
    }
    // Invalidate cache
    m.cache.Remove(ref.CacheKey())
    return nil
}

// Primary returns the primary provider (for health checks).
func (m *SecretManager) Primary() SecretProvider {
    return m.primary
}

// Close releases all resources.
func (m *SecretManager) Close() error {
    m.cache.Purge()
    if m.primary != nil {
        m.primary.Close()
    }
    if m.fallback != nil && m.fallback != m.primary {
        m.fallback.Close()
    }
    return nil
}
```

### 3.4 SecretRef Types

```go
// ref.go
package secret

import "fmt"

type SecretCategory int

const (
    CategoryDataSource SecretCategory = iota
    CategorySetting
    CategoryAuth
    CategoryIDP
    CategoryWebhook
    CategoryLicense
)

func (c SecretCategory) String() string {
    switch c {
    case CategoryDataSource: return "datasource"
    case CategorySetting:    return "setting"
    case CategoryAuth:       return "auth"
    case CategoryIDP:        return "idp"
    case CategoryWebhook:    return "webhook"
    case CategoryLicense:    return "license"
    default:                 return "unknown"
    }
}

type SecretRef struct {
    Category SecretCategory
    Scope    string // "instances/{id}/datasources/{id}" or "workspaces/default"
    Key      string // "password", "ssl_key", etc.
    Version  int    // Optional, for rotation tracking
}

// VaultPath generates the full vault path.
// Convention: {namespace}/{category}/{scope}
func (r SecretRef) VaultPath(namespace string) string {
    return fmt.Sprintf("%s/%s/%s", namespace, r.Category.String(), r.Scope)
}

// CacheKey generates a unique cache key.
func (r SecretRef) CacheKey() string {
    return fmt.Sprintf("%s/%s/%s", r.Category.String(), r.Scope, r.Key)
}
```

### 3.5 Cache Implementation

```go
// cache.go — Thin wrapper using hashicorp/golang-lru/v2/expirable
// Reuses the same library already in use by Store (store.go line 155-158)
// No custom implementation needed — uses lru.NewLRU[string, string]()
// Cache bypass: Resolve() accepts WithBypassCache option for security-critical reads
```

### 3.6 LocalProvider (Wrap Existing Obfuscation)

```go
// providers/local.go
package providers

import (
    "context"
    "github.com/bytebase/bytebase/backend/common"
    "github.com/bytebase/bytebase/backend/component/secret"
)

func init() {
    secret.Register("local", func(config map[string]any) (secret.SecretProvider, error) {
        store, _ := config["store"].(secret.Store)
        return &LocalProvider{store: store}, nil
    })
}

type LocalProvider struct {
    store secret.Store
}

func (p *LocalProvider) Name() string { return "local" }

func (p *LocalProvider) GetSecret(ctx context.Context, path, key string) (string, error) {
    // Delegate to existing obfuscation: common.Unobfuscate()
    authSecret, err := p.store.GetAuthSecret(ctx)
    if err != nil {
        return "", err
    }
    // For local provider, path+key maps to the obfuscated value in DB
    // The actual deobfuscation is done in the store layer (instance.go)
    return common.Unobfuscate(path, authSecret)
}

func (p *LocalProvider) SetSecret(ctx context.Context, path, key, value string) error {
    // Local provider: obfuscate and store in DB (handled by store layer)
    return nil
}

func (p *LocalProvider) DeleteSecret(ctx context.Context, path, key string) error { return nil }
func (p *LocalProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
    return nil, nil
}
func (p *LocalProvider) Healthy(ctx context.Context) error { return nil }
func (p *LocalProvider) Close() error                      { return nil }
```

### 3.7 Vault KV V2 Provider (Refactor)

```go
// providers/vault_kv.go — Refactor from existing vault.go
package providers

import (
    "context"
    "sync"
    "github.com/hashicorp/vault/api"
    "github.com/bytebase/bytebase/backend/component/secret"
)

func init() {
    secret.Register("vault-kv-v2", func(config map[string]any) (secret.SecretProvider, error) {
        return newVaultKVProvider(config)
    })
}

type VaultKVProvider struct {
    client     *api.Client
    enginePath string
    mu         sync.RWMutex
}

func newVaultKVProvider(config map[string]any) (*VaultKVProvider, error) {
    // Parse config: url, auth_type, token/approle/k8s, engine_name, tls
    vaultConfig := api.DefaultConfig()
    vaultConfig.Address = config["url"].(string)

    // TLS configuration
    if caData, ok := config["tls_ca"].(string); ok {
        // Configure custom CA
        _ = caData // Apply to TLSConfig
    }

    client, err := api.NewClient(vaultConfig)
    if err != nil {
        return nil, err
    }

    // Auth based on auth_type
    authType, _ := config["auth_type"].(string)
    switch authType {
    case "TOKEN":
        client.SetToken(config["token"].(string))
    case "APPROLE":
        // Use AppRole auth: role_id + secret_id
        // POST /auth/approle/login
    case "KUBERNETES":
        // Use K8s JWT from /var/run/secrets/kubernetes.io/serviceaccount/token
        // POST /auth/kubernetes/login
    case "LDAP":
        // POST /auth/ldap/login/{username}
    }

    return &VaultKVProvider{
        client:     client,
        enginePath: config["engine_name"].(string),
    }, nil
}

func (p *VaultKVProvider) Name() string { return "vault-kv-v2" }

func (p *VaultKVProvider) GetSecret(ctx context.Context, path, key string) (string, error) {
    // KV V2 read: GET /v1/{engine}/data/{path}
    secretPath := fmt.Sprintf("%s/data/%s", p.enginePath, path)
    s, err := p.client.Logical().ReadWithContext(ctx, secretPath)
    if err != nil {
        return "", err
    }
    data, _ := s.Data["data"].(map[string]any)
    val, _ := data[key].(string)
    return val, nil
}

func (p *VaultKVProvider) SetSecret(ctx context.Context, path, key, value string) error {
    // KV V2 write: POST /v1/{engine}/data/{path}
    secretPath := fmt.Sprintf("%s/data/%s", p.enginePath, path)
    _, err := p.client.Logical().WriteWithContext(ctx, secretPath, map[string]any{
        "data": map[string]any{key: value},
    })
    return err
}

func (p *VaultKVProvider) DeleteSecret(ctx context.Context, path, key string) error {
    secretPath := fmt.Sprintf("%s/data/%s", p.enginePath, path)
    _, err := p.client.Logical().DeleteWithContext(ctx, secretPath)
    return err
}

func (p *VaultKVProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
    secretPath := fmt.Sprintf("%s/metadata/%s", p.enginePath, path)
    s, err := p.client.Logical().ListWithContext(ctx, secretPath)
    if err != nil {
        return nil, err
    }
    keys, _ := s.Data["keys"].([]any)
    result := make([]string, len(keys))
    for i, k := range keys {
        result[i], _ = k.(string)
    }
    return result, nil
}

func (p *VaultKVProvider) Healthy(ctx context.Context) error {
    _, err := p.client.Sys().HealthWithContext(ctx)
    return err
}

func (p *VaultKVProvider) Close() error {
    p.client.ClearToken()
    return nil
}
```

### 3.8 Vaultwarden/Bitwarden Provider (NEW)

```go
// providers/vaultwarden.go — Bitwarden REST API integration
package providers

func init() {
    secret.Register("vaultwarden", func(config map[string]any) (secret.SecretProvider, error) {
        return newVaultwardenProvider(config)
    })
}

type VaultwardenProvider struct {
    baseURL      string
    clientID     string
    clientSecret string
    orgID        string
    collID       string
    httpClient   *http.Client
    accessToken  string
    tokenExpiry  time.Time
    mu           sync.Mutex
}

// Auth flow: POST /identity/connect/token
//   grant_type=client_credentials
//   client_id={clientID}
//   client_secret={clientSecret}
//   scope=api.organization

// GetSecret flow:
//   1. Ensure valid access token (auto-refresh)
//   2. GET /api/ciphers/organization-details?organizationId={orgID}
//   3. Find cipher by name matching path
//   4. Extract custom field by key
//   5. Return value

// SetSecret flow:
//   1. Search for existing cipher by name
//   2. If exists: PUT /api/ciphers/{id} with updated fields
//   3. If not: POST /api/ciphers with new login item + custom fields

// Mapping:
//   SecretRef.Path  → Cipher.Name  (e.g., "bytebase/instances/pg-prod")
//   SecretRef.Key   → Cipher.Fields[].Name  (e.g., "password")
//   SecretRef.Scope → Cipher.CollectionIds[]
```

### 3.9 AWS/GCP/Azure Provider Refactoring

Mỗi provider hiện tại (aws.go, gcp.go, azure.go) được refactor từ **standalone function** thành **SecretProvider implementation**:

| Provider | Current Code | Refactored | Key Changes |
|---|---|---|---|
| AWS SM | `getSecretFromAWS()` 57 lines | `providers/aws_sm.go` | Add SetSecret, IAM role assumption, connection reuse |
| GCP SM | `getSecretFromGCP()` 57 lines | `providers/gcp_sm.go` | Add SetSecret, workload identity, client reuse |
| Azure KV | `getSecretFromAzure()` 53 lines | `providers/azure_kv.go` | Add SetSecret, managed identity, client reuse |

**Key refactoring pattern** (ví dụ AWS):
```go
// BEFORE: tạo client mỗi lần gọi
func getSecretFromAWS(ctx, externalSecret) (string, error) {
    cfg, _ := config.LoadDefaultConfig(ctx)
    client := secretsmanager.NewFromConfig(cfg)
    // ... read only
}

// AFTER: reuse client, support read + write
type AWSProvider struct {
    client *secretsmanager.Client  // Reused across calls
    prefix string
}
func (p *AWSProvider) GetSecret(ctx, path, key string) (string, error) { ... }
func (p *AWSProvider) SetSecret(ctx, path, key, value string) error { ... }
```

---

## 4. Proto Schema Changes

```protobuf
// setting.proto — Add to WorkspaceGeneralSetting or top-level setting
message VaultConfig {
    enum ProviderType {
        PROVIDER_UNSPECIFIED = 0;
        LOCAL = 1;
        VAULT_KV_V2 = 2;
        AWS_SM = 3;
        GCP_SM = 4;
        AZURE_KV = 5;
        VAULTWARDEN = 6;
        CYBERARK = 7;
        K8S_SECRETS = 8;
    }
    ProviderType provider_type = 1;
    string provider_config = 2;       // JSON string
    int32 cache_ttl_seconds = 3;      // Default: 300
    int32 cache_max_size = 4;         // Default: 1024
    bool fallback_to_local = 5;       // Default: true
    string namespace = 6;             // Default: "bytebase"
    VaultCategoryConfig categories = 7;
}

message VaultCategoryConfig {
    bool datasource_enabled = 1;
    bool setting_enabled = 2;
    bool auth_enabled = 3;
    bool idp_enabled = 4;
    bool webhook_enabled = 5;
}
```

**Storage**: `setting` table, `name = 'bb.workspace.vault'`, `value = VaultConfig JSON`.

---

## 5. Backward Compatibility

### 5.1 Legacy `ReplaceExternalSecret` Adapter

```go
// secret.go — Modified to delegate to SecretManager
func ReplaceExternalSecret(
    ctx context.Context,
    secretManager *SecretManager,
    secret string,
    externalSecret *storepb.DataSourceExternalSecret,
) (string, error) {
    // Priority 1: Per-instance external_secret (legacy config)
    if externalSecret != nil {
        return secretManager.legacyResolve(ctx, externalSecret)
    }
    // Priority 2: Workspace-level vault (new config)
    // Handled by caller via SecretManager.Resolve()

    // Priority 3: Return obfuscated value as-is
    return secret, nil
}
```

### 5.2 Feature Gate

```go
// Enterprise feature check — same pattern as existing feature gates
err := licenseService.IsFeatureEnabled(ctx, workspaceID,
    v1pb.PlanFeature_FEATURE_EXTERNAL_SECRET_MANAGER)
```

---

## 6. Metrics (Prometheus)

```go
// metrics.go — follow existing pattern from backend/component/secret/
var (
    secretOpsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytebase_secret_operations_total",
            Help: "Total secret operations by provider and operation",
        }, []string{"provider", "operation", "status"},
    )
    secretCacheHits = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_secret_cache_hits_total",
    })
    secretCacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_secret_cache_misses_total",
    })
    secretLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "bytebase_secret_operation_duration_seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
        }, []string{"provider", "operation"},
    )
)
```

---

## 7. Test Strategy

| Layer | Test Type | Tool |
|---|---|---|
| Unit: Provider interface | Mock provider, table-driven tests | Go testing |
| Unit: Registry | Register/resolve/unknown-provider | Go testing |
| Unit: Cache | Hit/miss/TTL/invalidation | Go testing |
| Integration: Vault KV V2 | testcontainers (Vault dev server) | testcontainers-go |
| Integration: AWS SM | LocalStack | testcontainers-go |
| Integration: Vaultwarden | Vaultwarden Docker image | testcontainers-go |
| E2E: Full flow | Bootstrap → configure → resolve | backend/tests/ |

---

## 8. Phụ thuộc mới (go.mod)

| Package | Purpose | Already in go.mod? |
|---|---|---|
| `github.com/hashicorp/vault/api` | Vault KV V2 client | ✅ Yes (existing vault.go) |
| `github.com/aws/aws-sdk-go-v2` | AWS SM client | ✅ Yes (existing aws.go) |
| `cloud.google.com/go/secretmanager` | GCP SM client | ✅ Yes (existing gcp.go) |
| `github.com/Azure/azure-sdk-for-go` | Azure KV client | ✅ Yes (existing azure.go) |
| `github.com/hashicorp/golang-lru/v2` | LRU cache | ✅ Yes (store.go) |
| `k8s.io/client-go` | K8s Secrets | ❌ New dependency |
| `github.com/cyberark/conjur-api-go` | CyberArk Conjur | ❌ New dependency |

> **Note**: Vaultwarden uses plain HTTP REST API — no additional SDK dependency needed.

# SOL-VLT-002: Sensitive Data Catalog & Service Integration

| Field | Value |
|---|---|
| **SOL ID** | SOL-VLT-002 |
| **CR Reference** | CR-VLT-003 |
| **Layer** | L5 (Component) + L4 (Service) + L8 (Store) |
| **Dependencies** | SOL-VLT-001 (SecretManager + Registry) |
| **Estimated Effort** | 6 sprints |

---

## 1. Phân tích hiện trạng

### 1.1 Sensitive Data Storage Patterns

**Pattern A — XOR Obfuscation** (DataSource fields):
```go
// store/instance.go line ~349
secret, _ := s.GetAuthSecret(ctx)
ds.ObfuscatedPassword = common.Obfuscate(ds.GetPassword(), secret)
ds.ObfuscatedSslCa = common.Obfuscate(ds.GetSslCa(), secret)
// ... 12 fields total
```

**Pattern B — Plaintext JSONB** (Setting fields):
```go
// setting_service.go — SMTP, AI, IDP configs stored directly in setting.value JSONB
// No obfuscation, no encryption
```

**Pattern C — Plaintext DB Column** (Auth):
```go
// store/server_config.go line 29
func (s *Store) GetAuthSecret(ctx context.Context) (string, error) {
    // Returns auth_secret from server_config table — plaintext
}
```

### 1.2 Sensitive Data Inventory (24 fields)

| # | Category | Field | Source File | Current Storage | Risk |
|---|---|---|---|---|---|
| 1-12 | DATASOURCE | password, ssl_ca, ssl_cert, ssl_key, ssh_password, ssh_private_key, auth_private_key, master_password, azure_client_secret, aws_secret_key, aws_session_token, gcp_content | `store/instance.go` | XOR obfuscation | CRITICAL |
| 13 | SETTING | smtp_password | `api/v1/setting_service.go` | Plaintext JSONB | HIGH |
| 14 | SETTING | ai_api_key | `api/v1/setting_service.go` | Plaintext JSONB | HIGH |
| 15 | SETTING | directory_sync_token | `api/v1/setting_service.go` | Plaintext JSONB | HIGH |
| 16 | AUTH | auth_secret | `store/server_config.go` | Plaintext column | CRITICAL |
| 17-19 | IDP | oidc_client_secret, saml_signing_key, ldap_bind_password | `api/v1/idp_service.go` | Plaintext JSONB | CRITICAL |
| 20-24 | WEBHOOK | slack_token, dingtalk_secret, feishu_secret, wecom_secret, teams_webhook_url | `component/webhook/manager.go` | Plaintext JSONB | MEDIUM |

---

## 2. Giải pháp

### 2.1 Sensitive Data Catalog Registry

```go
// catalog.go
package secret

type StorageMode int
const (
    StoragePlaintext   StorageMode = iota // No protection
    StorageObfuscated                      // XOR with auth_secret
    StorageVaultBacked                     // External vault
)

type RiskLevel int
const (
    RiskCritical RiskLevel = iota
    RiskHigh
    RiskMedium
)

type SensitiveFieldDef struct {
    Category    SecretCategory
    FieldName   string
    Source      string      // Package/struct origin
    CurrentMode StorageMode
    TargetMode  StorageMode
    Risk        RiskLevel
}

// Catalog — complete registry of all sensitive fields.
// Used by: migration scanner, compliance reporter, health dashboard.
var Catalog = []SensitiveFieldDef{
    // DataSource (12 fields) — currently XOR obfuscated
    {CategoryDataSource, "password", "DataSource", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "ssl_key", "DataSource", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "ssh_private_key", "DataSource", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "auth_private_key", "DataSource", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "azure_client_secret", "AzureCredential", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "aws_secret_key", "AWSCredential", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "gcp_content", "GCPCredential", StorageObfuscated, StorageVaultBacked, RiskCritical},
    {CategoryDataSource, "ssl_ca", "DataSource", StorageObfuscated, StorageVaultBacked, RiskHigh},
    {CategoryDataSource, "ssl_cert", "DataSource", StorageObfuscated, StorageVaultBacked, RiskHigh},
    {CategoryDataSource, "ssh_password", "DataSource", StorageObfuscated, StorageVaultBacked, RiskHigh},
    {CategoryDataSource, "master_password", "DataSource", StorageObfuscated, StorageVaultBacked, RiskHigh},
    {CategoryDataSource, "aws_session_token", "AWSCredential", StorageObfuscated, StorageVaultBacked, RiskHigh},
    // Setting (3 fields) — currently plaintext JSONB
    {CategorySetting, "smtp_password", "Setting.SMTPConfig", StoragePlaintext, StorageVaultBacked, RiskHigh},
    {CategorySetting, "ai_api_key", "Setting.AIConfig", StoragePlaintext, StorageVaultBacked, RiskHigh},
    {CategorySetting, "directory_sync_token", "Setting.AuthSetting", StoragePlaintext, StorageVaultBacked, RiskHigh},
    // Auth (1 field) — currently plaintext column
    {CategoryAuth, "auth_secret", "server_config", StoragePlaintext, StorageVaultBacked, RiskCritical},
    // IDP (3 fields) — currently plaintext JSONB
    {CategoryIDP, "oidc_client_secret", "IdentityProviderConfig", StoragePlaintext, StorageVaultBacked, RiskCritical},
    {CategoryIDP, "saml_signing_key", "IdentityProviderConfig", StoragePlaintext, StorageVaultBacked, RiskCritical},
    {CategoryIDP, "ldap_bind_password", "IdentityProviderConfig", StoragePlaintext, StorageVaultBacked, RiskHigh},
    // Webhook (5 fields) — currently plaintext JSONB
    {CategoryWebhook, "slack_token", "IMWebhook", StoragePlaintext, StorageVaultBacked, RiskMedium},
    {CategoryWebhook, "dingtalk_secret", "IMWebhook", StoragePlaintext, StorageVaultBacked, RiskMedium},
    {CategoryWebhook, "feishu_secret", "IMWebhook", StoragePlaintext, StorageVaultBacked, RiskMedium},
    {CategoryWebhook, "wecom_secret", "IMWebhook", StoragePlaintext, StorageVaultBacked, RiskMedium},
    {CategoryWebhook, "teams_webhook_url", "IMWebhook", StoragePlaintext, StorageVaultBacked, RiskMedium},
}
```

### 2.2 Vault Path Convention

```
{namespace}/{category}/{scope}/{key}

Ví dụ:
  bytebase/datasource/instances/pg-prod/admin/password
  bytebase/setting/workspaces/default/smtp_password
  bytebase/auth/workspaces/default/auth_secret
  bytebase/idp/workspaces/default/oidc_client_secret
  bytebase/webhook/workspaces/default/slack_token
```

### 2.3 Service Layer Integration Points

#### 2.3.1 Store Layer — `instance.go` Refactor

```go
// BEFORE (instance.go ~line 349):
secret, _ := s.GetAuthSecret(ctx)
ds.ObfuscatedPassword = common.Obfuscate(ds.GetPassword(), secret)
ds.ObfuscatedSslCa = common.Obfuscate(ds.GetSslCa(), secret)
// ... 12 more fields manually obfuscated

// AFTER:
if err := s.secretManager.ObfuscateDataSource(ctx, instanceID, ds); err != nil {
    return nil, err
}
// SecretManager internally routes to vault or local obfuscation based on config
```

#### 2.3.2 DBFactory — Credential Resolution

```go
// component/dbfactory/factory.go
// BEFORE: dbfactory receives password directly from store
// AFTER: dbfactory calls SecretManager to resolve credentials

func (f *Factory) GetDB(ctx context.Context, instance, dataSource) (*sql.DB, error) {
    // Resolve password via SecretManager
    password, err := f.secretManager.Resolve(ctx, SecretRef{
        Category: CategoryDataSource,
        Scope:    fmt.Sprintf("instances/%s/datasources/%s", instance.ResourceID, dataSource.Id),
        Key:      "password",
    })
    // ... use resolved password for connection
}
```

#### 2.3.3 Setting Service — SMTP/AI/Token

```go
// api/v1/setting_service.go — SetSetting handler
// When saving sensitive setting values:

func (s *SettingService) handleSensitiveSetting(ctx context.Context, setting) error {
    if !s.secretManager.IsCategoryEnabled(CategorySetting) {
        return nil // Vault not enabled for settings, use existing plaintext path
    }

    // Example: SMTP password
    if smtp := setting.GetEmailConfig().GetSmtp(); smtp != nil && smtp.Password != "" {
        ref := SecretRef{
            Category: CategorySetting,
            Scope:    "workspaces/default",
            Key:      "smtp_password",
        }
        if err := s.secretManager.Store(ctx, ref, smtp.Password); err != nil {
            return err
        }
        smtp.Password = ""    // Clear plaintext before DB write
        // Store VaultRef marker in JSONB
    }
    return nil
}
```

#### 2.3.4 IDP Service

```go
// api/v1/idp_service.go
// When creating/updating identity providers:
// Route client_secret through SecretManager if IDP category enabled
```

#### 2.3.5 Webhook Manager

```go
// component/webhook/manager.go
// When loading IM webhook configs:
// Resolve tokens from SecretManager if webhook category enabled
```

### 2.4 Proto Changes

```protobuf
// setting.proto
message VaultRef {
    string path = 1;     // Vault path
    string key = 2;      // Field key
    int32 version = 3;   // Rotation version tracking
}

// Usage: fields that have been migrated to vault store a VaultRef
// instead of the actual value. The service layer checks for VaultRef
// presence and resolves via SecretManager.
```

### 2.5 Sensitive Data Discovery Scanner

```go
// runner/secretscan/scanner.go — Background runner (L6)
// Same pattern as runner/cleaner/cleaner.go

type Scanner struct {
    store         *store.Store
    secretManager *SecretManager
    interval      time.Duration // 24h
}

func (s *Scanner) Run(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:  s.scan(ctx)
        }
    }
}

func (s *Scanner) scan(ctx context.Context) *ScanResult {
    result := &ScanResult{}
    for _, field := range Catalog {
        // Count instances of each field type
        // Check if vault-backed or still plaintext/obfuscated
        // Report findings
    }
    // Emit Prometheus metrics
    return result
}
```

---

## 3. Security: Auth Secret Bootstrap Problem

`auth_secret` presents a chicken-and-egg problem: it's needed to decrypt vault config, but it should itself be stored in vault.

**Solution**: Two-phase bootstrap:

```
Phase 1 (Initial startup):
  auth_secret stays in server_config table (plaintext)
  Used to sign JWTs and decrypt vault provider_config

Phase 2 (After vault configured):
  auth_secret copied to vault
  server_config.auth_secret replaced with vault reference marker
  On boot: read vault config → init vault → read auth_secret from vault
  Fallback: if vault unavailable, use local auth_secret
```

---

## 4. Data Flow: Before vs After

```
BEFORE:
  User → InstanceService → Store → DB (obfuscated password)
  DBFactory → Store.GetDataSource() → common.Unobfuscate() → password

AFTER:
  User → InstanceService → SecretManager.Store(ref, password) → Vault
                          → Store → DB (VaultRef marker, no password)
  DBFactory → SecretManager.Resolve(ref) → Cache/Vault → password
```

---

## 5. Rollout Strategy (Category-Level Opt-In)

| Phase | Category | Default State | Risk |
|---|---|---|---|
| Phase 1 | DataSource | Opt-in (vault_category.datasource_enabled) | LOW — existing external_secret path proven |
| Phase 2 | Setting | Opt-in | LOW — 3 fields only |
| Phase 3 | IDP | Opt-in | MEDIUM — SSO disruption if vault fails |
| Phase 4 | Webhook | Opt-in | LOW — non-critical notifications |
| Phase 5 | Auth | Opt-in (last) | HIGH — JWT signing disruption if vault fails |

Each category can be independently enabled/disabled via `VaultCategoryConfig` in workspace settings.

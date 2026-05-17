# Change Request: Platform-Wide Sensitive Data Catalog

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-VLT-003                                               |
| **Title**          | Platform-Wide Sensitive Data Catalog                     |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-VLT-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Xác định và catalog **tất cả dữ liệu nhạy cảm** trong Bytebase platform cần được migrate sang vault-backed storage. Hiện tại chỉ có DataSource passwords hỗ trợ external secret — CR này mở rộng coverage sang **toàn bộ** sensitive data categories.

### 1.2 Bối cảnh
Qua phân tích source code, có ít nhất **11 loại dữ liệu nhạy cảm** đang lưu trữ không an toàn:

| # | Dữ liệu | Cách lưu hiện tại | Rủi ro |
|---|---|---|---|
| 1 | DB passwords | XOR obfuscation (`Obfuscate()`) | XOR reversible nếu biết seed |
| 2 | SSL private keys | XOR obfuscation | Key material exposure |
| 3 | SSH passwords/keys | XOR obfuscation | Lateral movement risk |
| 4 | SMTP password | Plaintext JSONB (`SMTPConfig.password`) | Email system compromise |
| 5 | IDP client secrets | Plaintext JSONB (`IdentityProviderConfig`) | SSO bypass attack |
| 6 | AI API keys | Plaintext JSONB (`AIConfig.api_key`) | API abuse, billing fraud |
| 7 | Webhook tokens | Plaintext JSONB | Webhook impersonation |
| 8 | Auth secret (JWT) | Plaintext (`server_config.auth_secret`) | Token forgery |
| 9 | IM notification secrets | Plaintext JSONB (Slack/DingTalk/Feishu tokens) | Notification spam |
| 10 | Directory sync token | Plaintext (`directory_sync_token`) | SCIM impersonation |
| 11 | Azure/AWS/GCP credentials | XOR obfuscation | Cloud account compromise |

### 1.3 Mục tiêu
- Formal catalog của tất cả sensitive data với vault mapping
- `SecretRef` convention cho mỗi loại data
- Adapter layer để bridge existing code → SecretManager
- Feature flag cho progressive rollout per category

---

## 2. Yêu cầu chức năng

### FR-001: Sensitive Data Registry

Định nghĩa registry cho tất cả sensitive fields:

```go
// SensitiveFieldDef describes a sensitive field that should be vault-backed.
type SensitiveFieldDef struct {
    Category    SecretCategory
    FieldName   string          // e.g., "password", "ssl_key"
    Source      string          // e.g., "DataSource", "Setting.SMTPConfig"
    CurrentMode StorageMode     // OBFUSCATED, PLAINTEXT, HASHED
    TargetMode  StorageMode     // VAULT_BACKED
    Risk        RiskLevel       // CRITICAL, HIGH, MEDIUM
    Proto       string          // Proto field path
}

var SensitiveFields = []SensitiveFieldDef{
    // Category: DATASOURCE (already partially supported)
    {SecretCategoryDataSource, "password",       "DataSource",        OBFUSCATED, VAULT_BACKED, CRITICAL, "DataSource.password"},
    {SecretCategoryDataSource, "ssl_ca",         "DataSource",        OBFUSCATED, VAULT_BACKED, HIGH,     "DataSource.ssl_ca"},
    {SecretCategoryDataSource, "ssl_cert",       "DataSource",        OBFUSCATED, VAULT_BACKED, HIGH,     "DataSource.ssl_cert"},
    {SecretCategoryDataSource, "ssl_key",        "DataSource",        OBFUSCATED, VAULT_BACKED, CRITICAL, "DataSource.ssl_key"},
    {SecretCategoryDataSource, "ssh_password",   "DataSource",        OBFUSCATED, VAULT_BACKED, HIGH,     "DataSource.ssh_password"},
    {SecretCategoryDataSource, "ssh_private_key","DataSource",        OBFUSCATED, VAULT_BACKED, CRITICAL, "DataSource.ssh_private_key"},
    {SecretCategoryDataSource, "auth_private_key","DataSource",       OBFUSCATED, VAULT_BACKED, CRITICAL, "DataSource.authentication_private_key"},
    {SecretCategoryDataSource, "master_password","DataSource",        OBFUSCATED, VAULT_BACKED, HIGH,     "DataSource.master_password"},
    {SecretCategoryDataSource, "azure_client_secret","DataSource.AzureCredential", OBFUSCATED, VAULT_BACKED, CRITICAL, "AzureCredential.client_secret"},
    {SecretCategoryDataSource, "aws_secret_key", "DataSource.AWSCredential",      OBFUSCATED, VAULT_BACKED, CRITICAL, "AWSCredential.secret_access_key"},
    {SecretCategoryDataSource, "aws_session_token","DataSource.AWSCredential",    OBFUSCATED, VAULT_BACKED, HIGH,     "AWSCredential.session_token"},
    {SecretCategoryDataSource, "gcp_content",    "DataSource.GCPCredential",      OBFUSCATED, VAULT_BACKED, CRITICAL, "GCPCredential.content"},
    
    // Category: SETTING (currently plaintext)
    {SecretCategorySetting, "smtp_password",     "Setting.SMTPConfig",            PLAINTEXT,  VAULT_BACKED, HIGH,     "EmailConfig.SMTPConfig.password"},
    {SecretCategorySetting, "ai_api_key",        "Setting.AIConfig",              PLAINTEXT,  VAULT_BACKED, HIGH,     "AIConfig.api_key"},
    {SecretCategorySetting, "directory_sync_token","Setting.AuthSetting",          PLAINTEXT,  VAULT_BACKED, HIGH,     "AuthSetting.directory_sync_token"},
    
    // Category: AUTH (currently plaintext)
    {SecretCategoryAuth, "auth_secret",          "server_config",                 PLAINTEXT,  VAULT_BACKED, CRITICAL, "server_config.auth_secret"},
    
    // Category: IDP (currently plaintext JSONB)
    {SecretCategoryIDP, "oidc_client_secret",    "Setting.IdentityProvider",      PLAINTEXT,  VAULT_BACKED, CRITICAL, "IdentityProviderConfig.OAuth2Config.client_secret"},
    {SecretCategoryIDP, "saml_signing_key",      "Setting.IdentityProvider",      PLAINTEXT,  VAULT_BACKED, CRITICAL, "IdentityProviderConfig.SAMLConfig"},
    {SecretCategoryIDP, "ldap_bind_password",    "Setting.IdentityProvider",      PLAINTEXT,  VAULT_BACKED, HIGH,     "IdentityProviderConfig.LDAPConfig"},
    
    // Category: WEBHOOK (currently plaintext JSONB)
    {SecretCategoryWebhook, "slack_token",       "Setting.IMWebhook",             PLAINTEXT,  VAULT_BACKED, MEDIUM,   "IMWebhook.SlackConfig.token"},
    {SecretCategoryWebhook, "dingtalk_secret",   "Setting.IMWebhook",             PLAINTEXT,  VAULT_BACKED, MEDIUM,   "IMWebhook.DingTalkConfig.app_secret"},
    {SecretCategoryWebhook, "feishu_secret",     "Setting.IMWebhook",             PLAINTEXT,  VAULT_BACKED, MEDIUM,   "IMWebhook.FeishuConfig.secret"},
    {SecretCategoryWebhook, "wecom_secret",      "Setting.IMWebhook",             PLAINTEXT,  VAULT_BACKED, MEDIUM,   "IMWebhook.WeComConfig.client_secret"},
    {SecretCategoryWebhook, "teams_webhook_url", "Setting.IMWebhook",             PLAINTEXT,  VAULT_BACKED, MEDIUM,   "IMWebhook.TeamsConfig"},
}
```

### FR-002: Vault Path Convention

Mỗi sensitive field được map sang vault path theo convention:

```
{namespace}/{category}/{scope}/{field_name}

Ví dụ:
  bytebase/datasource/instances/pg-prod/admin/password
  bytebase/datasource/instances/pg-prod/admin/ssl_key
  bytebase/setting/workspaces/default/smtp_password
  bytebase/auth/workspaces/default/auth_secret
  bytebase/idp/workspaces/default/oidc_client_secret
  bytebase/webhook/workspaces/default/slack_token
```

### FR-003: Category-Level Feature Flags

Cho phép bật/tắt vault backing theo từng category:

```go
type VaultCategoryConfig struct {
    DataSourceEnabled  bool  // Default: true (backward compatible)
    SettingEnabled     bool  // Default: false (opt-in)
    AuthEnabled        bool  // Default: false (opt-in)
    IDPEnabled         bool  // Default: false (opt-in)
    WebhookEnabled     bool  // Default: false (opt-in)
    LicenseEnabled     bool  // Default: false (opt-in)
}
```

### FR-004: Setting Service Vault Integration

Modify `SettingService` để route sensitive fields qua SecretManager:

```go
// In setting_service.go — SetSetting handler
func (s *SettingService) SetSetting(ctx context.Context, req *v1pb.SetSettingRequest) (*v1pb.Setting, error) {
    // ... existing logic ...
    
    // NEW: If vault enabled for this setting category, store secret in vault
    if s.secretManager.IsCategoryEnabled(SecretCategorySetting) {
        if smtpConfig := setting.GetEmailConfig().GetSmtp(); smtpConfig != nil && smtpConfig.Password != "" {
            ref := SecretRef{
                Category: SecretCategorySetting,
                Scope:    fmt.Sprintf("workspaces/%s", workspace),
                Key:      "smtp_password",
            }
            if err := s.secretManager.Store(ctx, ref, smtpConfig.Password); err != nil {
                return nil, err
            }
            smtpConfig.Password = ""  // Clear plaintext before DB write
            smtpConfig.VaultRef = ref.ToProto()  // Store reference
        }
    }
    // ... save to store ...
}
```

### FR-005: Instance Service Vault Integration Enhancement

Enhance existing DataSource handling:

```go
// In instance_service.go — when creating/updating DataSource
// Current: each DataSource has its own external_secret config (per-instance)
// New: if workspace-level vault is configured, auto-use it unless overridden

func (s *InstanceService) resolveDataSourceCredentials(ctx context.Context, ds *storepb.DataSource) error {
    // Priority: per-instance external_secret > workspace vault > obfuscation
    if ds.ExternalSecret != nil {
        // Legacy per-instance config — keep backward compatibility
        return s.legacyResolve(ctx, ds)
    }
    
    if s.secretManager.IsCategoryEnabled(SecretCategoryDataSource) {
        // Use workspace vault
        ref := SecretRef{
            Category: SecretCategoryDataSource,
            Scope:    fmt.Sprintf("instances/%s/datasources/%s", instanceID, ds.Id),
            Key:      "password",
        }
        password, err := s.secretManager.Resolve(ctx, ref)
        if err != nil {
            return err
        }
        ds.Password = password
        return nil
    }
    
    // Fallback: existing obfuscation
    return s.deobfuscate(ctx, ds)
}
```

### FR-006: Sensitive Data Discovery Scanner

Runner tự động scan PostgreSQL metadata để phát hiện sensitive data chưa vault-backed:

```go
type SensitiveDataScanner struct {
    store         *store.Store
    secretManager *SecretManager
    interval      time.Duration  // Default: 24h
}

// ScanResult reports which sensitive fields are not yet vault-backed
type ScanResult struct {
    TotalFields     int
    VaultBacked     int
    Obfuscated      int
    Plaintext       int
    Details         []FieldStatus
}

type FieldStatus struct {
    Def     SensitiveFieldDef
    Count   int           // Number of instances
    Mode    StorageMode   // Current storage mode
    Risk    RiskLevel
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| Sensitive Field Registry           | `backend/component/secret/catalog.go`                | New: field definitions + vault path mapping  |
| Setting Service integration        | `backend/api/v1/setting_service.go`                  | Modify: route secrets through SecretManager  |
| Instance Service enhancement       | `backend/api/v1/instance_service.go`                 | Modify: workspace vault priority logic       |
| Auth Service integration           | `backend/api/v1/auth_service.go`                     | Modify: JWT secret from vault                |
| IDP Service integration            | `backend/api/v1/idp_service.go`                      | Modify: client secrets via vault             |
| Webhook Manager integration        | `backend/component/webhook/manager.go`               | Modify: IM tokens from vault                 |
| DBFactory integration              | `backend/component/dbfactory/factory.go`             | Modify: resolve credentials via SecretManager|
| Sensitive Data Scanner             | `backend/runner/secretscan/scanner.go`               | New: periodic scanner runner                 |
| Proto: VaultRef field              | `proto/store/store/setting.proto`                    | Add: VaultRef message for referencing secrets|
| Proto: VaultCategoryConfig         | `proto/store/store/setting.proto`                    | Add: per-category enable/disable             |
| Store: deobfuscation refactor      | `backend/store/instance.go`                          | Modify: delegate to SecretManager            |
| Store: runner_queries refactor     | `backend/store/runner_queries.go`                    | Modify: batch deobfuscation via SecretManager|

### 3.1 Proto Changes

```protobuf
// Add to setting.proto
message VaultRef {
    string path = 1;      // Vault path (e.g., "bytebase/datasource/instances/pg-prod/admin")
    string key = 2;       // Field key (e.g., "password")
    int32 version = 3;    // Secret version (for rotation tracking)
}

// Add to VaultConfig
message VaultCategoryConfig {
    bool datasource_enabled = 1;    // Default: true
    bool setting_enabled = 2;       // Default: false
    bool auth_enabled = 3;          // Default: false
    bool idp_enabled = 4;           // Default: false
    bool webhook_enabled = 5;       // Default: false
}
```

### 3.2 Store Layer Changes

Current `instance.go` obfuscation flow:
```go
// BEFORE: 40+ lines of manual obfuscation per DataSource
secret, _ := s.GetAuthSecret(ctx)
ds.ObfuscatedPassword = common.Obfuscate(ds.GetPassword(), secret)
ds.ObfuscatedSslCa = common.Obfuscate(ds.GetSslCa(), secret)
// ... 12 more fields ...
```

Proposed refactor:
```go
// AFTER: delegate to SecretManager
if err := s.secretManager.ObfuscateDataSource(ctx, ds); err != nil {
    return nil, err
}
// SecretManager internally routes to vault or local obfuscation based on config
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Auth secret in vault = chicken-egg | Bootstrap auth_secret locally first, then migrate to vault; or use env var for initial auth |
| Plaintext → vault migration window | Migration runs in background with encryption; old plaintext overwritten with vault ref |
| Category-level access control    | Each category maps to different vault policy/path; principle of least privilege |
| Scanner false positives          | Only flag fields in SensitiveFields registry; ignore user-created data |
| Batch operations performance     | SecretManager caches resolved secrets; batch resolve API for runner_queries |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Catalog lists all 24 sensitive fields                 | Complete registry, no missing fields       |
| TC-002  | SMTP password stored via vault when enabled           | Password in vault, VaultRef in setting DB  |
| TC-003  | SMTP password stored plaintext when vault disabled    | Backward compatible behavior               |
| TC-004  | AI API key routed to vault                            | Key in vault, cleared from JSONB           |
| TC-005  | IDP client secret stored in vault                     | Secret in vault, cleared from JSONB        |
| TC-006  | Scanner detects 5 plaintext fields                    | Report: 5 plaintext, risk=HIGH+CRITICAL   |
| TC-007  | Scanner reports 100% vault-backed                     | Clean scan, all fields vault-backed        |
| TC-008  | Instance DataSource: workspace vault priority         | Workspace vault used over obfuscation      |
| TC-009  | Instance DataSource: per-instance override            | Per-instance external_secret takes priority|
| TC-010  | Auth secret bootstrap without vault                   | Falls back to local storage on first start |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Catalog definition + VaultRef proto        | Sprint 1       |
| Phase 2 | DataSource enhanced integration            | Sprint 2       |
| Phase 3 | Setting/IDP/Webhook integration            | Sprint 3-4     |
| Phase 4 | Auth secret migration + Scanner            | Sprint 5       |
| Phase 5 | UI dashboard + coverage reporting          | Sprint 6       |

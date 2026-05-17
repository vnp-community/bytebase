# Change Request: Multi-Platform Sharing Federation

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-006                                                   |
| **Title**          | Multi-Platform Sharing Federation                            |
| **Priority**       | P2 — Medium                                                  |
| **Status**         | Draft                                                        |
| **PRD Refs**       | SEC-18                                                       |
| **Arch Layers**    | L7 (Plugin)                                                  |
| **Dependencies**   | CR-SHR-001, CR-SHR-002                                       |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Mở rộng sharing layer để hỗ trợ nhiều nền tảng chia sẻ ngoài Vaultwarden, bao gồm **HashiCorp Vault Transit** (envelope encryption + response wrapping), **1Password Connect** (item sharing via Connect Server), và **CyberArk Conjur** (secret retrieval tokens). Cho phép workspace admin chọn provider phù hợp với infrastructure hiện có.

---

## 2. Supported Providers

| Provider | Sharing Mechanism | Use Case |
|---|---|---|
| Vaultwarden (CR-SHR-002) | Bitwarden Send API | Default, self-hosted |
| HashiCorp Vault | Response Wrapping (cubbyhole) | Enterprise, existing Vault infra |
| 1Password Connect | Item sharing via Connect Server | Teams using 1Password |
| CyberArk Conjur | Temporary secret retrieval token | Large enterprise |
| Azure Key Vault | Managed secret + SAS URL | Azure-native deployments |

---

## 3. Provider Implementations

### 3.1 HashiCorp Vault — Response Wrapping

```go
// backend/plugin/sharing/vault_transit/provider.go
func init() {
    sharing.Register("vault_transit", func(config *sharing.ProviderConfig) (sharing.SharingProvider, error) {
        return NewVaultTransitProvider(config)
    })
}

// Uses Vault's Response Wrapping (cubbyhole)
// POST /v1/sys/wrapping/wrap → wrapping_token (single-use, TTL)
// Recipient: POST /v1/sys/wrapping/unwrap (token) → credential
```

### 3.2 1Password Connect

```go
// backend/plugin/sharing/onepassword/provider.go
func init() {
    sharing.Register("1password", func(config *sharing.ProviderConfig) (sharing.SharingProvider, error) {
        return NewOnePasswordProvider(config)
    })
}

// Uses 1Password Connect Server API
// POST /v1/vaults/{vault_id}/items → create item
// Generate access link via 1Password sharing URL
```

### 3.3 Provider Selection

```yaml
# Workspace Setting — provider routing
sharing:
  default_provider: "vaultwarden"
  providers:
    vaultwarden:
      enabled: true
      endpoint: "https://vault.internal.com"
    vault_transit:
      enabled: true
      endpoint: "https://vault.hashicorp.internal:8200"
      mount_path: "transit"
    onepassword:
      enabled: false
  
  # Routing rules — different providers for different credential types
  routing:
    - credential_type: "ssl_cert"
      provider: "vault_transit"    # Vault for PKI
    - credential_type: "password"
      provider: "vaultwarden"      # Vaultwarden for passwords
    - credential_type: "*"
      provider: "vaultwarden"      # Default
```

---

## 4. Provider Comparison Matrix

| Feature | Vaultwarden | Vault Transit | 1Password | CyberArk |
|---|---|---|---|---|
| Single-use access | ✅ | ✅ (cubbyhole) | ❌ | ✅ |
| TTL | ✅ | ✅ | ❌ | ✅ |
| Password protection | ✅ | ❌ | ❌ | ❌ |
| File sharing | ✅ | ❌ (text only) | ✅ | ❌ |
| Self-hosted | ✅ | ✅ | ✅ (Connect) | ✅ |
| Audit integration | ✅ | ✅ (native) | ✅ | ✅ |
| Complexity | Low | Medium | Medium | High |

---

## 5. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Vault Transit provider | Sprint 6 |
| 2 | 1Password Connect provider | Sprint 7 |
| 3 | Provider routing engine | Sprint 7 |
| 4 | CyberArk Conjur provider | Sprint 8 (optional) |
| 5 | Azure Key Vault provider | Sprint 8 (optional) |
| 6 | Provider selection UI | Sprint 8 |

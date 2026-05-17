# Change Request: Vaultwarden Send Integration

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-002                                                   |
| **Title**          | Vaultwarden Send Integration                                 |
| **Priority**       | P0 — Critical                                                |
| **Status**         | Draft                                                        |
| **PRD Refs**       | SEC-18                                                       |
| **Arch Layers**    | L7 (Plugin)                                                  |
| **Dependencies**   | CR-SHR-001                                                   |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Implement `SharingProvider` cho **Vaultwarden/Bitwarden Send API**, cho phép Bytebase tạo, quản lý, và revoke secure shares qua Vaultwarden — nền tảng self-hosted password manager phổ biến nhất cho enterprise.

### 1.1 Tại sao chọn Vaultwarden

| Tiêu chí | Vaultwarden | HashiCorp Vault |
|---|---|---|
| Self-hosted | ✅ Rust binary, nhẹ | ✅ Nặng hơn |
| Send API (ephemeral sharing) | ✅ Native | ❌ Cần custom |
| Password protection | ✅ Built-in | Cần custom |
| Auto-expiry | ✅ TTL + max access | Cần custom |
| Client encryption | ✅ End-to-end | Transit encrypt |
| Open source | ✅ (GPL-3.0) | ✅ (BSL) |
| Resource footprint | ~50MB RAM | ~200MB+ RAM |

### 1.2 Bitwarden Send Flow

```
Bytebase DBA
  → Encrypt credential (AES-256-GCM, client-side)
  → Create Send via Vaultwarden API
  → Vaultwarden stores encrypted blob
  → Generate access URL + decryption key
  → Deliver URL to recipient via Bytebase notification
  → Recipient accesses URL → decrypts with key
  → Send auto-expires after TTL or max access count
```

---

## 2. Requirements

### 2.1 Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Authenticate to Vaultwarden API | Support API key + OAuth2 client credentials |
| FR-2 | Create Send (Text type) | Encrypted credential → Vaultwarden Send |
| FR-3 | Create Send (File type) | SSL certs/SSH keys → file-based Send |
| FR-4 | Set TTL on Send | Configurable: 1h, 6h, 24h, 7d, 30d |
| FR-5 | Set max access count | Configurable: 1, 3, 5, unlimited |
| FR-6 | Set password on Send | Optional additional password protection |
| FR-7 | Revoke/delete Send | Immediate invalidation |
| FR-8 | List active Sends | Filter by status, date range |
| FR-9 | Health check | Verify Vaultwarden connectivity |

### 2.2 Non-Functional Requirements

| ID | Requirement | Target |
|---|---|---|
| NF-1 | API timeout | 10s per request, circuit breaker after 3 failures |
| NF-2 | TLS | Mandatory TLS to Vaultwarden |
| NF-3 | Retry | Exponential backoff (max 3 retries) |

---

## 3. Technical Design

### 3.1 Provider Implementation

```go
// backend/plugin/sharing/vaultwarden/provider.go
package vaultwarden

func init() {
    sharing.Register("vaultwarden", func(config *sharing.ProviderConfig) (sharing.SharingProvider, error) {
        return NewProvider(config)
    })
}

type Provider struct {
    client   *http.Client
    endpoint string       // e.g., "https://vault.company.com"
    apiKey   string
    orgID    string
}

func (p *Provider) Type() string { return "vaultwarden" }

func (p *Provider) CreateShare(ctx context.Context, req *sharing.ShareRequest) (*sharing.ShareResponse, error) {
    // 1. Encrypt payload (client-side AES-256-GCM)
    ciphertext, key, err := sharing.EncryptPayload(req.Payload)
    
    // 2. Build Bitwarden Send API request
    sendReq := &BitwardenSendRequest{
        Type:           determineSendType(req.CredentialType), // 0=Text, 1=File
        Name:           encrypt(req.Name, key),
        Text:           &SendTextData{Text: base64Encode(ciphertext)},
        DeletionDate:   req.ExpiresAt,
        MaxAccessCount: req.MaxAccessCount,
        Password:       hashPassword(req.Password),
    }
    
    // 3. POST to /api/sends
    resp, err := p.post(ctx, "/api/sends", sendReq)
    
    // 4. Return share response with access URL
    return &sharing.ShareResponse{
        ShareID:       resp.ID,
        AccessURL:     fmt.Sprintf("%s/#/send/%s/key/%s", p.endpoint, resp.AccessID, resp.Key),
        ExpiresAt:     resp.DeletionDate,
        EncryptionKey: key,
    }, nil
}
```

### 3.2 Bitwarden Send API Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/sends` | Create new Send |
| GET | `/api/sends/{id}` | Get Send metadata |
| PUT | `/api/sends/{id}` | Update Send |
| DELETE | `/api/sends/{id}` | Delete Send |
| PUT | `/api/sends/{id}/remove-password` | Remove password |
| POST | `/api/sends/access/{id}` | Access Send (recipient) |

### 3.3 Configuration

```yaml
# Workspace Setting JSONB
sharing:
  provider: "vaultwarden"
  vaultwarden:
    endpoint: "https://vault.company.com"
    api_key: "vault://sharing/vaultwarden-api-key"  # Resolved from vault
    organization_id: "org-uuid"
    default_ttl: "24h"
    default_max_access: 3
    tls:
      ca_cert: "vault://sharing/vaultwarden-ca-cert"
      skip_verify: false
```

### 3.4 Encryption Detail

Bitwarden Send uses a specific encryption scheme:

```
1. Generate Send Key (random 64 bytes)
2. Derive encryption key = HKDF-SHA256(sendKey, "bitwarden-send", "send")
3. Encrypt payload with AES-256-CBC + HMAC-SHA256 (EncString type 2)
4. Send Key itself is encrypted with user's master key
5. AccessId + Key → combined into access URL
```

Bytebase implementation wraps this with an additional AES-256-GCM layer for defense-in-depth.

---

## 4. Store Integration

```go
// Store Send mapping for audit and lifecycle management
type SharedCredentialMessage struct {
    ID              int64
    WorkspaceID     string
    Project         string
    IssueUID        int64
    ProviderType    string  // "vaultwarden"
    ProviderShareID string  // Vaultwarden Send ID
    Name            string
    CredentialType  string
    CreatorUID      int64
    RecipientUIDs   []int64
    Status          string  // ACTIVE, EXPIRED, REVOKED
    CreatedAt       time.Time
    ExpiresAt       time.Time
}
```

---

## 5. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Vaultwarden API changes | Pin API version, integration tests |
| Self-hosted Vaultwarden downtime | Circuit breaker, queue retries |
| Send key exposure | Key delivered via separate channel (notification) |
| Brute-force on Send password | Rate limiting on Vaultwarden side |

---

## 6. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Vaultwarden API client, auth | Sprint 2 |
| 2 | Send creation (text + file) | Sprint 2 |
| 3 | Lifecycle management (revoke, list) | Sprint 3 |
| 4 | Health check, circuit breaker | Sprint 3 |
| 5 | Integration tests with Vaultwarden container | Sprint 3 |

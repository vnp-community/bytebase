# SOL-SHR-001 — Sharing Provider Core & Plugin Registry

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-001 |
| CRs | CR-SHR-001 (Provider Abstraction), CR-SHR-002 (Vaultwarden Send), CR-SHR-006 (Multi-Platform) |
| Arch Layers | L5 (Component), L7 (Plugin), L8 (Store), L9 (Enterprise) |
| Priority | P0 — Critical |
| Sprints | 1–3 |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 Pattern tham chiếu: DB Driver Registration (L7)

Bytebase đã có pattern chuẩn cho plugin registration tại `plugin/db/`:

```go
// plugin/db/pg/driver.go
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}

// Factory: db.Open(ctx, engine, config) → db.Driver
```

**Solution**: Sharing provider sẽ dùng **chính xác pattern này** — `init()` registration + factory function.

### 1.2 Component Layer hiện tại (L5)

`component/secret/` hiện có:
- `secret.go` — `ReplaceExternalSecret()` với switch/case 4 providers
- Direct dependency vào store + enterprise license

**Solution**: Tạo `component/sharing/` song song với `component/secret/` — **không modify existing secret module** (giảm risk).

### 1.3 Bus Event System (L5)

```go
type Bus struct {
    ApprovalCheckChan  chan IssueRef  // 1000 buffer
    TaskRunTickleChan  chan int       // 1000 buffer
    // ... existing channels
}
```

**Solution**: Thêm `ShareEventChan chan ShareEvent` vào Bus để coordinate sharing operations.

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/
├── component/sharing/           ← L5: Core business logic
│   ├── sharing.go              ← SharingProvider interface
│   ├── manager.go              ← SharingManager (DI vào services)
│   ├── encrypt.go              ← AES-256-GCM encryption
│   ├── config.go               ← Provider configuration types
│   └── types.go                ← ShareRequest, ShareResponse, etc.
│
├── plugin/sharing/              ← L7: Provider implementations
│   ├── registry.go             ← Provider factory + registration
│   ├── vaultwarden/            ← CR-SHR-002
│   │   ├── provider.go         ← SharingProvider implementation
│   │   ├── client.go           ← Bitwarden Send API HTTP client
│   │   ├── crypto.go           ← Bitwarden-specific encryption
│   │   └── types.go            ← API request/response types
│   ├── vault_transit/           ← CR-SHR-006
│   │   ├── provider.go         ← Vault Response Wrapping
│   │   └── client.go           ← Vault API client
│   └── onepassword/             ← CR-SHR-006
│       ├── provider.go         ← 1Password Connect provider
│       └── client.go           ← Connect Server API client
│
├── store/
│   └── shared_credential.go    ← L8: CRUD for shared_credential table
│
└── api/v1/
    └── sharing_service.go      ← L4: gRPC service implementation
```

### 2.2 Core Interface Implementation

```go
// backend/component/sharing/sharing.go
package sharing

import "context"

// CredentialType enumerates types of shareable credentials.
type CredentialType string

const (
    CredentialTypePassword       CredentialType = "password"
    CredentialTypeSSLCert        CredentialType = "ssl_cert"
    CredentialTypeSSHKey         CredentialType = "ssh_key"
    CredentialTypeAPIKey         CredentialType = "api_key"
    CredentialTypeConnectionStr  CredentialType = "connection_string"
)

// SharingProvider defines the contract for sharing providers.
// Follows the same pattern as db.Driver interface.
type SharingProvider interface {
    // Type returns provider identifier (e.g., "vaultwarden", "vault_transit").
    Type() string
    // CreateShare creates a new ephemeral share.
    CreateShare(ctx context.Context, req *ShareRequest) (*ShareResponse, error)
    // GetShare retrieves share metadata (not content).
    GetShare(ctx context.Context, shareID string) (*ShareInfo, error)
    // RevokeShare invalidates a share immediately.
    RevokeShare(ctx context.Context, shareID string) error
    // ListShares returns shares matching filter criteria.
    ListShares(ctx context.Context, filter *ShareFilter) ([]*ShareInfo, error)
    // Ping verifies provider connectivity.
    Ping(ctx context.Context) error
    // Close releases provider resources.
    Close(ctx context.Context) error
}
```

### 2.3 Plugin Registry (Mirror db.Driver pattern)

```go
// backend/plugin/sharing/registry.go
package sharing

import (
    "fmt"
    "sync"
    component "github.com/bytebase/bytebase/backend/component/sharing"
)

type ProviderFactory func(config *component.ProviderConfig) (component.SharingProvider, error)

var (
    mu        sync.RWMutex
    providers = make(map[string]ProviderFactory)
)

// Register registers a sharing provider factory (called from init()).
func Register(providerType string, factory ProviderFactory) {
    mu.Lock()
    defer mu.Unlock()
    if _, dup := providers[providerType]; dup {
        panic(fmt.Sprintf("sharing: Register called twice for provider %s", providerType))
    }
    providers[providerType] = factory
}

// Open creates a provider instance of the specified type.
func Open(providerType string, config *component.ProviderConfig) (component.SharingProvider, error) {
    mu.RLock()
    factory, ok := providers[providerType]
    mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("sharing: unknown provider type %q", providerType)
    }
    return factory(config)
}
```

### 2.4 SharingManager — DI vào Services (L4)

```go
// backend/component/sharing/manager.go
package sharing

import (
    "context"
    "github.com/bytebase/bytebase/backend/store"
    enterprise "github.com/bytebase/bytebase/backend/enterprise/api"
)

// SharingManager is the central coordinator, injected into services.
type SharingManager struct {
    store          *store.Store
    licenseService enterprise.LicenseService
    provider       SharingProvider          // Active provider
    encryptor      *PayloadEncryptor        // AES-256-GCM
    config         *SharingConfig
}

// NewManager creates a SharingManager.
// Called during server bootstrap (after step 6 in TDD bootstrap sequence).
func NewManager(
    store *store.Store,
    licenseService enterprise.LicenseService,
    config *SharingConfig,
) (*SharingManager, error) {
    m := &SharingManager{
        store:          store,
        licenseService: licenseService,
        config:         config,
        encryptor:      NewPayloadEncryptor(config.EncryptionKey),
    }
    
    // Initialize provider if configured
    if config.ProviderType != "" {
        provider, err := sharingplugin.Open(config.ProviderType, config.ProviderConfig)
        if err != nil {
            return nil, fmt.Errorf("sharing: failed to initialize provider %q: %w", config.ProviderType, err)
        }
        m.provider = provider
    }
    
    return m, nil
}

// CreateShare encrypts payload and delegates to provider.
func (m *SharingManager) CreateShare(ctx context.Context, req *ShareRequest) (*ShareResponse, error) {
    // 1. Enterprise feature gate
    if err := m.checkFeatureGate(); err != nil {
        return nil, err
    }
    
    // 2. Encrypt payload before sending to provider
    encrypted, err := m.encryptor.Encrypt(req.Payload)
    if err != nil {
        return nil, fmt.Errorf("sharing: encryption failed: %w", err)
    }
    req.Payload = encrypted
    
    // 3. Delegate to provider
    resp, err := m.provider.CreateShare(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 4. Store metadata in Bytebase DB
    if err := m.store.CreateSharedCredential(ctx, &store.SharedCredentialMessage{
        WorkspaceID:     req.WorkspaceID,
        Project:         req.ProjectID,
        IssueUID:        req.IssueUID,
        ProviderType:    m.provider.Type(),
        ProviderShareID: resp.ShareID,
        Name:            req.Name,
        CredentialType:  string(req.CredentialType),
        CreatorUID:      req.CreatorUID,
        RecipientUIDs:   req.RecipientUIDs,
        MaxAccessCount:  req.MaxAccessCount,
        ExpiresAt:       resp.ExpiresAt,
        Status:          "ACTIVE",
    }); err != nil {
        return nil, err
    }
    
    return resp, nil
}
```

### 2.5 Server Bootstrap Integration

```
Server Bootstrap (updated — TDD Section 2):
  └─ NewServer(ctx, profile)
       ├─ 1. StartMetadataInstance()
       ├─ 2. store.New(pgURL)
       ├─ 3. migrator.MigrateSchema()
       ├─ 4. enterprise.NewLicenseService()
       ├─ 5. iam.NewManager()
       ├─ 6. webhook.NewManager()
       ├─ 6.5. sharing.NewManager(store, license, sharingConfig) ← NEW
       ├─ 7. dbfactory.New()
       ...
```

### 2.6 Proto Definition

```protobuf
// proto/v1/sharing_service.proto
syntax = "proto3";
package bytebase.v1;

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

service SharingService {
    rpc CreateShare(CreateShareRequest) returns (CreateShareResponse) {
        option (google.api.http) = {
            post: "/v1/{project=projects/*}/shares"
            body: "*"
        };
    }
    rpc GetShare(GetShareRequest) returns (Share) {
        option (google.api.http) = {
            get: "/v1/{name=projects/*/shares/*}"
        };
    }
    rpc RevokeShare(RevokeShareRequest) returns (google.protobuf.Empty) {
        option (google.api.http) = {
            post: "/v1/{name=projects/*/shares/*}:revoke"
        };
    }
    rpc ListShares(ListSharesRequest) returns (ListSharesResponse) {
        option (google.api.http) = {
            get: "/v1/{parent=projects/*}/shares"
        };
    }
}
```

### 2.7 Database Migration

```sql
-- backend/migrator/migration/prod/LATEST_sharing.sql
CREATE TABLE shared_credential (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    project         TEXT NOT NULL,
    issue_uid       BIGINT,
    provider_type   TEXT NOT NULL,
    provider_share_id TEXT NOT NULL,
    name            TEXT NOT NULL,
    credential_type TEXT NOT NULL,
    creator_uid     BIGINT NOT NULL REFERENCES principal(id),
    recipient_uids  JSONB NOT NULL DEFAULT '[]',
    max_access_count INT NOT NULL DEFAULT 1,
    access_count    INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    labels          JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_shared_credential_workspace ON shared_credential(workspace_id);
CREATE INDEX idx_shared_credential_project ON shared_credential(project, created_at DESC);
CREATE INDEX idx_shared_credential_creator ON shared_credential(creator_uid);
CREATE INDEX idx_shared_credential_status ON shared_credential(status)
    WHERE status = 'ACTIVE';
```

### 2.8 Vaultwarden Provider (CR-SHR-002)

```go
// backend/plugin/sharing/vaultwarden/provider.go
package vaultwarden

import (
    sharing "github.com/bytebase/bytebase/backend/component/sharing"
    sharingplugin "github.com/bytebase/bytebase/backend/plugin/sharing"
)

func init() {
    sharingplugin.Register("vaultwarden", func(config *sharing.ProviderConfig) (sharing.SharingProvider, error) {
        return NewProvider(config)
    })
}

type Provider struct {
    client    *BitwardenAPIClient
    endpoint  string
    orgID     string
}

func NewProvider(config *sharing.ProviderConfig) (*Provider, error) {
    client, err := NewBitwardenAPIClient(
        config.Endpoint,
        config.APIKey,
        config.TLSConfig,
    )
    if err != nil {
        return nil, err
    }
    return &Provider{
        client:   client,
        endpoint: config.Endpoint,
        orgID:    config.OrganizationID,
    }, nil
}

func (p *Provider) Type() string { return "vaultwarden" }

func (p *Provider) CreateShare(ctx context.Context, req *sharing.ShareRequest) (*sharing.ShareResponse, error) {
    // 1. Generate Send Key (64-byte random)
    sendKey := make([]byte, 64)
    if _, err := crypto_rand.Read(sendKey); err != nil {
        return nil, err
    }
    
    // 2. Derive encryption key via HKDF
    encKey := hkdf.Expand(sha256.New, sendKey, []byte("bitwarden-send"), 32)
    
    // 3. Encrypt payload with AES-256-CBC + HMAC (Bitwarden EncString Type 2)
    encData := encryptBitwardenFormat(req.Payload, encKey)
    
    // 4. Build Bitwarden Send API request
    sendReq := &SendCreateRequest{
        Type:           SendTypeText,  // 0=Text, 1=File
        Name:           encryptBitwardenFormat([]byte(req.Name), encKey),
        Text:           &SendTextData{Text: encData},
        DeletionDate:   req.ExpiresAt.Format(time.RFC3339),
        MaxAccessCount: int(req.MaxAccessCount),
    }
    if req.Password != "" {
        sendReq.Password = hashSendPassword(req.Password, encKey)
    }
    
    // 5. POST /api/sends
    resp, err := p.client.CreateSend(ctx, sendReq)
    if err != nil {
        return nil, fmt.Errorf("vaultwarden: create send failed: %w", err)
    }
    
    // 6. Build access URL
    accessURL := fmt.Sprintf("%s/#/send/%s/key/%s",
        p.endpoint,
        resp.AccessID,
        base64url.EncodeToString(sendKey),
    )
    
    return &sharing.ShareResponse{
        ShareID:       resp.ID,
        AccessURL:     accessURL,
        ExpiresAt:     resp.DeletionDate,
        EncryptionKey: sendKey, // NOT stored on provider
    }, nil
}
```

### 2.9 Provider Routing (CR-SHR-006)

```go
// backend/component/sharing/router.go
type ProviderRouter struct {
    defaultProvider string
    routes          []RouteRule
    providers       map[string]SharingProvider
}

type RouteRule struct {
    CredentialType CredentialType
    ProviderType   string
}

// Route selects the appropriate provider based on credential type.
func (r *ProviderRouter) Route(credType CredentialType) SharingProvider {
    for _, rule := range r.routes {
        if rule.CredentialType == credType || rule.CredentialType == "*" {
            if p, ok := r.providers[rule.ProviderType]; ok {
                return p
            }
        }
    }
    return r.providers[r.defaultProvider]
}
```

---

## 3. ACL Integration (L3)

Thêm sharing permissions vào `acl.go` interceptor:

```go
// backend/api/v1/acl.go — add to methodPermissionMap
"bytebase.v1.SharingService/CreateShare":  {iam.PermissionSharesCreate},
"bytebase.v1.SharingService/GetShare":     {iam.PermissionSharesGet},
"bytebase.v1.SharingService/RevokeShare":  {iam.PermissionSharesDelete},
"bytebase.v1.SharingService/ListShares":   {iam.PermissionSharesList},
```

Enterprise feature gate (L9):

```go
func (m *SharingManager) checkFeatureGate() error {
    return m.licenseService.IsFeatureEnabled(api.FeatureSecureSharing)
}
```

---

## 4. Bus Event Extension (L5)

```go
// backend/component/bus/bus.go — extend
type Bus struct {
    // ... existing channels ...
    ShareEventChan chan ShareEvent // 100 buffer ← NEW
}

type ShareEvent struct {
    Type     ShareEventType // CREATED, ACCESSED, REVOKED, EXPIRED
    ShareID  string
    ActorUID int64
}
```

---

## 5. Test Strategy

| Layer | Type | File | Coverage |
|---|---|---|---|
| L7 | Unit | `plugin/sharing/vaultwarden/provider_test.go` | Mock HTTP → test Send creation/revocation |
| L5 | Unit | `component/sharing/manager_test.go` | Test encryption, feature gate, store interaction |
| L5 | Unit | `component/sharing/encrypt_test.go` | AES-256-GCM roundtrip, edge cases |
| L7 | Integration | `plugin/sharing/vaultwarden/integration_test.go` | Real Vaultwarden container (testcontainers-go) |
| L4 | E2E | `tests/sharing_test.go` | Full API flow: create → access → revoke |

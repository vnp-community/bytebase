# Change Request: Sharing Provider Abstraction Layer

| Metadata           | Value                                                        |
|--------------------|--------------------------------------------------------------|
| **CR ID**          | CR-SHR-001                                                   |
| **Title**          | Sharing Provider Abstraction Layer                           |
| **Priority**       | P0 — Critical                                                |
| **Status**         | Draft                                                        |
| **PRD Refs**       | SEC-18, SEC-20                                               |
| **Arch Layers**    | L5 (Component), L7 (Plugin)                                  |
| **Dependencies**   | CR-VLT-001                                                   |
| **Created**        | 2026-05-17                                                   |

---

## 1. Mô tả

Xây dựng abstraction layer cho việc chia sẻ thông tin nhạy cảm qua các nền tảng trung gian an toàn. Layer này cung cấp interface chuẩn hóa cho tất cả sharing providers (Vaultwarden Send, HashiCorp Vault Transit, 1Password Connect), cho phép business logic tách biệt hoàn toàn khỏi implementation cụ thể.

### 1.1 Tại sao cần

Bytebase có `component/secret/` để lưu trữ secrets nhưng không có cơ chế chia sẻ giữa các actors (DBA → Developer). Capability Assessment xác nhận: "bàn giao vẫn là process thủ công".

---

## 2. Requirements

### 2.1 Functional Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | `SharingProvider` interface | Hỗ trợ: CreateShare, GetShare, RevokeShare, ListShares |
| FR-2 | Plugin registration via `init()` | Providers tự register giống DB driver pattern |
| FR-3 | Provider factory | `sharing.Open(providerType, config)` trả về provider instance |
| FR-4 | Credential packaging | Đóng gói: password, SSL cert/key, SSH key, API key, connection string |
| FR-5 | Encryption before sharing | AES-256-GCM encryption trước khi gửi tới provider |
| FR-6 | Time-limited shares | TTL configuration cho mỗi shared credential |
| FR-7 | Access-limited shares | Max access count cho mỗi shared credential |
| FR-8 | Share metadata | Track: creator, recipients, created_at, expires_at, access_count |

### 2.2 Non-Functional Requirements

| ID | Requirement | Target |
|---|---|---|
| NF-1 | Latency | Share creation < 500ms |
| NF-2 | Encryption | AES-256-GCM for credential payload |
| NF-3 | Zero plaintext | Credentials NEVER in logs |
| NF-4 | Enterprise only | Feature gate: `FEATURE_SECURE_SHARING` |

---

## 3. Technical Design

### 3.1 Core Interface

```go
// backend/component/sharing/sharing.go
type SharingProvider interface {
    Type() string
    CreateShare(ctx context.Context, req *ShareRequest) (*ShareResponse, error)
    GetShare(ctx context.Context, shareID string) (*ShareInfo, error)
    RevokeShare(ctx context.Context, shareID string) error
    ListShares(ctx context.Context, filter *ShareFilter) ([]*ShareInfo, error)
    Ping(ctx context.Context) error
    Close(ctx context.Context) error
}

type ShareRequest struct {
    Payload        []byte
    CredentialType CredentialType // password, ssl_cert, ssh_key, api_key
    Name           string
    MaxAccessCount int32
    ExpiresAt      *time.Time
    Password       string        // Optional additional password
    CreatorUID     int64
    RecipientUIDs  []int64
    ProjectID      string
    IssueUID       int64
}

type ShareResponse struct {
    ShareID        string
    AccessURL      string
    ExpiresAt      time.Time
    EncryptionKey  []byte // NOT stored on provider
}
```

### 3.2 Plugin Registry (same pattern as db.Driver)

```go
// backend/plugin/sharing/registry.go
var providers = make(map[string]ProviderFactory)

func Register(providerType string, factory ProviderFactory) { ... }
func Open(providerType string, config *ProviderConfig) (SharingProvider, error) { ... }
```

### 3.3 Store Schema

```sql
CREATE TABLE shared_credential (
    id              BIGSERIAL PRIMARY KEY,
    workspace_id    TEXT NOT NULL,
    project         TEXT NOT NULL,
    issue_uid       BIGINT,
    provider_type   TEXT NOT NULL,
    provider_share_id TEXT NOT NULL,
    name            TEXT NOT NULL,
    credential_type TEXT NOT NULL,
    creator_uid     BIGINT NOT NULL,
    recipient_uids  JSONB NOT NULL DEFAULT '[]',
    max_access_count INT NOT NULL DEFAULT 1,
    access_count    INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    labels          JSONB NOT NULL DEFAULT '{}'
);
```

### 3.4 Architecture Placement

```
L5 — component/sharing/
      ├── sharing.go    — Interface definitions
      ├── encrypt.go    — AES-256-GCM encryption
      ├── manager.go    — SharingManager (injected into services)
      └── config.go     — Provider configuration

L7 — plugin/sharing/
      ├── registry.go   — Provider registration
      ├── vaultwarden/  — CR-SHR-002
      └── vault_transit/— CR-SHR-006
```

---

## 4. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Encryption key loss | Key escrowed trong Bytebase vault (CR-VLT-001) |
| Provider unavailability | Circuit breaker + cached share metadata |
| Credential leak via logs | Zero-plaintext logging policy |

---

## 5. Implementation Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Interface definition, encryption module | Sprint 1 |
| 2 | Store schema, SharingManager | Sprint 1 |
| 3 | Plugin registry, factory | Sprint 2 |
| 4 | Feature gate, ACL, tests | Sprint 2-3 |

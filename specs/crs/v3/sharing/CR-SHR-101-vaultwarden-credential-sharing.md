# Change Request: Vaultwarden Secure Credential Sharing Integration

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SHR-101                                               |
| **Gap ID**         | G-SHR-1                                                  |
| **Title**          | Vaultwarden Secure Credential Sharing Integration        |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | Vaultwarden (Bitwarden-compatible self-hosted)           |
| **Extends**        | CR-SHR-001 (Sharing Provider Abstraction), CR-SHR-002 (Vaultwarden Send) |

---

## 1. Tổng quan

### 1.1 Mô tả
Mở rộng `backend/component/secret/` hiện tại để tích hợp **Vaultwarden** làm nền tảng trung gian chia sẻ thông tin nhạy cảm ở mức **Organization Vault** — vượt ra ngoài Bitwarden Send (CR-SHR-002) để cung cấp:
- Đồng bộ database credentials từ Bytebase → Vaultwarden Organization vaults
- Chia sẻ credentials an toàn giữa DBA, Developer, và Platform Engineers qua Vaultwarden collections
- Tự động rotate passwords trên cả Bytebase và Vaultwarden khi policy trigger
- Bitwarden CLI/API-compatible — hỗ trợ mọi Bitwarden client (desktop, browser extension, mobile)

### 1.2 Bối cảnh
CR-SHR-001 định nghĩa abstraction layer (`SharingProvider` interface). CR-SHR-002 implement Vaultwarden Send cho **ephemeral sharing**. CR này bổ sung **persistent sharing** qua Organization Vault — cho phép team-based secret management dài hạn.

Hiện tại Bytebase lưu database credentials trong:
- **Store (L8)**: PostgreSQL `instance` table → `data_sources` JSONB (chứa password encrypted)
- **External Secret Manager (L5)**: HashiCorp Vault, AWS SM, GCP SM → chỉ hỗ trợ read/write, không sharing

DBA khi cần chia sẻ credentials cho team thường dùng:
- Slack/Teams messages (⚠️ plaintext, không auto-expire)
- Shared documents (⚠️ lưu trữ vĩnh viễn, không audit)
- Email (⚠️ không mã hóa, dễ forward)

### 1.3 Mục tiêu
- Bidirectional sync: Bytebase ↔ Vaultwarden organization vault
- Auto-provision collections theo Bytebase project/environment hierarchy
- IAM access mapping: Bytebase roles → Vaultwarden permissions
- Credential lifecycle management qua Vaultwarden API

---

## 2. Yêu cầu chức năng

### FR-001: Organization Vault Mapping
- Map Bytebase workspace → Vaultwarden Organization
- Map Bytebase projects → Vaultwarden Collections
- Map Bytebase environments → Collection sub-groups (via naming convention)
- Naming convention: `BB/{project_name}/{environment}/{instance_name}`
- Auto-create collections khi tạo project/instance mới trong Bytebase
- Auto-archive collections khi project bị archive

### FR-002: Credential Sync Engine
- **Push sync** (Bytebase → Vaultwarden):
  - Khi DBA thêm/cập nhật instance credentials → push to Vaultwarden collection
  - Fields synced: hostname, port, username, password, SSL cert, SSH key
  - Credentials stored as Bitwarden Login item type
  - Custom fields: `bytebase_instance_id`, `bytebase_project`, `bytebase_environment`, `last_rotated`
- **Pull sync** (Vaultwarden → Bytebase):
  - Khi credentials được update trên Vaultwarden → sync back to Bytebase instance config
  - Conflict resolution: Vaultwarden wins (manual update = intentional change)
  - Webhook-based: Vaultwarden events → Bytebase API endpoint
- **Sync schedule**: Real-time via webhooks + periodic reconciliation (default: every 5 minutes)

### FR-003: Access Control Mapping
- Map Bytebase IAM roles → Vaultwarden collection permissions:

  | Bytebase Role | Vaultwarden Permission | Collection Access |
  |---|---|---|
  | `workspaceAdmin` | Organization Admin | All collections — Read/Write |
  | `workspaceDBA` | Collection Manager | Assigned collections — Read/Write |
  | `projectOwner` | Collection User (Manage) | Project collections — Read/Write |
  | `projectDeveloper` | Collection User (Read) | Project collections — Read only |
  | `projectQuerier` | Collection User (Read) | Project collections — Read only |

- Auto-sync membership: Bytebase IAM policy change → update Vaultwarden collection access
- Group support: Bytebase User Groups → Vaultwarden Groups

### FR-004: Credential Lifecycle Events
- **On Instance Create**: Auto-generate strong password, store in both Bytebase + Vaultwarden
- **On Password Rotation**: Update both stores atomically (2-phase commit with rollback)
- **On Instance Delete**: Soft-delete in Vaultwarden (move to trash, auto-purge after 30 days)
- **On Project Archive**: Revoke collection access for non-admin users
- Integration with existing `Bus` events (L5)

### FR-005: Configuration UI
- Settings page: `Workspace Settings → Integration → Vaultwarden`
- Configuration fields:
  - Server URL (validated via `/api/alive`)
  - Authentication method selector (API Key, OAuth2, PAT)
  - Organization ID
  - Sync mode: Push-only / Pull-only / Bidirectional
  - Sync interval (minutes)
  - Collection naming template (customizable)
- Connection test button
- Sync status dashboard: last sync time, items synced, errors

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Vault Sync Engine | `backend/component/sharing/vaultwarden/sync.go` | Bidirectional sync logic |
| Collection Manager | `backend/component/sharing/vaultwarden/collection.go` | Organization/collection CRUD |
| Access Mapper | `backend/component/sharing/vaultwarden/access.go` | IAM → Vaultwarden permission mapping |
| Sync Runner | `backend/runner/sharing/vaultwarden_sync.go` | Background sync runner |
| Config Model | `backend/component/sharing/vaultwarden/config.go` | Extended configuration |
| Setting API Extension | `backend/api/v1/setting_service.go` | Vaultwarden config CRUD |
| Instance Service Hook | `backend/api/v1/instance_service.go` | Trigger sync on instance CRUD |
| UI — Settings Page | `frontend/src/views/Settings/VaultwardenConfig.vue` | Configuration UI |
| Database Migration | `backend/migrator/migration/*/` | Tables: `vaultwarden_config`, `vaultwarden_sync_state` |

### 3.1 Database Schema

```sql
CREATE TABLE vaultwarden_config (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    server_url TEXT NOT NULL,
    auth_method TEXT NOT NULL DEFAULT 'api_key',
    encrypted_credentials BYTEA NOT NULL,
    organization_id TEXT NOT NULL,
    sync_mode TEXT NOT NULL DEFAULT 'bidirectional',
    sync_interval_minutes INT DEFAULT 5,
    collection_template TEXT DEFAULT 'BB/{project}/{environment}/{instance}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    creator_id INT REFERENCES principal(id),
    UNIQUE(workspace_id)
);

CREATE TABLE vaultwarden_sync_state (
    id SERIAL PRIMARY KEY,
    config_id INT REFERENCES vaultwarden_config(id),
    instance_id TEXT NOT NULL,
    vaultwarden_item_id TEXT,
    vaultwarden_collection_id TEXT,
    last_sync_at TIMESTAMPTZ,
    sync_direction TEXT,
    sync_status TEXT DEFAULT 'pending',
    error_message TEXT,
    version INT DEFAULT 1,
    UNIQUE(config_id, instance_id)
);
```

### 3.2 Sync Engine Flow

```
Instance CRUD Event (L4)
  │
  ├─► Bus.InstanceEventChan ─► VaultwardenSyncRunner
  │     ├─ 1. Load VaultwardenConfig from store
  │     ├─ 2. Resolve target collection (project/env mapping)
  │     ├─ 3. Encrypt credentials (reuse CR-SHR-001 encryption)
  │     ├─ 4. Push to Vaultwarden API (create/update item)
  │     ├─ 5. Update sync_state table
  │     └─ 6. Emit audit event (CR-SHR-004)
  │
  └─► Periodic Reconciliation (every N minutes)
        ├─ List all Bytebase instances with sync enabled
        ├─ List all Vaultwarden items in mapped collections
        ├─ Compare versions → detect drift
        └─ Resolve conflicts + update both sides
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---|---|
| Vaultwarden API key exposure | Stored encrypted (AES-256-GCM) using Bytebase master key |
| Credentials in transit | TLS 1.3 required + application-level encryption |
| Vaultwarden server compromise | Bitwarden organization encryption; Bytebase adds additional layer |
| Sync race conditions | Optimistic locking via version counter |
| Unauthorized access mapping | ACL interceptor (L3) validates before sync; IAM mapping is additive only |

---

## 5. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | Configure Vaultwarden, test connectivity | Connection passes, organization verified |
| TC-002 | Add PG instance → auto-sync | Item in correct collection |
| TC-003 | Update instance password → push sync | Vaultwarden item updated |
| TC-004 | Update password in Vaultwarden → pull sync | Bytebase updated |
| TC-005 | Create project → auto-create collection | Collection with correct name |
| TC-006 | Add user to project → update access | User gains collection access |
| TC-007 | Delete instance → soft-delete | Item moved to trash |
| TC-008 | Vaultwarden down → graceful failure | Error logged, retried with backoff |

---

## 6. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Sync engine + collection auto-provisioning | Sprint 4-5 |
| Phase 2 | Bidirectional reconciliation | Sprint 5-6 |
| Phase 3 | IAM access mapping + group sync | Sprint 6-7 |
| Phase 4 | E2E testing + production hardening | Sprint 7-8 |

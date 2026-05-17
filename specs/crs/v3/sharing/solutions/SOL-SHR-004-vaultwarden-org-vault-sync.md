# SOL-SHR-004 — Vaultwarden Organization Vault Sync

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-004 |
| CRs | CR-SHR-101 (Vaultwarden Secure Credential Sharing) |
| Arch Layers | L4 (Service), L5 (Component + Bus), L6 (Runner), L7 (Plugin), L8 (Store) |
| Priority | P0 — Critical |
| Sprints | 4–8 |
| Dependencies | SOL-SHR-001 (Provider Core), SOL-SHR-002 (Envelope Encryption) |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 Instance & DataSource Storage (L8)

```go
// store/instance.go
type InstanceMessage struct {
    ResourceID  string
    Title       string
    Engine      storepb.Engine
    Host        string
    Port        string
    DataSources []*DataSourceMessage  // Contains credentials
}

type DataSourceMessage struct {
    Type     DataSourceType
    Username string
    Password string  // Obfuscated (XOR + base64), or external_secret ref
    SSLCert  string
    SSLKey   string
    SSHHost  string
    SSHKey   string
    ExternalSecret *storepb.DataSourceExternalSecret
}
```

### 1.2 Bus Event System (L5)

Hiện tại Bus không có `InstanceEventChan`. Cần thêm để trigger sync.

### 1.3 SchemaSync Runner Pattern (L6)

`runner/schemasync/` — periodic sync runner, là mẫu tham chiếu cho VaultwardenSync:
- Periodic ticker
- Per-instance processing
- Error recovery + logging

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/
├── component/sharing/vaultwarden/    ← L5: Vaultwarden org vault logic
│   ├── sync.go                       ← Bidirectional sync engine
│   ├── collection.go                 ← Collection management
│   ├── access.go                     ← IAM → Vaultwarden mapping
│   ├── client.go                     ← Bitwarden Organization API client
│   ├── config.go                     ← Configuration model
│   └── types.go                      ← Bitwarden API types
│
├── runner/sharing/                    ← L6: Background runners
│   └── vaultwarden_sync.go          ← Sync runner (periodic + event-driven)
│
├── store/                             ← L8: Data persistence
│   ├── vaultwarden_config.go         ← Config CRUD
│   └── vaultwarden_sync_state.go     ← Sync state tracking
│
└── api/v1/
    └── setting_service.go            ← L4: Extended with Vaultwarden config
```

### 2.2 Bitwarden Organization API Client

```go
// backend/component/sharing/vaultwarden/client.go
package vaultwarden

import (
    "context"
    "net/http"
)

// OrgClient wraps the Bitwarden Organization API.
type OrgClient struct {
    httpClient *http.Client
    baseURL    string
    token      string
    orgID      string
}

// Organization API endpoints used:
// POST   /api/accounts/prelogin            ← Pre-login (get KDF params)
// POST   /identity/connect/token           ← Authenticate
// GET    /api/organizations/{orgId}         ← Org info
// GET    /api/organizations/{orgId}/collections         ← List collections
// POST   /api/organizations/{orgId}/collections         ← Create collection
// DELETE /api/organizations/{orgId}/collections/{id}    ← Delete collection
// GET    /api/ciphers/organization-details?organizationId=  ← List org items
// POST   /api/ciphers/create               ← Create item
// PUT    /api/ciphers/{id}                  ← Update item
// DELETE /api/ciphers/{id}                  ← Delete item
// PUT    /api/organizations/{orgId}/users/{id} ← Update user access

func (c *OrgClient) CreateCollection(ctx context.Context, name string) (*Collection, error) {
    req := &CreateCollectionRequest{
        Name: c.encryptOrgKey(name), // Encrypted with org key
        Groups: []GroupAccessSelection{},
    }
    return c.post(ctx, fmt.Sprintf("/api/organizations/%s/collections", c.orgID), req)
}

func (c *OrgClient) CreateCipherInOrg(ctx context.Context, item *CipherCreateRequest) (*Cipher, error) {
    item.OrganizationID = c.orgID
    return c.post(ctx, "/api/ciphers/create", item)
}

func (c *OrgClient) UpdateCipher(ctx context.Context, cipherID string, item *CipherUpdateRequest) (*Cipher, error) {
    return c.put(ctx, fmt.Sprintf("/api/ciphers/%s", cipherID), item)
}

func (c *OrgClient) ListOrgCiphers(ctx context.Context) ([]*Cipher, error) {
    return c.get(ctx, fmt.Sprintf("/api/ciphers/organization-details?organizationId=%s", c.orgID))
}
```

### 2.3 Collection Manager

```go
// backend/component/sharing/vaultwarden/collection.go
package vaultwarden

// CollectionManager maps Bytebase project/environment hierarchy to Vaultwarden collections.
type CollectionManager struct {
    client   *OrgClient
    store    *store.Store
    template string // e.g., "BB/{project}/{environment}/{instance}"
}

// EnsureCollection creates a collection if it doesn't exist, returns collection ID.
func (m *CollectionManager) EnsureCollection(ctx context.Context, project, env, instance string) (string, error) {
    // 1. Build collection name from template
    name := m.buildName(project, env, instance)
    
    // 2. Check if already exists in sync_state
    state, err := m.store.GetVaultwardenSyncState(ctx, &store.FindSyncStateMessage{
        CollectionName: name,
    })
    if err == nil && state != nil {
        return state.VaultwardenCollectionID, nil
    }
    
    // 3. Create in Vaultwarden
    collection, err := m.client.CreateCollection(ctx, name)
    if err != nil {
        return "", fmt.Errorf("vaultwarden: create collection failed: %w", err)
    }
    
    return collection.ID, nil
}

// SyncProjectStructure ensures all project/env combinations have collections.
func (m *CollectionManager) SyncProjectStructure(ctx context.Context) error {
    projects, _ := m.store.ListProjectsV2(ctx, &store.FindProjectMessage{})
    envs, _ := m.store.ListEnvironments(ctx)
    
    for _, proj := range projects {
        if proj.Deleted {
            // Archive collections for deleted projects
            m.archiveProjectCollections(ctx, proj.ResourceID)
            continue
        }
        for _, env := range envs {
            m.EnsureCollection(ctx, proj.ResourceID, env.ResourceID, "")
        }
    }
    return nil
}

func (m *CollectionManager) buildName(project, env, instance string) string {
    r := strings.NewReplacer(
        "{project}", project,
        "{environment}", env,
        "{instance}", instance,
    )
    return r.Replace(m.template)
}
```

### 2.4 Credential Sync Engine

```go
// backend/component/sharing/vaultwarden/sync.go
package vaultwarden

// SyncEngine handles bidirectional credential sync.
type SyncEngine struct {
    client     *OrgClient
    collMgr    *CollectionManager
    accessMgr  *AccessMapper
    encryptor  *envelope.Encryptor
    store      *store.Store
}

// PushCredential syncs a Bytebase instance credential to Vaultwarden.
func (s *SyncEngine) PushCredential(ctx context.Context, instance *store.InstanceMessage, ds *store.DataSourceMessage) error {
    // 1. Resolve collection
    collectionID, err := s.collMgr.EnsureCollection(
        ctx,
        instance.Project,
        instance.EnvironmentID,
        instance.ResourceID,
    )
    if err != nil {
        return err
    }
    
    // 2. Check existing sync state
    state, _ := s.store.GetVaultwardenSyncState(ctx, &store.FindSyncStateMessage{
        InstanceID: instance.ResourceID,
    })
    
    // 3. Build Bitwarden Login cipher
    cipher := &CipherCreateRequest{
        Type: CipherTypeLogin, // 1 = Login
        Name: s.encryptField(fmt.Sprintf("%s@%s:%s", ds.Username, instance.Host, instance.Port)),
        Login: &LoginData{
            Username: s.encryptField(ds.Username),
            Password: s.encryptField(ds.Password),
            URIs: []LoginURI{{
                URI: s.encryptField(fmt.Sprintf("%s:%s", instance.Host, instance.Port)),
            }},
        },
        Fields: []CipherField{
            {Name: s.encryptField("bytebase_instance_id"), Value: s.encryptField(instance.ResourceID)},
            {Name: s.encryptField("bytebase_environment"), Value: s.encryptField(instance.EnvironmentID)},
            {Name: s.encryptField("engine"), Value: s.encryptField(instance.Engine.String())},
            {Name: s.encryptField("last_rotated"), Value: s.encryptField(time.Now().Format(time.RFC3339))},
        },
        CollectionIDs: []string{collectionID},
    }
    
    if state != nil && state.VaultwardenItemID != "" {
        // 4a. Update existing item
        _, err = s.client.UpdateCipher(ctx, state.VaultwardenItemID, cipher.ToUpdate())
    } else {
        // 4b. Create new item
        resp, err := s.client.CreateCipherInOrg(ctx, cipher)
        if err != nil {
            return err
        }
        state = &store.VaultwardenSyncStateMessage{
            InstanceID:              instance.ResourceID,
            VaultwardenItemID:       resp.ID,
            VaultwardenCollectionID: collectionID,
        }
    }
    
    // 5. Update sync state
    state.LastSyncAt = time.Now()
    state.SyncDirection = "push"
    state.SyncStatus = "synced"
    state.Version++
    return s.store.UpsertVaultwardenSyncState(ctx, state)
}

// Reconcile performs periodic drift detection and correction.
func (s *SyncEngine) Reconcile(ctx context.Context) error {
    // 1. List all Bytebase instances with sync enabled
    instances, _ := s.store.ListInstancesV2(ctx, &store.FindInstanceMessage{})
    
    // 2. List all Vaultwarden items in org
    vwItems, _ := s.client.ListOrgCiphers(ctx)
    vwIndex := indexByCustomField(vwItems, "bytebase_instance_id")
    
    // 3. Compare and reconcile
    for _, inst := range instances {
        for _, ds := range inst.DataSources {
            state, _ := s.store.GetVaultwardenSyncState(ctx, &store.FindSyncStateMessage{
                InstanceID: inst.ResourceID,
            })
            
            vwItem, exists := vwIndex[inst.ResourceID]
            if !exists {
                // Bytebase has, Vaultwarden doesn't → push
                s.PushCredential(ctx, inst, ds)
                continue
            }
            
            // Compare versions
            if state != nil && state.Version != extractVersion(vwItem) {
                // Drift detected
                if s.config.SyncMode == "bidirectional" {
                    // Vaultwarden wins (manual change = intentional)
                    s.PullCredential(ctx, inst, vwItem)
                } else {
                    s.PushCredential(ctx, inst, ds)
                }
            }
        }
    }
    return nil
}

// PullCredential syncs a Vaultwarden item back to Bytebase.
func (s *SyncEngine) PullCredential(ctx context.Context, inst *store.InstanceMessage, cipher *Cipher) error {
    // Decrypt Vaultwarden cipher fields → update Bytebase instance DataSource
    password := s.decryptField(cipher.Login.Password)
    username := s.decryptField(cipher.Login.Username)
    
    // Update via InstanceService (respects existing obfuscation/external secret flow)
    return s.store.UpdateDataSource(ctx, inst.ResourceID, &store.UpdateDataSourceMessage{
        Username: &username,
        Password: &password,
    })
}
```

### 2.5 IAM Access Mapper

```go
// backend/component/sharing/vaultwarden/access.go
package vaultwarden

// AccessMapper syncs Bytebase IAM roles to Vaultwarden collection permissions.
type AccessMapper struct {
    client    *OrgClient
    iamMgr    *iam.Manager
    store     *store.Store
}

// Mapping table: Bytebase Role → Vaultwarden Permission
var roleMapping = map[string]VaultwardenPermission{
    "roles/workspaceAdmin":    {ReadOnly: false, HidePasswords: false, Manage: true},
    "roles/workspaceDBA":      {ReadOnly: false, HidePasswords: false, Manage: true},
    "roles/projectOwner":      {ReadOnly: false, HidePasswords: false, Manage: false},
    "roles/projectDeveloper":  {ReadOnly: true,  HidePasswords: false, Manage: false},
    "roles/projectQuerier":    {ReadOnly: true,  HidePasswords: true,  Manage: false},
}

// SyncProjectAccess updates Vaultwarden collection access for a project.
func (m *AccessMapper) SyncProjectAccess(ctx context.Context, projectID string) error {
    // 1. Get project IAM policy
    policy, _ := m.store.GetProjectIAMPolicy(ctx, projectID)
    
    // 2. Get collection ID for this project
    collections, _ := m.store.ListVaultwardenSyncStates(ctx, &store.FindSyncStateMessage{
        ProjectID: projectID,
    })
    
    for _, binding := range policy.Bindings {
        vwPerm, ok := roleMapping[binding.Role]
        if !ok {
            continue
        }
        for _, member := range binding.Members {
            userUID := extractUID(member)
            // 3. Map user to Vaultwarden org member
            vwMember, err := m.findOrInviteVaultwardenUser(ctx, userUID)
            if err != nil {
                continue
            }
            // 4. Set collection access
            for _, coll := range collections {
                m.client.SetCollectionAccess(ctx, vwMember.ID, coll.VaultwardenCollectionID, vwPerm)
            }
        }
    }
    return nil
}
```

### 2.6 Sync Runner (L6)

```go
// backend/runner/sharing/vaultwarden_sync.go
package sharing

// VaultwardenSyncRunner handles both event-driven and periodic sync.
// Pattern: similar to SchemaSync runner.
type VaultwardenSyncRunner struct {
    store      *store.Store
    syncEngine *vaultwarden.SyncEngine
    bus        *bus.Bus
    interval   time.Duration
}

func (r *VaultwardenSyncRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
            
        case event := <-r.bus.InstanceEventChan:
            // Event-driven sync (real-time)
            r.handleInstanceEvent(ctx, event)
            
        case event := <-r.bus.IAMPolicyEventChan:
            // IAM change → update Vaultwarden access
            r.syncEngine.AccessMapper.SyncProjectAccess(ctx, event.ProjectID)
            
        case <-ticker.C:
            // Periodic reconciliation
            r.syncEngine.Reconcile(ctx)
        }
    }
}

func (r *VaultwardenSyncRunner) handleInstanceEvent(ctx context.Context, event bus.InstanceEvent) {
    switch event.Type {
    case bus.InstanceEventCreated, bus.InstanceEventUpdated:
        inst, _ := r.store.GetInstanceV2(ctx, &store.FindInstanceMessage{ResourceID: &event.InstanceID})
        for _, ds := range inst.DataSources {
            r.syncEngine.PushCredential(ctx, inst, ds)
        }
    case bus.InstanceEventDeleted:
        r.syncEngine.SoftDeleteCredential(ctx, event.InstanceID)
    }
}
```

### 2.7 Bus Extension

```go
// backend/component/bus/bus.go — extend
type Bus struct {
    // ... existing channels ...
    InstanceEventChan  chan InstanceEvent  // 100 buffer ← NEW
    IAMPolicyEventChan chan IAMPolicyEvent // 100 buffer ← NEW
}

type InstanceEvent struct {
    Type       InstanceEventType
    InstanceID string
    ActorUID   int64
}

type IAMPolicyEvent struct {
    ProjectID string
    ActorUID  int64
}
```

### 2.8 Database Migration

```sql
CREATE TABLE vaultwarden_config (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL UNIQUE,
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
    creator_id INT REFERENCES principal(id)
);

CREATE TABLE vaultwarden_sync_state (
    id SERIAL PRIMARY KEY,
    config_id INT REFERENCES vaultwarden_config(id) ON DELETE CASCADE,
    instance_id TEXT NOT NULL,
    project_id TEXT,
    environment_id TEXT,
    vaultwarden_item_id TEXT,
    vaultwarden_collection_id TEXT,
    last_sync_at TIMESTAMPTZ,
    sync_direction TEXT,
    sync_status TEXT DEFAULT 'pending',
    error_message TEXT,
    version INT DEFAULT 1,
    UNIQUE(config_id, instance_id)
);

CREATE INDEX idx_vw_sync_status ON vaultwarden_sync_state(sync_status)
    WHERE sync_status != 'synced';
```

---

## 3. Server Bootstrap Integration

```
Server Bootstrap (updated):
  ├─ 6.5. sharingManager = sharing.NewManager(...)     ← SOL-SHR-001
  ├─ 6.6. vwSyncRunner = sharing.NewVaultwardenSyncRunner(
  │          store, sharingManager.SyncEngine, bus)     ← NEW
  └─ 9. Initialize Runners:
       ├─ ... existing runners ...
       └─ vwSyncRunner.Run(ctx)                         ← NEW
```

---

## 4. Test Strategy

| Test | Description | Method |
|---|---|---|
| Collection auto-provision | Create project → collection exists | Integration (mock VW API) |
| Push sync | Add instance → VW item created | Integration |
| Pull sync | Update VW password → Bytebase updated | Integration |
| Reconciliation | Drift detected → corrected | Integration |
| IAM mapping | Add user to project → VW access set | Unit + Integration |
| Connection failure | VW down → graceful retry | Unit (mock HTTP 500) |
| Real Vaultwarden | Full E2E with Docker Vaultwarden | testcontainers-go |

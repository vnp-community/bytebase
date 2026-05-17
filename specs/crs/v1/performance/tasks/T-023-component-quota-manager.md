# T-023: Component — Quota Manager

| Field | Value |
|-------|-------|
| **Task ID** | T-023 |
| **Solution** | SOL-PERF-004 |
| **Type** | New file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | None |
| **Status** | DONE |

## Target File

`backend/component/quota/manager.go` (new)

## Implementation

```go
package quota

import (
    "context"
    "fmt"
    "sync"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "github.com/bytebase/bytebase/backend/store"
)

type ResourceType string
const (
    ResourceInstance ResourceType = "instance"
    ResourceDatabase ResourceType = "database"
    ResourceProject  ResourceType = "project"
    ResourceUser     ResourceType = "user"
)

type QuotaConfig struct {
    MaxInstances int `json:"maxInstances"`
    MaxDatabases int `json:"maxDatabases"`
    MaxProjects  int `json:"maxProjects"`
    MaxUsers     int `json:"maxUsers"`
}

var DefaultQuota = QuotaConfig{
    MaxInstances: 100, MaxDatabases: 5000,
    MaxProjects: 50, MaxUsers: 200,
}

type Manager struct {
    store      *store.Store
    mu         sync.RWMutex
    quotaCache map[string]*QuotaConfig
    usageCache map[string]map[ResourceType]int
}

func NewManager(s *store.Store) *Manager {
    return &Manager{
        store: s, quotaCache: make(map[string]*QuotaConfig),
        usageCache: make(map[string]map[ResourceType]int),
    }
}

func (m *Manager) CheckQuota(ctx context.Context, workspace string, resource ResourceType) error {
    quota := m.getQuota(workspace)
    usage := m.getUsage(ctx, workspace, resource)
    var limit int
    switch resource {
    case ResourceInstance: limit = quota.MaxInstances
    case ResourceDatabase: limit = quota.MaxDatabases
    case ResourceProject:  limit = quota.MaxProjects
    case ResourceUser:     limit = quota.MaxUsers
    }
    if usage >= limit {
        return status.Errorf(codes.ResourceExhausted,
            "workspace %q exceeded %s quota: %d/%d", workspace, resource, usage, limit)
    }
    return nil
}

func (m *Manager) getQuota(workspace string) *QuotaConfig {
    m.mu.RLock()
    if q, ok := m.quotaCache[workspace]; ok {
        m.mu.RUnlock()
        return q
    }
    m.mu.RUnlock()
    return &DefaultQuota
}

func (m *Manager) getUsage(ctx context.Context, workspace string, resource ResourceType) int {
    m.mu.RLock()
    if ws, ok := m.usageCache[workspace]; ok {
        if c, ok := ws[resource]; ok { m.mu.RUnlock(); return c }
    }
    m.mu.RUnlock()
    count := m.queryCount(ctx, workspace, resource)
    m.mu.Lock()
    if m.usageCache[workspace] == nil { m.usageCache[workspace] = make(map[ResourceType]int) }
    m.usageCache[workspace][resource] = count
    m.mu.Unlock()
    return count
}

func (m *Manager) queryCount(ctx context.Context, workspace string, resource ResourceType) int {
    table := map[ResourceType]string{
        ResourceInstance: "instance", ResourceDatabase: "db",
        ResourceProject: "project", ResourceUser: "principal",
    }[resource]
    var count int
    q := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE workspace = $1 AND deleted = false", table)
    m.store.GetDB().QueryRowContext(ctx, q, workspace).Scan(&count)
    return count
}

func (m *Manager) InvalidateUsage(workspace string, resource ResourceType) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if ws, ok := m.usageCache[workspace]; ok { delete(ws, resource) }
}
```

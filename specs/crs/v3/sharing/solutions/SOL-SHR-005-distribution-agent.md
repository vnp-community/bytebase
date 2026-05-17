# SOL-SHR-005 — Cross-Platform Secret Distribution Agent

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-005 |
| CRs | CR-SHR-104 (Cross-Platform Distribution Agent) |
| Arch Layers | L5 (Component), L6 (Runner), L7 (Plugin) |
| Priority | P1 — High |
| Sprints | 6–10 |
| Dependencies | SOL-SHR-001, SOL-SHR-002 (BEE envelope), SOL-SHR-004 (VW sync triggers) |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 Plugin Registration Pattern (L7)

Tham chiếu `plugin/db/` registration:
```go
func init() { db.Register(engine, factory) }
```
Target adapters sẽ dùng pattern tương tự.

### 1.2 TaskRun DAG (L6)

Bytebase đã có concept **task dependencies** trong migration plans:
```go
// store/task.go
type TaskMessage struct {
    DependsOn []int64 // Task dependencies
}
```
Distribution pipeline sẽ reuse pattern này cho target dependencies.

### 1.3 Bus Event Coordination (L5)

Credential rotation event từ SOL-SHR-004 (VW sync) hoặc trực tiếp từ `InstanceService` sẽ trigger distribution via Bus.

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/
├── component/distribution/           ← L5: Core agent logic
│   ├── agent.go                      ← Distribution agent coordinator
│   ├── pipeline.go                   ← DAG execution engine
│   ├── rollback.go                   ← Rollback manager
│   ├── version.go                    ← Credential versioning
│   └── types.go                      ← Common types
│
├── plugin/distribution/              ← L7: Target adapters
│   ├── registry.go                   ← Adapter registration
│   ├── k8s/adapter.go               ← Kubernetes Secret
│   ├── jenkins/adapter.go           ← Jenkins Credentials
│   ├── gitlab/adapter.go            ← GitLab CI Variables
│   ├── github/adapter.go            ← GitHub Actions Secrets
│   ├── grafana/adapter.go           ← Grafana Datasource
│   └── webhook/adapter.go           ← Generic HTTP webhook
│
├── runner/distribution/              ← L6: Background runner
│   └── runner.go                     ← Event-driven distribution
│
└── store/
    ├── distribution_target.go        ← Target config CRUD
    ├── distribution_event.go         ← Event tracking
    └── credential_version.go         ← Version history
```

### 2.2 Target Adapter Interface & Registry

```go
// backend/plugin/distribution/registry.go
package distribution

import (
    "context"
    "sync"
    component "github.com/bytebase/bytebase/backend/component/distribution"
)

type AdapterFactory func(config map[string]interface{}) (component.TargetAdapter, error)

var (
    mu       sync.RWMutex
    adapters = make(map[string]AdapterFactory)
)

func Register(adapterType string, factory AdapterFactory) {
    mu.Lock()
    defer mu.Unlock()
    adapters[adapterType] = factory
}

func Open(adapterType string, config map[string]interface{}) (component.TargetAdapter, error) {
    mu.RLock()
    factory, ok := adapters[adapterType]
    mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("distribution: unknown adapter %q", adapterType)
    }
    return factory(config)
}
```

```go
// backend/component/distribution/types.go
package distribution

type TargetAdapter interface {
    Type() string
    Validate(ctx context.Context, config map[string]interface{}) error
    Distribute(ctx context.Context, target *TargetConfig, creds *Credentials) (*DistributeResult, error)
    Verify(ctx context.Context, target *TargetConfig, creds *Credentials) error
    Rollback(ctx context.Context, target *TargetConfig, prevCreds *Credentials) error
}

type Credentials struct {
    Host     string
    Port     string
    Username string
    Password string
    Engine   string
    SSLCert  []byte
    SSLKey   []byte
    Metadata map[string]string
}

type DistributeResult struct {
    TargetID    int
    TargetType  string
    Status      string // "success", "failed", "skipped"
    Message     string
    DistributedAt time.Time
}
```

### 2.3 Kubernetes Adapter

```go
// backend/plugin/distribution/k8s/adapter.go
package k8s

import (
    dist "github.com/bytebase/bytebase/backend/plugin/distribution"
    component "github.com/bytebase/bytebase/backend/component/distribution"
    "k8s.io/client-go/kubernetes"
)

func init() {
    dist.Register("kubernetes", func(config map[string]interface{}) (component.TargetAdapter, error) {
        return NewK8sAdapter(config)
    })
}

type Adapter struct {
    clientset *kubernetes.Clientset
    namespace string
}

func (a *Adapter) Type() string { return "kubernetes" }

func (a *Adapter) Distribute(ctx context.Context, target *component.TargetConfig, creds *component.Credentials) (*component.DistributeResult, error) {
    secretName := target.Config["secret_name"].(string)
    namespace := target.Config["namespace"].(string)
    
    secret, err := a.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
    if err != nil {
        // Create new secret
        secret = &corev1.Secret{
            ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
            Type:       corev1.SecretTypeOpaque,
            Data: map[string][]byte{
                "host":     []byte(creds.Host),
                "port":     []byte(creds.Port),
                "username": []byte(creds.Username),
                "password": []byte(creds.Password),
            },
        }
        _, err = a.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
    } else {
        // Update existing
        secret.Data["username"] = []byte(creds.Username)
        secret.Data["password"] = []byte(creds.Password)
        _, err = a.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
    }
    
    return &component.DistributeResult{
        Status: "success",
        DistributedAt: time.Now(),
    }, err
}

func (a *Adapter) Verify(ctx context.Context, target *component.TargetConfig, creds *component.Credentials) error {
    secretName := target.Config["secret_name"].(string)
    namespace := target.Config["namespace"].(string)
    
    secret, err := a.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("k8s: secret %s/%s not found", namespace, secretName)
    }
    
    if string(secret.Data["password"]) != creds.Password {
        return fmt.Errorf("k8s: password mismatch in %s/%s", namespace, secretName)
    }
    return nil
}

func (a *Adapter) Rollback(ctx context.Context, target *component.TargetConfig, prev *component.Credentials) error {
    return a.Distribute(ctx, target, prev).Error
}
```

### 2.4 DAG Pipeline Engine

```go
// backend/component/distribution/pipeline.go
package distribution

import (
    "context"
    "sync"
)

// Pipeline executes distribution to multiple targets with dependency ordering.
type Pipeline struct {
    store      *store.Store
    adapters   map[string]TargetAdapter
    versioner  *CredentialVersioner
}

type PipelineResult struct {
    EventID         int
    TotalTargets    int
    SucceededTargets int
    FailedTargets   int
    Results         []*DistributeResult
    RolledBack      bool
}

// Execute runs distribution following DAG order.
func (p *Pipeline) Execute(ctx context.Context, instanceID string, creds *Credentials, dryRun bool) (*PipelineResult, error) {
    // 1. Version current credentials
    p.versioner.SaveVersion(ctx, instanceID, creds)
    
    // 2. Load mappings with dependencies
    mappings, _ := p.store.ListDistributionMappings(ctx, instanceID)
    
    // 3. Build DAG levels
    levels := p.buildDAGLevels(mappings)
    
    // 4. Execute level by level
    result := &PipelineResult{TotalTargets: len(mappings)}
    
    for _, level := range levels {
        var wg sync.WaitGroup
        var mu sync.Mutex
        
        for _, mapping := range level {
            wg.Add(1)
            go func(m *DistributionMapping) {
                defer wg.Done()
                
                target, _ := p.store.GetDistributionTarget(ctx, m.TargetID)
                adapter, ok := p.adapters[target.TargetType]
                if !ok {
                    mu.Lock()
                    result.FailedTargets++
                    mu.Unlock()
                    return
                }
                
                if dryRun {
                    // Validate only, don't execute
                    err := adapter.Validate(ctx, target.Config)
                    mu.Lock()
                    if err != nil {
                        result.FailedTargets++
                    } else {
                        result.SucceededTargets++
                    }
                    mu.Unlock()
                    return
                }
                
                // Distribute
                r, err := adapter.Distribute(ctx, &TargetConfig{Config: target.Config}, creds)
                if err != nil {
                    mu.Lock()
                    result.FailedTargets++
                    result.Results = append(result.Results, &DistributeResult{
                        TargetID: target.ID, Status: "failed", Message: err.Error(),
                    })
                    mu.Unlock()
                    return
                }
                
                // Verify
                if err := adapter.Verify(ctx, &TargetConfig{Config: target.Config}, creds); err != nil {
                    mu.Lock()
                    result.FailedTargets++
                    mu.Unlock()
                    
                    // Critical target failed → trigger rollback
                    if target.Criticality == "critical" {
                        p.rollbackAll(ctx, instanceID, result)
                    }
                    return
                }
                
                mu.Lock()
                result.SucceededTargets++
                result.Results = append(result.Results, r)
                mu.Unlock()
            }(mapping)
        }
        wg.Wait()
        
        // If critical failure at this level, stop pipeline
        if result.RolledBack {
            break
        }
    }
    
    return result, nil
}

// buildDAGLevels groups mappings by dependency levels.
func (p *Pipeline) buildDAGLevels(mappings []*DistributionMapping) [][]*DistributionMapping {
    // Topological sort by depends_on
    var levels [][]*DistributionMapping
    visited := make(map[int]bool)
    
    for len(visited) < len(mappings) {
        var level []*DistributionMapping
        for _, m := range mappings {
            if visited[m.ID] {
                continue
            }
            allDepsVisited := true
            for _, dep := range m.DependsOn {
                if !visited[dep] {
                    allDepsVisited = false
                    break
                }
            }
            if allDepsVisited {
                level = append(level, m)
            }
        }
        for _, m := range level {
            visited[m.ID] = true
        }
        levels = append(levels, level)
    }
    return levels
}
```

### 2.5 Credential Versioner

```go
// backend/component/distribution/version.go
package distribution

const maxVersions = 5

type CredentialVersioner struct {
    store *store.Store
}

// SaveVersion stores the current credential as a new version.
func (v *CredentialVersioner) SaveVersion(ctx context.Context, instanceID string, creds *Credentials) error {
    // 1. Mark all current versions as non-current
    v.store.DeactivateCredentialVersions(ctx, instanceID)
    
    // 2. Get next version number
    latest, _ := v.store.GetLatestCredentialVersion(ctx, instanceID)
    nextVersion := 1
    if latest != nil {
        nextVersion = latest.Version + 1
    }
    
    // 3. Encrypt and store
    encrypted, _ := envelope.Seal(ctx, creds.Marshal(), envelope.EnvelopeMetadata{
        ContentType: "credential_version",
    })
    
    v.store.CreateCredentialVersion(ctx, &store.CredentialVersionMessage{
        InstanceID:           instanceID,
        Version:              nextVersion,
        EncryptedCredentials: encrypted,
        IsCurrent:            true,
    })
    
    // 4. Prune old versions (keep last N)
    v.store.PruneCredentialVersions(ctx, instanceID, maxVersions)
    
    return nil
}

// GetPreviousVersion retrieves the version before current (for rollback).
func (v *CredentialVersioner) GetPreviousVersion(ctx context.Context, instanceID string) (*Credentials, error) {
    version, err := v.store.GetCredentialVersion(ctx, instanceID, -1) // second-latest
    if err != nil {
        return nil, err
    }
    plaintext, err := envelope.Open(ctx, version.EncryptedCredentials)
    if err != nil {
        return nil, err
    }
    return UnmarshalCredentials(plaintext)
}
```

### 2.6 Distribution Runner (L6)

```go
// backend/runner/distribution/runner.go
package distribution

// DistributionRunner listens for credential change events and triggers distribution.
type DistributionRunner struct {
    store    *store.Store
    pipeline *distribution.Pipeline
    bus      *bus.Bus
    notifier *sharing.ShareNotifier
}

func (r *DistributionRunner) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-r.bus.CredentialRotationChan:
            r.handleRotation(ctx, event)
        }
    }
}

func (r *DistributionRunner) handleRotation(ctx context.Context, event bus.CredentialRotationEvent) {
    // 1. Check if distribution is configured for this instance
    mappings, _ := r.store.ListDistributionMappings(ctx, event.InstanceID)
    if len(mappings) == 0 {
        return
    }
    
    // 2. Create distribution event
    distEvent, _ := r.store.CreateDistributionEvent(ctx, &store.DistributionEventMessage{
        WorkspaceID:  event.WorkspaceID,
        InstanceID:   event.InstanceID,
        TriggerType:  event.TriggerType,
        TotalTargets: len(mappings),
    })
    
    // 3. Execute pipeline
    result, err := r.pipeline.Execute(ctx, event.InstanceID, event.Credentials, false)
    if err != nil {
        slog.Error("distribution: pipeline failed", "error", err, "instance", event.InstanceID)
    }
    
    // 4. Update event status
    r.store.UpdateDistributionEvent(ctx, distEvent.ID, result)
    
    // 5. Notify results
    r.notifier.NotifyDistributionResult(ctx, distEvent, result)
}
```

### 2.7 Database Migration

```sql
CREATE TABLE distribution_target (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 0,
    criticality TEXT DEFAULT 'normal',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE distribution_mapping (
    id SERIAL PRIMARY KEY,
    target_id INT REFERENCES distribution_target(id) ON DELETE CASCADE,
    instance_id TEXT NOT NULL,
    target_secret_path TEXT NOT NULL,
    depends_on INT[] DEFAULT '{}',
    UNIQUE(target_id, instance_id)
);

CREATE INDEX idx_dist_mapping_instance ON distribution_mapping(instance_id);

CREATE TABLE distribution_event (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    trigger_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    total_targets INT,
    succeeded_targets INT DEFAULT 0,
    failed_targets INT DEFAULT 0,
    details JSONB DEFAULT '{}'
);

CREATE TABLE credential_version (
    id SERIAL PRIMARY KEY,
    instance_id TEXT NOT NULL,
    version INT NOT NULL,
    encrypted_credentials JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_current BOOLEAN DEFAULT FALSE,
    UNIQUE(instance_id, version)
);

CREATE INDEX idx_cred_version_current ON credential_version(instance_id)
    WHERE is_current = TRUE;
```

---

## 3. Bus Extension

```go
// backend/component/bus/bus.go
type Bus struct {
    // ... existing + from SOL-SHR-004 ...
    CredentialRotationChan chan CredentialRotationEvent // 50 buffer ← NEW
}

type CredentialRotationEvent struct {
    WorkspaceID string
    InstanceID  string
    TriggerType string // "manual", "policy", "sync"
    Credentials *distribution.Credentials
}
```

---

## 4. Test Strategy

| Test | Description | Method |
|---|---|---|
| K8s adapter | Distribute → verify | Mock k8s clientset |
| Jenkins adapter | Update credential | Mock Jenkins HTTP API |
| DAG execution | L0 parallel, L1 sequential | Unit: mock adapters |
| Critical failure → rollback | Verify fails → all reverted | Integration |
| Dry-run | No actual changes | Unit |
| Credential versioning | Save 5, prune oldest | Unit |
| Runner event handling | Bus event → pipeline execute | Integration |

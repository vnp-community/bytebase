# Change Request: OPAL Policy Distribution Integration

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-POL-004                                               |
| **Title**          | OPAL Policy Distribution Integration                    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-POL-001, CR-POL-002                                   |

---

## 1. Tổng quan

### 1.1 Mô tả
Tích hợp **OPAL (Open Policy Administration Layer)** vào Bytebase để cung cấp khả năng **real-time policy distribution** cho toàn bộ hệ thống. OPAL hoạt động như control plane, quản lý vòng đời phân phối chính sách từ Git repository tới tất cả OPA agents (embedded, sidecar, fleet) với real-time data synchronization.

### 1.2 Bối cảnh
Hiện tại policies trong Bytebase:
- Lưu trong PostgreSQL (`policy` table, JSONB payload)
- Không có mechanism cho real-time distribution
- Không có centralized policy management cho multi-instance deployments
- Không hỗ trợ GitOps workflow cho policy changes
- Không có data synchronization giữa policy engine và application state

OPAL giải quyết các vấn đề này bằng:
- **Git-based policy source** — policies được quản lý trong Git, auto-distributed
- **Real-time data updates** — khi Bytebase data thay đổi (roles, projects, environments), OPA data được cập nhật ngay lập tức
- **Multi-instance sync** — tất cả Bytebase instances trong HA deployment nhận policy updates đồng thời
- **Policy lifecycle** — staging, testing, rollout, rollback cho policy changes

### 1.3 Mục tiêu
- OPAL Client integration trong Bytebase binary
- OPAL Server deployment option (sidecar hoặc external)
- Real-time policy distribution từ Git repository
- Real-time data synchronization từ Bytebase store → OPA
- Policy change notification via Bus/NATS
- Multi-instance HA support

---

## 2. Yêu cầu chức năng

### FR-001: OPAL Client Integration

Bytebase embeds OPAL Client để nhận policy updates:

```go
// OPALClientEngine extends OPAEmbeddedEngine with OPAL distribution.
type OPALClientEngine struct {
    *OPAEmbeddedEngine                    // Inherits OPA evaluation
    opalClient    *opal.Client            // OPAL client for policy distribution
    serverURL     string                  // OPAL server URL
    dataTopics    []string                // Subscribed data topics
    policyTopics  []string                // Subscribed policy topics
    callbacks     *OPALCallbacks          // Lifecycle callbacks
}

func NewOPALClientEngine(config *OPALConfig) (*OPALClientEngine, error) {
    // Initialize embedded OPA engine
    // Connect to OPAL server
    // Subscribe to policy + data topics
    // Start sync goroutine
}

// OnPolicyUpdate is called when OPAL distributes a policy change.
func (e *OPALClientEngine) OnPolicyUpdate(update *opal.PolicyUpdate) error {
    // Compile new Rego modules
    // Hot-swap policy set
    // Emit Bus event for audit logging
}

// OnDataUpdate is called when OPAL distributes a data change.
func (e *OPALClientEngine) OnDataUpdate(update *opal.DataUpdate) error {
    // Update OPA store with new data
    // Invalidate decision cache
}
```

### FR-002: Bytebase-OPAL Data Publisher

Publishes Bytebase state changes to OPAL for distribution:

```go
// OPALDataPublisher watches Bytebase events and publishes to OPAL.
type OPALDataPublisher struct {
    store       *store.Store
    bus         *component.Bus
    opalServer  *opal.DataSourceClient
    topics      map[string]DataTopicConfig
}

// Data topics that Bytebase publishes to OPAL
var DataTopics = map[string]DataTopicConfig{
    "bytebase/environments": {
        Source:   "store.ListEnvironments",
        Trigger:  "environment.created|updated|deleted",
        Interval: 60 * time.Second,  // Fallback polling
    },
    "bytebase/projects": {
        Source:   "store.ListProjects",
        Trigger:  "project.created|updated|deleted",
        Interval: 60 * time.Second,
    },
    "bytebase/iam/roles": {
        Source:   "store.GetRoleSnapshot",
        Trigger:  "role.created|updated|deleted",
        Interval: 120 * time.Second,
    },
    "bytebase/iam/groups": {
        Source:   "store.ListGroups",
        Trigger:  "group.members.changed",
        Interval: 60 * time.Second,
    },
    "bytebase/policies/masking": {
        Source:   "store.ListPolicies(MASKING)",
        Trigger:  "policy.masking.updated",
        Interval: 30 * time.Second,
    },
    "bytebase/databases/classification": {
        Source:   "store.ListDatabaseClassifications",
        Trigger:  "database.classification.updated",
        Interval: 300 * time.Second,
    },
}

// Watch starts listening for Bus events and publishing to OPAL.
func (p *OPALDataPublisher) Watch(ctx context.Context) {
    // Listen to NATSBus/Go channels for relevant events
    // On event → fetch data from store → publish to OPAL server
}
```

### FR-003: OPAL Server Deployment Configuration

```go
type OPALServerConfig struct {
    // Deployment mode
    Mode        OPALMode   // EMBEDDED, SIDECAR, EXTERNAL

    // Server connection
    ServerURL   string     // OPAL server URL (sidecar/external)
    AuthToken   string     // Authentication token (stored in vault via CR-VLT-001)

    // Policy source
    PolicyRepo  OPALPolicyRepo

    // Data sources
    DataSources []OPALDataSource

    // Distribution settings
    BroadcastType string  // "pubsub", "webhook", "polling"
    PubSubURL     string  // Redis/Kafka URL for pub/sub distribution
}

type OPALPolicyRepo struct {
    URL         string     // Git repository URL
    Branch      string     // Default: "main"
    Path        string     // Path within repo for Bytebase policies
    SSHKey      string     // SSH key for private repos (vault reference)
    PollInterval time.Duration // Default: 30s
}

type OPALMode int
const (
    OPALModeEmbedded OPALMode = iota  // OPAL client in Bytebase process
    OPALModeSidecar                    // OPAL server as sidecar container
    OPALModeExternal                   // External OPAL server (shared fleet)
)
```

### FR-004: Policy Repository Structure

Recommended Git repository structure cho OPAL-managed policies:

```
bytebase-policies/
├── README.md
├── opal.yaml                    # OPAL configuration
├── base/                        # Base policies (always loaded)
│   ├── access_control.rego      # Core access control rules
│   ├── masking.rego             # Base masking rules
│   └── governance.rego          # Schema change governance
├── environments/                # Environment-specific overrides
│   ├── development/
│   │   └── relaxed_access.rego  # Relaxed rules for dev
│   ├── staging/
│   │   └── staging_rules.rego
│   └── production/
│       ├── strict_access.rego   # Strict production rules
│       ├── change_window.rego   # Time-based change windows
│       └── approval_rules.rego  # Multi-approver requirements
├── compliance/                  # Compliance-specific policies
│   ├── gdpr/
│   │   └── data_residency.rego
│   ├── pci-dss/
│   │   └── cardholder_access.rego
│   └── sox/
│       └── audit_requirements.rego
├── custom/                      # Organization-specific policies
│   └── .gitkeep
├── tests/                       # Policy tests (Conftest/OPA test)
│   ├── access_test.rego
│   ├── masking_test.rego
│   └── governance_test.rego
└── data/                        # Static data for policies
    ├── classifications.json
    └── change_windows.json
```

### FR-005: Bus Integration — Policy Change Events

Extend existing Bus with policy-specific channels:

```go
// Extensions to component/bus/Bus
type Bus struct {
    // ... existing channels ...

    // Policy-specific channels
    PolicyUpdateChan     chan PolicyUpdateEvent  // buffer: 100
    PolicyReloadChan     chan PolicyReloadEvent  // buffer: 10
    PolicyDecisionChan   chan PolicyDecisionLog  // buffer: 5000
}

type PolicyUpdateEvent struct {
    EngineID   string
    PolicyID   string
    Action     string  // "loaded", "updated", "removed"
    Version    int
    Source     string  // "opal", "api", "gitops"
    Timestamp  time.Time
}

type PolicyReloadEvent struct {
    EngineID   string
    Reason     string  // "opal_update", "manual", "health_recovery"
    Timestamp  time.Time
}
```

### FR-006: Multi-Instance HA Support

```
┌──────────────────────────────────────────────────┐
│                   OPAL Server                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Git Sync │  │ Pub/Sub  │  │ Data Sources │   │
│  │ (Rego)   │  │ (Redis)  │  │ (Bytebase)   │   │
│  └──────────┘  └──────────┘  └──────────────┘   │
└─────────┬──────────┬──────────┬──────────────────┘
          │          │          │
    ┌─────▼──┐ ┌─────▼──┐ ┌────▼───┐
    │ BB #1  │ │ BB #2  │ │ BB #3  │
    │ OPAL   │ │ OPAL   │ │ OPAL   │
    │ Client │ │ Client │ │ Client │
    │   +    │ │   +    │ │   +    │
    │  OPA   │ │  OPA   │ │  OPA   │
    └────────┘ └────────┘ └────────┘
```

- Mỗi Bytebase instance chạy OPAL Client
- OPAL Server broadcast policy + data updates qua pub/sub
- Tất cả instances nhận updates đồng thời (< 5 giây latency)
- Data Publisher chạy trên leader instance only (HA mode leader election)

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| OPALClientEngine                   | `backend/component/policy/opal/client.go`            | New: OPAL client engine implementation       |
| OPALDataPublisher                  | `backend/component/policy/opal/publisher.go`         | New: Bytebase → OPAL data publisher          |
| OPALConfig                         | `backend/component/policy/opal/config.go`            | New: OPAL configuration types                |
| Bus extensions                     | `backend/component/bus/bus.go`                       | Extend: policy event channels                |
| OPAL Runner                       | `backend/runner/opal/`                               | New: OPAL data publisher background runner   |
| Docker Compose                     | `docker-compose.yaml`                                | Add: OPAL server sidecar option              |
| Proto: OPALConfig                  | `proto/store/policy_engine.proto`                    | Add: OPAL-specific configuration fields      |

### 3.1 Docker Compose — OPAL Sidecar

```yaml
services:
  bytebase:
    image: bytebase/bytebase:latest
    environment:
      - BB_POLICY_ENGINE_TYPE=opal-managed
      - BB_OPAL_SERVER_URL=http://opal-server:7002
    depends_on:
      - opal-server

  opal-server:
    image: permitio/opal-server:latest
    environment:
      - OPAL_POLICY_REPO_URL=https://github.com/org/bytebase-policies.git
      - OPAL_POLICY_REPO_MAIN_BRANCH=main
      - OPAL_DATA_CONFIG_SOURCES={"config":{"entries":[{"url":"http://bytebase:8080/api/v1/opal-data","topics":["bytebase"]}]}}
      - OPAL_LOG_LEVEL=INFO
    ports:
      - "7002:7002"  # OPAL server API
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| OPAL server authentication       | JWT token authentication, stored in vault                     |
| Git repository access            | SSH key or deploy token, stored in vault                      |
| Policy tampering in transit      | TLS for all OPAL communications                              |
| Data publisher exposure           | OPAL data endpoint requires internal auth (HMAC)             |
| Policy rollback safety           | Git revert → OPAL auto-distributes previous version          |
| Multi-tenant isolation           | OPAL topics scoped per workspace                             |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | OPAL client connects to server                        | Connected, subscribed to topics            |
| TC-002  | Git push triggers policy update                       | Policy distributed to all instances < 5s   |
| TC-003  | Data publisher: environment change                    | OPA data updated across instances          |
| TC-004  | OPAL server unavailable → fallback                    | Local policy cache continues to work       |
| TC-005  | HA: leader publishes, followers receive               | All instances have consistent data         |
| TC-006  | Policy rollback via git revert                        | Previous policy version active across fleet|
| TC-007  | Bus: PolicyUpdateEvent emitted on policy change       | Audit log captures policy lifecycle events |
| TC-008  | Concurrent policy + data update                       | No race conditions, consistent state       |

---

## 6. Rollout Plan

| Phase   | Mô tả                                         | Timeline       |
|---------|------------------------------------------------|----------------|
| Phase 1 | OPAL client integration + basic sync           | Sprint 1-2     |
| Phase 2 | Data publisher + Bus integration               | Sprint 2-3     |
| Phase 3 | Docker Compose OPAL sidecar setup              | Sprint 3       |
| Phase 4 | Policy repository templates + CI/CD            | Sprint 3-4     |
| Phase 5 | HA multi-instance testing                      | Sprint 4       |
| Phase 6 | Documentation + runbook                        | Sprint 5       |

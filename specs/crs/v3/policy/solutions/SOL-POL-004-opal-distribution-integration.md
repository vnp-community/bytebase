# Solution: OPAL Policy Distribution Integration

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-004 |
| **CR Reference** | CR-POL-004 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001, SOL-POL-002 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| `OPALClientEngine` | **L7 — Plugin** | Extends OPAEmbeddedEngine with OPAL distribution |
| `OPALDataPublisher` | **L6 — Runner** | Background goroutine publishing Bytebase state to OPAL |
| Bus extensions | **L5 — Component** | Policy event channels in existing Bus |
| OPAL Runner | **L6 — Runner** | Background runner for data publisher, like SchemaSync |
| Docker Compose | **L10 — Infra** | OPAL server sidecar deployment |

---

## 2. Package Structure

```
backend/component/policy/opal/
├── client.go          ← OPALClientEngine: OPA + OPAL distribution
├── publisher.go       ← OPALDataPublisher: Bytebase → OPAL data sync
├── config.go          ← OPALConfig, OPALServerConfig, OPALPolicyRepo
└── callbacks.go       ← OPAL lifecycle callbacks (policy/data update handlers)

backend/runner/opal/
└── runner.go          ← OPALRunner: background data publisher runner
```

---

## 3. Key Design Decisions

### 3.1 OPAL Client as PolicyEngine Extension

`OPALClientEngine` extends `OPAEmbeddedEngine` (SOL-POL-002), adding real-time policy/data distribution:

```go
type OPALClientEngine struct {
    *OPAEmbeddedEngine                  // Inherits all OPA evaluation
    opalClient    *opal.Client          // OPAL WebSocket client
    serverURL     string
    dataTopics    []string
    policyTopics  []string
}

// OnPolicyUpdate — hot-swap policies when OPAL distributes changes
func (e *OPALClientEngine) OnPolicyUpdate(update *opal.PolicyUpdate) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    // 1. Compile new Rego modules
    // 2. Atomic swap of compiler + policy set
    // 3. Invalidate decision cache (via PolicyManager)
    // 4. Emit Bus PolicyUpdateEvent for audit
}

// OnDataUpdate — refresh OPA data store
func (e *OPALClientEngine) OnDataUpdate(update *opal.DataUpdate) error {
    // Update OPA in-memory store with new data
    // Invalidate cache entries affected by changed data topics
}
```

### 3.2 Data Publisher — Bus-Driven, Runner Pattern

Follows existing Runner architecture (TDD §5):

```go
type OPALDataPublisher struct {
    store       *store.Store
    bus         *bus.Bus
    opalServer  *opal.DataSourceClient
    topics      map[string]DataTopicConfig
}

// Data topics map Bytebase events → OPAL data updates
var DataTopics = map[string]DataTopicConfig{
    "bytebase/environments":           {Trigger: "environment.*",     Interval: 60s},
    "bytebase/projects":               {Trigger: "project.*",         Interval: 60s},
    "bytebase/iam/roles":              {Trigger: "role.*",            Interval: 120s},
    "bytebase/iam/groups":             {Trigger: "group.members.*",   Interval: 60s},
    "bytebase/policies/masking":       {Trigger: "policy.masking.*",  Interval: 30s},
    "bytebase/databases/classification": {Trigger: "database.class.*", Interval: 300s},
}
```

Runner lifecycle follows existing pattern (goroutine + context cancellation):

```go
// backend/runner/opal/runner.go
type OPALRunner struct {
    publisher *OPALDataPublisher
}

func (r *OPALRunner) Run(ctx context.Context) {
    // 1. Initial full data sync
    // 2. Start event listener on Bus channels
    // 3. On event → fetch from store → publish to OPAL server
    // 4. Fallback: periodic sync per topic interval
}
```

### 3.3 Bus Extension — Policy Event Channels

Extends existing Bus (TDD §5.1) with policy-specific channels:

```go
// backend/component/bus/bus.go — additions
type Bus struct {
    // ... existing channels (ApprovalCheckChan, PlanCheckTickleChan, etc.) ...

    // Policy channels
    PolicyUpdateChan     chan PolicyUpdateEvent   // buffer: 100
    PolicyReloadChan     chan PolicyReloadEvent   // buffer: 10
    PolicyDecisionChan   chan PolicyDecisionLog   // buffer: 5000 (high throughput)
}
```

**Design rationale**: Uses same buffered Go channel pattern as existing Bus. For NATS mode (`BB_USE_NATS_BUS=true`), policy events also published to NATS subjects.

### 3.4 HA Multi-Instance Support

```
┌──────────────────────────┐
│       OPAL Server        │
│  Git Sync + Pub/Sub      │
└────┬─────┬─────┬─────────┘
     │     │     │
  ┌──▼──┐┌─▼──┐┌▼───┐
  │BB #1││BB#2││BB#3│   ← All instances run OPAL Client
  │OPAL ││OPAL││OPAL│   ← Receive policy+data updates simultaneously
  │+OPA ││+OPA││+OPA│   ← < 5s latency for propagation
  └─────┘└────┘└────┘
```

- **Data Publisher**: Runs on **leader instance only** (HA mode leader election, existing pattern)
- **OPAL Client**: Runs on **all instances** (each receives updates)
- **Pub/Sub backbone**: Redis (recommended) or Kafka for OPAL server broadcasting

### 3.5 Deployment Modes

| Mode | Architecture | Use Case |
|---|---|---|
| **Embedded** | OPAL client in Bytebase process, no external OPAL server | Single-instance, simplest |
| **Sidecar** | OPAL server as Docker Compose sidecar | Multi-instance, standard |
| **External** | Shared OPAL server fleet (separate deployment) | Large-scale enterprise |

---

## 4. Docker Compose — Sidecar Setup

```yaml
services:
  bytebase:
    image: bytebase/bytebase:latest
    environment:
      - BB_POLICY_ENGINE_TYPE=opal-managed
      - BB_OPAL_SERVER_URL=http://opal-server:7002
    depends_on: [opal-server]

  opal-server:
    image: permitio/opal-server:latest
    environment:
      - OPAL_POLICY_REPO_URL=https://github.com/org/bytebase-policies.git
      - OPAL_POLICY_REPO_MAIN_BRANCH=main
      - OPAL_DATA_CONFIG_SOURCES={"config":{"entries":[{"url":"http://bytebase:8080/api/v1/opal-data","topics":["bytebase"]}]}}
    ports: ["7002:7002"]
    depends_on: [redis]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
```

---

## 5. Policy Repository Structure

```
bytebase-policies/
├── opal.yaml                  # OPAL configuration
├── base/                      # Always-loaded base policies
│   ├── access_control.rego
│   ├── masking.rego
│   └── governance.rego
├── environments/              # Environment-specific overrides
│   ├── production/
│   │   ├── strict_access.rego
│   │   └── change_window.rego
│   └── development/
│       └── relaxed_access.rego
├── compliance/                # Compliance policies
│   ├── gdpr/
│   └── pci-dss/
├── tests/                     # OPA test suites
│   ├── access_test.rego
│   └── masking_test.rego
└── data/                      # Static data for policies
    └── change_windows.json
```

---

## 6. Security Mitigations

| Concern | Solution |
|---|---|
| OPAL server auth | JWT token, stored in vault (CR-VLT-001) |
| Git repo access | SSH key or deploy token, stored in vault |
| Data in transit | TLS for all OPAL ↔ Bytebase communication |
| Data publisher endpoint | Internal HMAC auth (`BB_INTERNAL_AUTH_ENABLED`) |
| Policy rollback | Git revert → OPAL auto-distributes previous version |
| Multi-tenant isolation | OPAL topics scoped per workspace |

---

## 7. Integration with Existing Features

| Feature | Integration |
|---|---|
| NATSBus (`BB_USE_NATS_BUS`) | Policy events published to NATS subjects alongside Go channels |
| HA Leader Election | Data publisher only runs on leader (existing heartbeat runner pattern) |
| Feature Flag | `BB_OPAL_ENABLED=true` — new flag alongside `BB_USE_GATEWAY` |
| Prometheus | OPAL sync metrics: `opal_sync_total`, `opal_sync_duration_seconds` |

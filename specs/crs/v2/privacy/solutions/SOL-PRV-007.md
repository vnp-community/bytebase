# Solution: CR-PRV-007 — Environment Data Isolation

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-007                |
| **Solution**   | SOL-PRV-007               |
| **Status**     | Proposed                  |
| **Complexity** | Very High                 |

---

## 1. Tóm tắt giải pháp

Xây dựng **Isolation Policy Engine** tại L5 (`component/privacy/isolation.go`) tích hợp vào Environment/Instance management flow. Engine hook vào SchemaSync runner (L6) và DatabaseMigrateExecutor (L6 `runner/taskrun/`) để enforce isolation rules khi data di chuyển giữa environments. Tận dụng Environment Tiers (CR-ENT-019) cho tier classification và Anonymization Engine (SOL-PRV-002) cho auto-anonymization.

---

## 2. Architectural Alignment

```
L4 Service (DatabaseService / InstanceService)
  │  Clone/Sync request: prod → staging
  ▼
L5 Component (privacy/isolation.go)
  │  ├─ 1. Resolve source/target environment tiers
  │  ├─ 2. Evaluate isolation policy
  │  │     ├─ [Same tier] Allow
  │  │     ├─ [Higher→Lower] Check policy
  │  │     │     ├─ [No PII columns] Allow
  │  │     │     ├─ [Has PII] Require anonymization
  │  │     │     └─ [L4 data] Hard block
  │  │     └─ [Lower→Higher] Allow (no restriction)
  │  ├─ 3. Generate anonymization plan
  │  └─ 4. Dispatch to anonymization engine
  │
  ├──► L5 Component (privacy/anonymizer.go) — from SOL-PRV-002
  ├──► L5 Component (privacy/scanner.go) — from SOL-PRV-001
  ├──► L6 Runner (schemasync/) — hook post-sync verification
  └──► L8 Store (isolation_policy, environment) — existing + new

L6 Runner (monitor/data_flow.go)
  │  Periodic: monitor cross-env data access patterns
  └──► L5 Component (webhook/) — alert on violations
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/privacy/isolation.go` | Isolation policy engine |
| **L5 — Component** | `component/privacy/sync_pipeline.go` | Privacy pipeline for cross-env sync |
| **L5 — Component** | `component/privacy/anonymizer.go` | Auto-anonymization (SOL-PRV-002) |
| **L6 — Runner** | `runner/schemasync/` | Hook post-sync PII verification |
| **L6 — Runner** | `runner/taskrun/database_migrate_executor.go` | Enforce isolation on data migration |
| **L6 — Runner** | `runner/monitor/data_flow.go` | Cross-env data flow monitoring (NEW) |
| **L4 — Service** | `api/v1/database_service.go` | Enforce isolation on clone/transfer |
| **L8 — Store** | `store/environment.go` | Existing env store + tier metadata |
| **L8 — Store** | `store/isolation_policy.go` | Isolation policy CRUD |
| **L9 — Enterprise** | `feature.go` | `FeatureDataIsolation` gate |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Isolation Policy Engine

**File**: `backend/component/privacy/isolation.go`

```go
type IsolationEngine struct {
    store      *store.Store
    scanner    *PIIScanner
    anonymizer *AnonymizationEngine
}

type IsolationDecision int
const (
    IsolationAllow         IsolationDecision = iota // no restriction
    IsolationAnonymize                               // allow with anonymization
    IsolationApprovalNeeded                          // needs admin approval + anonymization
    IsolationBlock                                    // hard block
)

func (e *IsolationEngine) Evaluate(ctx context.Context, 
    sourceEnv, targetEnv *store.EnvironmentMessage) (IsolationDecision, *AnonymizationPlan, error) {
    
    // 1. Resolve environment tiers
    sourceTier := e.getEnvTier(sourceEnv) // PRODUCTION, STAGING, DEVELOPMENT
    targetTier := e.getEnvTier(targetEnv)
    
    // 2. Same tier or lower→higher: no restriction
    if sourceTier <= targetTier {
        return IsolationAllow, nil, nil
    }
    
    // 3. Higher→Lower (e.g., PRODUCTION → STAGING): apply policy
    policy, _ := e.store.GetIsolationPolicy(ctx, sourceEnv.UID)
    
    // 4. Scan for PII in source database
    piiResults, _ := e.scanner.GetScanResults(ctx, sourceDatabaseUID)
    
    // 5. Evaluate per classification level
    hasL4 := false
    columnsToAnonymize := []ColumnRef{}
    for _, result := range piiResults {
        classification := e.store.GetColumnClassification(ctx, result.Column)
        switch {
        case classification.Level == "L4":
            if policy.BlockL4 { // default: true, non-overridable
                hasL4 = true
            }
        case classification.Level >= "L2":
            columnsToAnonymize = append(columnsToAnonymize, result.Column)
        }
    }
    
    if hasL4 {
        return IsolationBlock, nil, nil
    }
    
    if len(columnsToAnonymize) > 0 {
        plan := e.buildAnonymizationPlan(ctx, columnsToAnonymize, policy)
        if policy.RequireApproval {
            return IsolationApprovalNeeded, plan, nil
        }
        return IsolationAnonymize, plan, nil
    }
    
    return IsolationAllow, nil, nil
}
```

### 3.2 L5 — Cross-Environment Sync Pipeline

**File**: `backend/component/privacy/sync_pipeline.go`

```go
type SyncPrivacyPipeline struct {
    isolation  *IsolationEngine
    anonymizer *AnonymizationEngine
    scanner    *PIIScanner
}

func (p *SyncPrivacyPipeline) Process(ctx context.Context, syncReq *SyncRequest) error {
    // 1. Evaluate isolation policy
    decision, plan, _ := p.isolation.Evaluate(ctx, syncReq.SourceEnv, syncReq.TargetEnv)
    
    switch decision {
    case IsolationBlock:
        return errors.New("data sync blocked: contains L4 restricted data")
    case IsolationApprovalNeeded:
        return p.requestApproval(ctx, syncReq, plan) // async approval
    case IsolationAnonymize:
        return p.executeWithAnonymization(ctx, syncReq, plan)
    case IsolationAllow:
        return nil // proceed without modification
    }
    return nil
}

func (p *SyncPrivacyPipeline) executeWithAnonymization(ctx context.Context, 
    syncReq *SyncRequest, plan *AnonymizationPlan) error {
    
    // Stream data through anonymization pipeline
    // ... execute sync with anonymized data ...
    
    // Post-sync verification: scan target for raw PII
    verifyResults, _ := p.scanner.Scan(ctx, ScanConfig{
        DatabaseUID: syncReq.TargetDB.UID,
        ScanType:    ScanTypeFull,
    })
    
    for _, result := range verifyResults {
        if result.Confidence > 0.8 && !result.IsAnonymized {
            // Alert: raw PII detected in target!
            return fmt.Errorf("verification failed: raw PII detected in %s.%s", 
                result.Column.Table, result.Column.Name)
        }
    }
    
    return nil
}
```

### 3.3 L6 — Data Flow Monitor

**File**: `backend/runner/monitor/data_flow.go`

```go
type DataFlowMonitor struct {
    store   *store.Store
    webhook *webhook.Manager
}

// Periodic monitoring: detect unauthorized cross-env data access
func (m *DataFlowMonitor) Run(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    for {
        select {
        case <-ticker.C:
            // Check audit logs for cross-environment queries
            flows, _ := m.store.DetectCrossEnvAccess(ctx, 5*time.Minute)
            for _, flow := range flows {
                if flow.SourceTier > flow.TargetTier {
                    m.webhook.Send(ctx, webhook.DataFlowAlert, flow)
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 3.4 L8 — Database Schema

```sql
CREATE TABLE isolation_policy (
    id BIGSERIAL PRIMARY KEY,
    workspace_uid BIGINT NOT NULL,
    source_env_uid BIGINT, -- NULL = workspace-wide default
    block_l4 BOOLEAN NOT NULL DEFAULT true,
    auto_anonymize BOOLEAN NOT NULL DEFAULT true,
    require_approval BOOLEAN NOT NULL DEFAULT true,
    max_sync_volume_bytes BIGINT DEFAULT 1073741824, -- 1GB
    anonymization_policy_id BIGINT REFERENCES anonymization_policy(id),
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_uid, source_env_uid)
);

CREATE TABLE data_flow_log (
    id BIGSERIAL PRIMARY KEY,
    source_env_uid BIGINT NOT NULL,
    target_env_uid BIGINT NOT NULL,
    source_tier TEXT NOT NULL,
    target_tier TEXT NOT NULL,
    operation TEXT NOT NULL, -- CLONE, SYNC, QUERY, EXPORT
    user_uid BIGINT NOT NULL,
    anonymized BOOLEAN NOT NULL DEFAULT false,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_data_flow_log_time ON data_flow_log (created_ts DESC);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Direct DB access bypasses isolation | Isolation enforced at Bytebase API level; direct DB access monitored via audit |
| Sync pipeline performance | Streaming anonymization, batch processing |
| L4 block override attempt | `block_l4` is non-overridable in code |
| Post-sync verification false negative | Conservative confidence threshold (0.8) + human review |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-019 (Env Tiers) | Tier classification drives isolation rules |
| CR-PRV-001 (PII Discovery) | PII scan results drive isolation decisions |
| CR-PRV-002 (Anonymization) | Auto-anonymization during cross-env sync |
| CR-ENT-013 (Classification) | Classification levels determine block/anonymize |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Isolation policy engine + DB schema | Sprint 1 |
| 2 | Cross-environment sync pipeline + anonymization | Sprint 2 |
| 3 | Post-sync verification + alerts | Sprint 3 |
| 4 | Data flow monitoring + dashboard | Sprint 3 |

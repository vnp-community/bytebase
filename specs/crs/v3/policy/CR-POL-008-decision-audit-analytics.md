# Change Request: Policy Decision Audit & Analytics

| Field | Value |
|---|---|
| **CR ID** | CR-POL-008 |
| **Title** | Policy Decision Audit & Analytics |
| **Plan** | ENTERPRISE |
| **Priority** | P2 — Medium |
| **Status** | Draft |
| **Created** | 2026-05-17 |
| **Author** | VNP AI Ops Team |
| **Dependencies** | CR-POL-001, CR-POL-006 |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **Policy Decision Audit & Analytics** — hệ thống logging, tracing, và phân tích cho tất cả policy decisions. Mỗi evaluation được ghi lại chi tiết: ai yêu cầu, engine nào đánh giá, chính sách nào match, kết quả gì, và obligations nào được áp dụng.

### 1.2 Bối cảnh
Hiện tại Bytebase có AuditInterceptor ghi API calls vào `audit_log` table, nhưng:
- Không ghi **policy decision** chi tiết (chỉ ghi request/response)
- Không track engine nào đưa ra quyết định
- Không có analytics cho policy effectiveness
- Không hỗ trợ compliance reporting

### 1.3 Mục tiêu
- Decision log cho mỗi policy evaluation
- Integration với existing AuditLog
- Analytics dashboard: top denied policies, most evaluated rules, engine performance
- Compliance reporting: GDPR, SOX, PCI-DSS audit trails
- OPA Decision Log compatibility

---

## 2. Yêu cầu chức năng

### FR-001: Decision Log Model

```go
type PolicyDecisionLog struct {
    ID              string
    Timestamp       time.Time
    RequestID       string     // Correlation with API request

    // Who
    SubjectType     string
    SubjectID       string

    // What
    Action          string
    ResourceType    string
    ResourceID      string

    // Decision
    Engine          string     // Which engine made the decision
    PolicyID        string     // Which policy matched
    PolicyVersion   int
    Allowed         bool
    Reason          string
    EvaluationTimeMs int64

    // Obligations
    Obligations     []AppliedObligation

    // Context
    WorkspaceID     string
    ProjectID       string
    EnvironmentID   string
}
```

### FR-002: Decision Log Store

```sql
CREATE TABLE policy_decision_log (
    id TEXT NOT NULL,
    workspace TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    engine TEXT NOT NULL,
    policy_id TEXT,
    policy_version INTEGER,
    allowed BOOLEAN NOT NULL,
    reason TEXT,
    evaluation_time_ms INTEGER,
    obligations JSONB NOT NULL DEFAULT '[]',
    project TEXT,
    environment TEXT,
    PRIMARY KEY (workspace, id)
);

CREATE INDEX idx_decision_log_time ON policy_decision_log(workspace, timestamp DESC);
CREATE INDEX idx_decision_log_subject ON policy_decision_log(workspace, subject_id, timestamp DESC);
CREATE INDEX idx_decision_log_resource ON policy_decision_log(workspace, resource_type, resource_id);
CREATE INDEX idx_decision_log_denied ON policy_decision_log(workspace, allowed, timestamp DESC) WHERE NOT allowed;
```

### FR-003: Analytics Queries

| Metric | Query | Use Case |
|---|---|---|
| Top denied policies | Group by policy_id, count denied | Identify strict policies |
| Denial rate by engine | Group by engine, avg denied | Engine comparison |
| Avg evaluation time | Avg evaluation_time_ms by engine | Performance monitoring |
| Access patterns | Group by subject + resource + action | Usage analytics |
| Policy coverage | Distinct resources with evaluations | Identify unprotected resources |
| Compliance audit | Filter by time range + resource type | Audit reporting |

### FR-004: Prometheus Metrics

```go
var (
    policyEvalTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "policy_evaluation_total"},
        []string{"engine", "policy_category", "decision", "pep"},
    )
    policyEvalDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "policy_evaluation_duration_seconds"},
        []string{"engine"},
    )
    policyDecisionDenied = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "policy_decision_denied_total"},
        []string{"engine", "policy_id", "action"},
    )
)
```

### FR-005: OPA Decision Log Compatibility

Export decision logs in OPA Decision Log format for integration with Styra DAS, ELK, Splunk:

```json
{
    "labels": {"id": "bytebase-001", "version": "0.1.0"},
    "decision_id": "dec-xyz",
    "path": "data/bytebase/authz/allow",
    "input": {...},
    "result": true,
    "timestamp": "2026-05-17T10:30:00Z",
    "metrics": {"timer_rego_query_eval_ns": 150000}
}
```

### FR-006: Data Retention & Cleanup

- Configurable retention period (default: 90 days)
- Auto-cleanup via existing DataCleaner runner
- Aggregate old logs into summary tables before purging

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Decision Logger | `backend/component/policy/audit/logger.go` | New |
| Decision Store | `backend/store/policy_decision_log.go` | New |
| Analytics Service | `backend/component/policy/audit/analytics.go` | New |
| Prometheus Metrics | `backend/component/policy/audit/metrics.go` | New |
| OPA Log Exporter | `backend/component/policy/audit/opa_export.go` | New |
| DataCleaner | `backend/runner/cleaner/cleaner.go` | Extend: decision log cleanup |
| Proto: DecisionLog | `proto/v1/policy_audit.proto` | New: decision log API |

---

## 4. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | Log policy decision | Entry in decision_log table |
| TC-002 | Query denied decisions | Filtered results returned |
| TC-003 | Analytics: top denied policies | Correct aggregation |
| TC-004 | OPA decision log export | Valid OPA log format |
| TC-005 | Data retention cleanup | Old entries purged |
| TC-006 | Prometheus metrics increment | Counters updated per evaluation |
| TC-007 | High-volume logging (10K/s) | No performance degradation |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Decision log model + store | Sprint 1 |
| Phase 2 | Logger integration with PolicyManager | Sprint 1-2 |
| Phase 3 | Prometheus metrics | Sprint 2 |
| Phase 4 | Analytics queries + API | Sprint 3 |
| Phase 5 | OPA log export | Sprint 3-4 |
| Phase 6 | Data retention + cleanup | Sprint 4 |

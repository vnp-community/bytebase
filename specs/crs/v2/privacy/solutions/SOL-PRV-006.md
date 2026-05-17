# Solution: CR-PRV-006 — Data Export Access Control

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-006                |
| **Solution**   | SOL-PRV-006               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng **Export component** hiện có tại L5 (`component/export/`) và **DataExportExecutor** tại L6 (`runner/taskrun/data_export_executor.go`) với privacy-aware pipeline. Thêm export policy engine, rate limiter, và approval workflow. Tận dụng MaskingEvaluator hiện có (L4 `masking_evaluator.go`) và Anonymization Engine (SOL-PRV-002) để apply protection trước khi export.

---

## 2. Architectural Alignment

```
L1 Presentation (SQL Editor: Export button)
  │
  ▼
L4 Service (sql_service.go → Export flow)
  │  ├─ 1. Parse query → identify columns
  │  ├─ 2. NEW: Check export policy (classification-based)
  │  ├─ 3. NEW: Rate limit check
  │  │     ├─ [Within limits] Continue
  │  │     └─ [Exceeded] Block + alert
  │  ├─ 4. NEW: PII scan on result columns
  │  │     ├─ [No PII] Auto-approve → export
  │  │     └─ [Has PII] Approval required
  │  │           └─ Bus.ApprovalCheckChan → Approval Runner (L6)
  │  ├─ 5. Apply protection mode:
  │  │     ├─ MaskingEvaluator (existing L4) → Masked export
  │  │     ├─ AnonymizationEngine (SOL-PRV-002) → Anonymized export
  │  │     └─ Raw → requires elevated approval
  │  └─ 6. Export via component/export/ (CSV, Excel, JSON)
  │
  ▼
L5 Component (export/) — existing, enhanced with privacy pipeline
  │
  ▼
L3 Security (Audit) — export audit trail
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/export/policy.go` | Export policy engine (NEW) |
| **L5 — Component** | `component/export/rate_limiter.go` | Export rate limiting (NEW) |
| **L5 — Component** | `component/export/privacy.go` | Privacy pipeline integration (NEW) |
| **L5 — Component** | `component/export/` | Existing CSV/Excel/JSON export |
| **L4 — Service** | `api/v1/sql_service.go` (77KB) | Enhance export flow with privacy checks |
| **L4 — Service** | `masking_evaluator.go` (12KB) | Reuse for masked export mode |
| **L6 — Runner** | `runner/taskrun/data_export_executor.go` | Enhance async export with policy |
| **L8 — Store** | `store/export_policy.go` | Export policy + rate limit persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureExportDLP` gate |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Export Policy Engine

**File**: `backend/component/export/policy.go`

```go
type ExportPolicyEngine struct {
    store          *store.Store
    piiScanner     *privacy.PIIScanner
    maskEvaluator  *MaskingEvaluator
}

type ExportDecision int
const (
    ExportAutoApprove    ExportDecision = iota // no PII, export freely
    ExportApprovalNeeded                        // PII detected, needs approval
    ExportBlocked                               // L4 classified, hard block
)

func (e *ExportPolicyEngine) Evaluate(ctx context.Context, 
    columns []ColumnMeta, project string) (ExportDecision, *ExportProtection) {
    
    policy, _ := e.store.GetExportPolicy(ctx, project)
    
    // Check each column against classification
    for _, col := range columns {
        classification := e.store.GetColumnClassification(ctx, col)
        switch {
        case classification.Level == "L4": // RESTRICTED
            return ExportBlocked, nil
        case classification.Level >= "L2": // SENSITIVE+
            protection := &ExportProtection{
                Mode:    policy.DefaultProtectionMode, // MASKED or ANONYMIZED
                Columns: append(protection.Columns, col),
            }
            return ExportApprovalNeeded, protection
        }
    }
    
    return ExportAutoApprove, nil
}
```

### 3.2 L5 — Rate Limiter

**File**: `backend/component/export/rate_limiter.go`

```go
type ExportRateLimiter struct {
    store *store.Store
    // In-memory counters with periodic sync to DB
    counters sync.Map // userUID → *ExportCounter
}

type ExportLimits struct {
    MaxRowsPerRequest   int   // default: 10,000
    MaxExportsPerDay    int   // default: 10
    MaxVolumeBytesPerDay int64 // default: 100MB
}

func (r *ExportRateLimiter) Check(ctx context.Context, userUID int64, 
    estimatedRows int, estimatedBytes int64) error {
    
    limits := r.getLimits(ctx, userUID) // per-role limits
    counter := r.getCounter(userUID)
    
    if estimatedRows > limits.MaxRowsPerRequest {
        return status.Errorf(codes.ResourceExhausted, 
            "export exceeds max rows: %d > %d", estimatedRows, limits.MaxRowsPerRequest)
    }
    if counter.DailyExports >= limits.MaxExportsPerDay {
        return status.Errorf(codes.ResourceExhausted, "daily export limit reached")
    }
    if counter.DailyBytes+estimatedBytes > limits.MaxVolumeBytesPerDay {
        return status.Errorf(codes.ResourceExhausted, "daily export volume limit reached")
    }
    
    counter.DailyExports++
    counter.DailyBytes += estimatedBytes
    return nil
}
```

### 3.3 L4 — SQLService Export Enhancement

**File**: `backend/api/v1/sql_service.go` (modify existing export section)

```go
func (s *SQLService) Export(ctx context.Context, req *v1pb.ExportRequest) (*v1pb.ExportResponse, error) {
    // 1. Existing: ACL check, feature gate
    
    // 2. NEW: Rate limit check
    if err := s.exportRateLimiter.Check(ctx, user.UID, estimatedRows, estimatedBytes); err != nil {
        return nil, err
    }
    
    // 3. NEW: Export policy evaluation
    decision, protection := s.exportPolicy.Evaluate(ctx, columns, project)
    switch decision {
    case ExportBlocked:
        return nil, status.Errorf(codes.PermissionDenied, "export blocked: contains restricted data")
    case ExportApprovalNeeded:
        if !req.HasApproval {
            // Create pending export request, wait for approval
            return s.createPendingExport(ctx, req, protection)
        }
    }
    
    // 4. Apply protection mode
    var rows []*v1pb.QueryRow
    switch protection.Mode {
    case ProtectionMasked:
        rows = s.queryResultMasker.MaskResults(ctx, rawRows, columns)
    case ProtectionAnonymized:
        rows, _ = s.anonymizer.Anonymize(ctx, rawRows, columns, anonymPolicy)
    default:
        rows = rawRows
    }
    
    // 5. Export with existing component/export/
    data, _ := s.exporter.Export(ctx, rows, req.Format) // CSV, Excel, JSON
    
    // 6. NEW: Export audit trail
    s.logExportAudit(ctx, ExportAuditEntry{
        UserUID:        user.UID,
        Query:          s.redactor.Redact(req.Statement),
        Columns:        columns,
        RowCount:       len(rows),
        Format:         req.Format,
        ProtectionMode: protection.Mode,
        ExportSize:     len(data),
        TraceID:        generateTraceID(),
    })
    
    return &v1pb.ExportResponse{Content: data}, nil
}
```

### 3.4 L8 — Database Schema

```sql
CREATE TABLE export_policy (
    id BIGSERIAL PRIMARY KEY,
    workspace_uid BIGINT NOT NULL,
    project TEXT, -- NULL = workspace-wide default
    default_protection_mode TEXT NOT NULL DEFAULT 'MASKED',
    max_rows_per_request INT NOT NULL DEFAULT 10000,
    max_exports_per_day INT NOT NULL DEFAULT 10,
    max_volume_bytes_per_day BIGINT NOT NULL DEFAULT 104857600, -- 100MB
    require_approval_above TEXT NOT NULL DEFAULT 'L2', -- classification level
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE export_audit (
    id BIGSERIAL PRIMARY KEY,
    user_uid BIGINT NOT NULL,
    query_hash TEXT NOT NULL,
    columns TEXT[] NOT NULL,
    row_count INT NOT NULL,
    format TEXT NOT NULL,
    protection_mode TEXT NOT NULL,
    export_size_bytes BIGINT NOT NULL,
    trace_id TEXT NOT NULL,
    approval_uid BIGINT,
    client_ip TEXT,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_export_audit_user ON export_audit (user_uid, created_ts DESC);
CREATE INDEX idx_export_audit_trace ON export_audit (trace_id);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Rate limit bypass via multiple sessions | Counter per user UID, not per session |
| Export via API (bypass UI controls) | Rate limiter + policy check in service layer |
| Large result set estimation | Pre-query COUNT(*) for row estimation |
| Approved export replay | Approval token single-use + expiry |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-005 (Restrict Copying) | Copy restriction + export control = defense-in-depth |
| CR-ENT-012 (Data Masking) | Masked export mode reuses masking pipeline |
| CR-PRV-002 (Anonymization) | Anonymized export mode uses anonymization engine |
| CR-ENT-013 (Classification) | Classification level drives export policy |
| CR-PRV-005 (Privacy Audit) | Export events in privacy-preserving audit trail |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Export policy engine + rate limiter | Sprint 1 |
| 2 | SQLService export enhancement + masked mode | Sprint 2 |
| 3 | Anonymized export mode integration | Sprint 2 |
| 4 | Export audit trail + compliance reports | Sprint 3 |
| 5 | Approval workflow for sensitive exports | Sprint 3 |

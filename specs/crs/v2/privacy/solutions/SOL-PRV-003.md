# Solution: CR-PRV-003 — User Consent & Data Subject Rights

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-003                |
| **Solution**   | SOL-PRV-003               |
| **Status**     | Proposed                  |
| **Complexity** | Very High                 |

---

## 1. Tóm tắt giải pháp

Xây dựng **DSR (Data Subject Request) Service** mới tại L4 (`api/v1/dsr_service.go`) sử dụng Issue/Plan/Rollout workflow pattern hiện có của Bytebase. DSR requests được model hóa tương tự Issues — tận dụng Approval Runner (L6) cho approval workflow, PII Scanner (SOL-PRV-001) cho data discovery, và Anonymization Engine (SOL-PRV-002) cho erasure/anonymization. Consent records lưu tại L8 Store.

---

## 2. Architectural Alignment

```
L1 Presentation (DSRDashboard.tsx)
  │  DSR submission / management
  ▼
L4 Service (dsr_service.go)
  │  ├─ CreateDSR → maps to Issue-like workflow
  │  ├─ ProcessDSR → orchestrates across databases
  │  └─ VerifyDSR → post-execution verification
  │
  ├──► L5 Component (privacy/scanner.go) — locate data subject's PII
  ├──► L5 Component (privacy/anonymizer.go) — anonymize for erasure
  ├──► L5 Component (privacy/erasure.go) — cross-DB deletion
  ├──► L6 Runner (approval/) — approval workflow reuse
  ├──► L7 Plugin (DB drivers) — execute DELETE/UPDATE across engines
  └──► L8 Store (consent.go, dsr.go) — persist DSR + consent records
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `api/v1/dsr_service.go` | DSR CRUD + workflow orchestration |
| **L4 — Service** | `api/v1/consent_service.go` | Consent records management |
| **L5 — Component** | `component/privacy/erasure.go` | Cross-database erasure engine |
| **L5 — Component** | `component/privacy/scanner.go` | Data subject PII discovery (SOL-PRV-001) |
| **L6 — Runner** | `runner/approval/` | Reuse existing approval workflow |
| **L6 — Runner** | `runner/dsr/` | DSR SLA monitoring + escalation |
| **L7 — Plugin** | DB drivers | Execute DELETE/UPDATE on target databases |
| **L8 — Store** | `store/dsr.go`, `store/consent.go` | Persist DSR + consent |
| **L9 — Enterprise** | `feature.go` | `FeatureDataSubjectRights` gate |

---

## 3. Chi tiết Implementation

### 3.1 L4 — DSR Service

**File**: `backend/api/v1/dsr_service.go`

```go
type DSRService struct {
    store       *store.Store
    piiScanner  *privacy.PIIScanner
    erasure     *privacy.ErasureEngine
    anonymizer  *privacy.AnonymizationEngine
    bus         *bus.Bus
}

// DSR workflow reuses Issue pattern:
// DSR → Impact Assessment → Approval → Execution Plan → Execute → Verify → Close
func (s *DSRService) CreateDSR(ctx context.Context, req *v1pb.CreateDSRRequest) (*v1pb.DSR, error) {
    // 1. Validate request type (ACCESS, RECTIFICATION, ERASURE, RESTRICTION, PORTABILITY)
    // 2. Identity verification check
    // 3. Auto-discover data subject's data via PII Scanner
    affectedColumns, _ := s.piiScanner.FindDataSubject(ctx, req.SubjectIdentifier)
    
    // 4. Generate impact assessment
    impact := s.assessImpact(ctx, affectedColumns, req.Type)
    
    // 5. Create DSR record + trigger approval
    dsr, _ := s.store.CreateDSR(ctx, &store.DSRMessage{
        Type:            req.Type,
        SubjectID:       req.SubjectIdentifier,
        Status:          store.DSRStatusPendingApproval,
        AffectedColumns: affectedColumns,
        Impact:          impact,
        SLADeadline:     time.Now().AddDate(0, 0, 30), // GDPR 30-day SLA
    })
    
    // 6. Trigger approval via Bus (reuse existing ApprovalRunner)
    s.bus.ApprovalCheckChan <- bus.IssueRef{UID: dsr.UID}
    return convertDSR(dsr), nil
}
```

### 3.2 L5 — Erasure Engine

**File**: `backend/component/privacy/erasure.go`

```go
type ErasureEngine struct {
    dbFactory  *dbfactory.DBFactory
    store      *store.Store
    anonymizer *AnonymizationEngine
}

type ErasureMethod int
const (
    ErasureHardDelete   ErasureMethod = iota // physical DELETE
    ErasureSoftDelete                         // SET deleted_at = now()
    ErasureAnonymize                          // replace PII with anonymous data
)

func (e *ErasureEngine) Execute(ctx context.Context, plan *ErasurePlan) (*ErasureReport, error) {
    report := &ErasureReport{}
    
    for _, target := range plan.Targets {
        driver, _ := e.dbFactory.GetDriver(ctx, target.Instance)
        defer driver.Close(ctx)
        
        switch plan.Method {
        case ErasureHardDelete:
            affected, _ := driver.Execute(ctx,
                fmt.Sprintf("DELETE FROM %s WHERE %s = $1", target.Table, target.IdentifierCol),
                db.ExecuteOptions{Args: []any{plan.SubjectID}})
            report.AddResult(target, affected, "DELETED")
            
        case ErasureAnonymize:
            // Use anonymization engine to replace PII
            anonymized := e.anonymizer.AnonymizeRecord(ctx, target, plan.SubjectID)
            affected, _ := driver.Execute(ctx, anonymized.UpdateSQL, 
                db.ExecuteOptions{Args: anonymized.Args})
            report.AddResult(target, affected, "ANONYMIZED")
        }
    }
    
    // Verification scan: confirm no raw PII remains
    report.Verified = e.verify(ctx, plan)
    return report, nil
}
```

### 3.3 L6 — DSR SLA Runner

**File**: `backend/runner/dsr/sla_runner.go`

```go
type DSRSLARunner struct {
    store     *store.Store
    webhook   *webhook.Manager
}

// Runs periodically (every hour) to check DSR SLA compliance
func (r *DSRSLARunner) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    for {
        select {
        case <-ticker.C:
            dsrs, _ := r.store.ListPendingDSRs(ctx)
            for _, dsr := range dsrs {
                remaining := time.Until(dsr.SLADeadline)
                if remaining < 7*24*time.Hour { // 7 days warning
                    r.webhook.Send(ctx, webhook.DSRSLAWarning, dsr)
                }
                if remaining < 0 { // SLA breached
                    r.webhook.Send(ctx, webhook.DSRSLABreached, dsr)
                    r.store.UpdateDSRStatus(ctx, dsr.UID, store.DSRStatusSLABreached)
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
CREATE TABLE dsr (
    id BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('ACCESS','RECTIFICATION','ERASURE','RESTRICTION','PORTABILITY','OBJECTION')),
    subject_identifier TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING_VERIFICATION',
    affected_data JSONB, -- discovered columns/tables
    impact_assessment JSONB,
    execution_plan JSONB,
    execution_report JSONB,
    sla_deadline TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    submitted_by BIGINT NOT NULL,
    approved_by BIGINT,
    workspace_uid BIGINT NOT NULL
);

CREATE TABLE consent_record (
    id BIGSERIAL PRIMARY KEY,
    user_uid BIGINT NOT NULL,
    purpose TEXT NOT NULL,
    scope TEXT NOT NULL,
    granted BOOLEAN NOT NULL DEFAULT true,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    workspace_uid BIGINT NOT NULL
);

CREATE INDEX idx_dsr_status ON dsr (status);
CREATE INDEX idx_dsr_sla ON dsr (sla_deadline) WHERE status NOT IN ('COMPLETED', 'CANCELLED');
CREATE INDEX idx_consent_user ON consent_record (user_uid);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Identity spoofing | Identity verification step before DSR processing |
| Cascade delete breaks FK | Impact assessment pre-checks FK dependencies |
| Backup data retention | Erasure report flags backup anonymization requirement |
| Legal hold conflict | Check legal hold (SOL-PRV-004) before erasure |
| DSR data itself is PII | DSR records subject to own retention policy |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-PRV-001 (PII Discovery) | Auto-discover data subject's PII across databases |
| CR-PRV-002 (Anonymization) | Anonymize as alternative to hard delete |
| CR-PRV-004 (Retention) | Legal hold suspends erasure |
| CR-ENT-003 (Audit Log) | All DSR actions fully audited |
| CR-ENT-007 (Approval) | DSR approval reuses existing workflow |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | DSR Service + database schema + basic workflow | Sprint 1 |
| 2 | Consent management + API | Sprint 2 |
| 3 | Erasure engine + cross-database execution | Sprint 3 |
| 4 | SLA runner + escalation + dashboard | Sprint 3 |
| 5 | Verification scan + compliance reporting | Sprint 4 |

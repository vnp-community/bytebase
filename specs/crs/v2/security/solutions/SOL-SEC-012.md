# Solution: CR-SEC-012 — Compliance Reporting Framework

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-012                |
| **Solution**   | SOL-SEC-012               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Xây dựng Compliance Engine (L5) kết nối với existing Store (L8) để thu thập evidence tự động từ audit logs, policies, settings, roles. Framework templates lưu dưới dạng JSONB (L8). Compliance assessment runner (L6) chạy scheduled. Report generator tạo PDF/HTML via Go templates.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4** | `compliance_service.go` (new) | API endpoints cho compliance operations |
| **L5** | `component/compliance/` (new) | Assessment engine, evidence collector |
| **L6** | `runner/compliance/` (new) | Scheduled compliance assessment |
| **L8** | Store (multiple tables) | Evidence sources: audit_log, policy, setting, role |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Compliance Engine

```go
type ComplianceEngine struct {
    store      *store.Store
    collectors []EvidenceCollector
}

type FrameworkTemplate struct {
    ID       string            `json:"id"`       // "soc2", "iso27001"
    Name     string            `json:"name"`
    Controls []ComplianceControl `json:"controls"`
}

type ComplianceControl struct {
    ID          string `json:"id"`       // "CC6.1"
    Name        string `json:"name"`     // "Logical and Physical Access Controls"
    Description string `json:"description"`
    EvidenceType string `json:"evidenceType"` // "audit_log", "policy", "setting"
    EvidenceQuery string `json:"evidenceQuery"` // CEL or SQL query
    Required    bool   `json:"required"`
}

func (e *ComplianceEngine) Assess(ctx context.Context, frameworkID string) (*AssessmentResult, error) {
    framework := e.getFramework(frameworkID)
    result := &AssessmentResult{Framework: frameworkID, Timestamp: time.Now()}

    for _, control := range framework.Controls {
        evidence, err := e.collectEvidence(ctx, control)
        controlResult := &ControlResult{
            ControlID: control.ID,
            Status:    evaluateCompliance(evidence, err),
            Evidence:  evidence,
        }
        result.Controls = append(result.Controls, controlResult)
    }

    result.Score = calculateScore(result.Controls)
    return result, nil
}
```

### 3.2 L5 — Evidence Collectors

```go
// Collect evidence from existing Store data
func (e *ComplianceEngine) collectEvidence(ctx context.Context, control ComplianceControl) (*Evidence, error) {
    switch control.EvidenceType {
    case "audit_log":
        // Check if audit logging is enabled and recent entries exist
        count, _ := e.store.CountAuditLogs(ctx, 30*24*time.Hour) // Last 30 days
        return &Evidence{Type: "audit_log", Value: count > 0, Detail: fmt.Sprintf("%d entries", count)}, nil

    case "policy":
        // Check if specific policy is configured (e.g., masking policy)
        policy, _ := e.store.GetPolicyByType(ctx, control.EvidenceQuery)
        return &Evidence{Type: "policy", Value: policy != nil, Detail: formatPolicy(policy)}, nil

    case "setting":
        // Check workspace setting (e.g., 2FA enabled, SSO configured)
        setting, _ := e.store.GetSetting(ctx, control.EvidenceQuery)
        return &Evidence{Type: "setting", Value: setting != nil, Detail: setting.Value}, nil

    case "role":
        // Check RBAC configuration (e.g., custom roles exist)
        roles, _ := e.store.ListRoles(ctx)
        return &Evidence{Type: "role", Value: len(roles) > 0, Detail: formatRoles(roles)}, nil
    }
    return nil, nil
}
```

### 3.3 L6 — Compliance Assessment Runner

```go
type ComplianceRunner struct {
    engine   *compliance.ComplianceEngine
    store    *store.Store
    webhook  *webhook.Manager
    interval time.Duration // Weekly
}

func (r *ComplianceRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    for {
        select {
        case <-ticker.C:
            frameworks := r.store.GetActiveFrameworks(ctx)
            for _, fw := range frameworks {
                result, _ := r.engine.Assess(ctx, fw.ID)
                r.store.SaveAssessmentResult(ctx, result)

                // Alert on compliance regression
                prev := r.store.GetPreviousAssessment(ctx, fw.ID)
                if prev != nil && result.Score < prev.Score {
                    r.webhook.NotifyComplianceRegression(ctx, fw, prev.Score, result.Score)
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
CREATE TABLE compliance_framework (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    controls    JSONB NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE compliance_assessment (
    id          BIGSERIAL PRIMARY KEY,
    framework_id TEXT NOT NULL REFERENCES compliance_framework(id),
    score       FLOAT NOT NULL,
    results     JSONB NOT NULL,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_assessment_framework ON compliance_assessment (framework_id, created_ts DESC);
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-003 (Audit Log) | Audit log as primary evidence source |
| CR-ENT-012 (Data Masking) | Masking policies as evidence |
| CR-SEC-011 (Tamper-Proof) | Integrity verification as evidence |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Framework templates + evidence collectors | Sprint 1 |
| 2 | Assessment engine + API | Sprint 2 |
| 3 | Compliance runner + regression alerts | Sprint 3 |
| 4 | Report generation (PDF/HTML) + UI | Sprint 4 |

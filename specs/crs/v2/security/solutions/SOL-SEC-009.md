# Solution: CR-SEC-009 — Secure Data Export & Transfer Controls

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-009                |
| **Solution**   | SOL-SEC-009               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Extend existing Export component (L5: `component/export/`) với approval workflow hook vào Approval Runner (L6), masking enforcement integration với MaskingEvaluator (L4), DLP scanner component (L5), và encrypted export support. Tận dụng existing DataExportExecutor (L6: `runner/taskrun/data_export_executor.go`).

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4** | `sql_service.go` (77KB) | Export request interception, row limit |
| **L4** | `masking_evaluator.go` (12KB) | Masking enforcement in exports |
| **L5** | `component/export/` | Export formatting + encryption |
| **L5** | `component/dlp/` (new) | Content scanning before export |
| **L6** | `runner/taskrun/data_export_executor.go` | Export task execution |

---

## 3. Chi tiết Implementation

### 3.1 L4 — Export Interception in SQLService

**File**: `backend/api/v1/sql_service.go`

```go
func (s *SQLService) Export(ctx context.Context, req *v1pb.ExportRequest) (*v1pb.ExportResponse, error) {
    user := getUserFromContext(ctx)
    env := s.resolveEnvironment(ctx, req.Database)

    // Check export row limit
    rowCount := s.estimateRowCount(ctx, req.Statement, req.Database)
    exportPolicy := s.getExportPolicy(ctx, env)

    if rowCount > exportPolicy.MaxRows {
        return nil, status.Errorf(codes.FailedPrecondition,
            "export exceeds row limit (%d > %d), approval required", rowCount, exportPolicy.MaxRows)
    }

    // Check if approval required for this environment
    if exportPolicy.RequireApproval && env.Tier == "PRODUCTION" {
        return s.createExportApprovalRequest(ctx, user, req)
    }

    // Execute export with masking
    results, _ := s.executeQuery(ctx, req)
    maskedResults := s.applyMasking(ctx, results) // Reuse existing MaskingEvaluator

    // DLP scan before export
    if exportPolicy.DLPEnabled {
        violations := s.dlpScanner.Scan(maskedResults)
        if len(violations) > 0 {
            s.alertSecurityTeam(ctx, "dlp_violation", violations)
            return nil, status.Errorf(codes.FailedPrecondition, "export blocked: sensitive data detected")
        }
    }

    // Export with optional encryption
    exportData := s.exportComponent.Export(maskedResults, req.Format)
    if exportPolicy.EncryptExports {
        exportData = s.encryptExport(exportData, exportPolicy.EncryptionPassword)
    }

    return &v1pb.ExportResponse{Data: exportData}, nil
}
```

### 3.2 L5 — DLP Scanner Component

**File**: `backend/component/dlp/scanner.go` (new)

```go
type DLPScanner struct {
    patterns []DLPPattern
}

type DLPPattern struct {
    Name    string
    Regex   *regexp.Regexp
    Type    string // "PII", "FINANCIAL", "HEALTH"
}

var defaultPatterns = []DLPPattern{
    {Name: "SSN", Regex: regexp.MustCompile(`\d{3}-\d{2}-\d{4}`), Type: "PII"},
    {Name: "CreditCard", Regex: regexp.MustCompile(`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`), Type: "FINANCIAL"},
    {Name: "Email", Regex: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), Type: "PII"},
}

func (s *DLPScanner) Scan(data []*v1pb.QueryRow) []DLPViolation {
    var violations []DLPViolation
    for _, row := range data {
        for _, value := range row.Values {
            for _, pattern := range s.patterns {
                if pattern.Regex.MatchString(value.GetStringValue()) {
                    violations = append(violations, DLPViolation{
                        Pattern: pattern.Name, Type: pattern.Type,
                    })
                }
            }
        }
    }
    return violations
}
```

### 3.3 L5 — Encrypted Export

**File**: `backend/component/export/encrypted.go` (new)

```go
func encryptExport(data []byte, password string) ([]byte, error) {
    // AES-256-GCM encryption with password-derived key
    key := argon2.IDKey([]byte(password), generateSalt(), 1, 64*1024, 4, 32)
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    return gcm.Seal(nonce, nonce, data, nil), nil
}
```

### 3.4 L8 — Export Policy Storage

```go
type ExportPolicy struct {
    MaxRows          int    `json:"maxRows"`          // Default: 10000
    MaxSizeBytes     int64  `json:"maxSizeBytes"`     // Default: 100MB
    RequireApproval  bool   `json:"requireApproval"`  // Per environment
    DLPEnabled       bool   `json:"dlpEnabled"`
    EncryptExports   bool   `json:"encryptExports"`
    AllowedFormats   []string `json:"allowedFormats"` // ["csv", "json"]
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-012 (Data Masking) | Masking enforced in exports |
| CR-ENT-005 (Restrict Copying) | Extended clipboard protection |
| CR-ENT-007 (Approval Workflow) | Export approval workflow |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Export policy + row/size limits | Sprint 1 |
| 2 | Masking enforcement in exports | Sprint 2 |
| 3 | Encrypted export + DLP scanner | Sprint 3 |
| 4 | Export approval workflow | Sprint 4 |

# Solution: CR-PRV-005 — Privacy-Preserving Query Audit

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-005                |
| **Solution**   | SOL-PRV-005               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng **Audit Interceptor** hiện có tại L3 (`api/v1/audit.go`, 25KB) với privacy-preserving pipeline. Thêm **Query Redactor** mới tại L5 (`component/privacy/redactor.go`) sử dụng SQL Parser (L7 `plugin/parser/`) để parse AST và redact string literals. Enhance `sanitizeRequestBody()` đã có trong SOL-ENT-003 với configurable detail levels và tiered access control trên `audit_log_service.go` (L4).

---

## 2. Architectural Alignment

```
L2 API Gateway → Request
  │
  ▼
L3 Security (Audit Interceptor — audit.go 25KB)
  │  ├─ 1. Classify method → determine audit detail level
  │  ├─ 2. Sanitize request body (existing: strip passwords)
  │  ├─ 3. NEW: Redact SQL query literals via Query Redactor
  │  ├─ 4. NEW: Filter sensitive parameters
  │  ├─ 5. NEW: Determine storage level (MINIMAL → FORENSIC)
  │  └─ 6. Write to L8 Store (audit_log table)
  │
  ▼
L5 Component (privacy/redactor.go)
  │  uses L7 Plugin (SQL Parser — ANTLR4) to parse + redact
  │
  ▼
L4 Service (audit_log_service.go — 7KB)
  │  NEW: Tiered access control on SearchAuditLogs
  │  ├─ Viewer: MINIMAL + STANDARD
  │  ├─ Auditor: + DETAILED
  │  └─ Forensic: + encrypted original (break-glass)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3 — Security** | `api/v1/audit.go` (25KB) | Enhance interceptor with privacy pipeline |
| **L5 — Component** | `component/privacy/redactor.go` | SQL literal redaction engine |
| **L5 — Component** | `component/privacy/param_filter.go` | Sensitive parameter filtering |
| **L7 — Plugin** | `plugin/parser/` | ANTLR4 SQL parsers for AST-based redaction |
| **L4 — Service** | `api/v1/audit_log_service.go` (7KB) | Tiered access control for audit queries |
| **L8 — Store** | `store/audit_log.go` | Add `detail_level` + `encrypted_original` columns |
| **L9 — Enterprise** | `feature.go` | `FeaturePrivacyAudit` gate |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Query Redactor

**File**: `backend/component/privacy/redactor.go`

```go
type QueryRedactor struct {
    parsers map[storepb.Engine]parser.Parser // reuse existing ANTLR parsers
}

type RedactionLevel int
const (
    RedactionOff          RedactionLevel = iota // no redaction
    RedactionLiteralsOnly                        // redact string/numeric literals
    RedactionFull                                // redact literals + table data refs
)

func (r *QueryRedactor) Redact(engine storepb.Engine, sql string, level RedactionLevel) (redacted string, hash string) {
    if level == RedactionOff {
        return sql, sha256Hex(sql)
    }
    
    // Use existing ANTLR parser (L7) to build AST
    p := r.parsers[engine]
    ast, err := p.Parse(sql)
    if err != nil {
        // Fallback: regex-based redaction for unparseable SQL
        return r.regexRedact(sql), sha256Hex(sql)
    }
    
    // Walk AST, replace string/numeric literals in WHERE, SET, VALUES, INSERT
    visitor := &RedactionVisitor{level: level}
    redacted = visitor.Visit(ast)
    
    return redacted, sha256Hex(sql)
}

// Regex fallback for engines without full parser support
func (r *QueryRedactor) regexRedact(sql string) string {
    // Replace string literals: 'anything' → '[REDACTED]'
    re := regexp.MustCompile(`'[^']*'`)
    return re.ReplaceAllString(sql, "'[REDACTED]'")
}
```

**Key design**: Tận dụng ANTLR4 parsers đã có cho 9 engines (pg, mysql, tidb, oracle, mssql, snowflake, oceanbase, redshift). Engines chưa có parser dùng regex fallback.

### 3.2 L3 — Audit Interceptor Enhancement

**File**: `backend/api/v1/audit.go` (modify existing)

```go
// Add to existing AuditInterceptor
func (a *AuditInterceptor) createAuditEntry(ctx context.Context, 
    method string, req proto.Message, resp proto.Message, err error) *store.AuditLogMessage {
    
    entry := &store.AuditLogMessage{
        Method:    method,
        UserUID:   extractUser(ctx),
        Status:    statusFromError(err),
        // ... existing fields ...
    }
    
    // NEW: Privacy-preserving enhancements
    detailLevel := a.resolveDetailLevel(ctx, method)
    entry.DetailLevel = detailLevel
    
    // Sanitize request (existing, enhanced)
    entry.Request = a.sanitizeRequest(req, detailLevel)
    
    // Redact SQL queries if present
    if sqlReq, ok := req.(*v1pb.QueryRequest); ok {
        redacted, hash := a.redactor.Redact(sqlReq.Engine, sqlReq.Statement, a.redactionLevel)
        entry.Request = redacted
        entry.QueryHash = hash
        
        // FORENSIC: encrypt original for break-glass access
        if detailLevel == DetailLevelForensic {
            encrypted, _ := a.encryptForForensic(sqlReq.Statement)
            entry.EncryptedOriginal = encrypted
        }
    }
    
    // Filter sensitive parameters
    entry.Request = a.paramFilter.Filter(entry.Request)
    
    return entry
}

func (a *AuditInterceptor) resolveDetailLevel(ctx context.Context, method string) DetailLevel {
    // SQL queries: STANDARD (redacted) by default
    // Auth events: DETAILED
    // Config changes: DETAILED (before/after)
    // Default: MINIMAL
    category := classifyMethod(method)
    policy, _ := a.store.GetAuditDetailPolicy(ctx)
    return policy.LevelFor(category)
}
```

### 3.3 L5 — Parameter Filter

**File**: `backend/component/privacy/param_filter.go`

```go
type ParamFilter struct {
    sensitiveFields []string
    sensitiveHeaders []string
}

var defaultSensitiveFields = []string{
    "password", "passwd", "secret", "token", "api_key", "apikey",
    "credential", "private_key", "access_token", "refresh_token",
    "connection_string", // mask password in connection strings
}

var defaultSensitiveHeaders = []string{
    "authorization", "cookie", "set-cookie", "x-api-key",
}

func (f *ParamFilter) Filter(body string) string {
    // Deep-clone protobuf JSON, strip sensitive fields
    var parsed map[string]any
    json.Unmarshal([]byte(body), &parsed)
    f.stripRecursive(parsed, f.sensitiveFields)
    result, _ := json.Marshal(parsed)
    return string(result)
}

func (f *ParamFilter) FilterConnectionString(connStr string) string {
    // postgresql://user:PASSWORD@host/db → postgresql://user:***@host/db
    u, err := url.Parse(connStr)
    if err == nil && u.User != nil {
        u.User = url.UserPassword(u.User.Username(), "***")
        return u.String()
    }
    return connStr
}
```

### 3.4 L4 — Tiered Access Control

**File**: `backend/api/v1/audit_log_service.go` (modify existing 7KB)

```go
func (s *AuditLogService) SearchAuditLogs(ctx context.Context, 
    req *v1pb.SearchAuditLogsRequest) (*v1pb.SearchAuditLogsResponse, error) {
    
    user := extractUser(ctx)
    accessTier := s.resolveAccessTier(ctx, user)
    
    // Filter results based on access tier
    maxDetailLevel := DetailLevelStandard
    switch accessTier {
    case AccessTierViewer:
        maxDetailLevel = DetailLevelStandard
    case AccessTierAuditor:
        maxDetailLevel = DetailLevelDetailed
    case AccessTierForensic:
        maxDetailLevel = DetailLevelForensic
        // Requires justification + logged as meta-audit
        if req.Justification == "" {
            return nil, status.Errorf(codes.InvalidArgument, "forensic access requires justification")
        }
        s.logMetaAudit(ctx, user, "FORENSIC_ACCESS", req.Justification)
    }
    
    logs, _ := s.store.SearchAuditLogs(ctx, &store.SearchAuditLogsQuery{
        Filter:         req.Filter,
        MaxDetailLevel: maxDetailLevel,
        // ... pagination ...
    })
    
    // Strip encrypted_original unless forensic tier
    if accessTier != AccessTierForensic {
        for _, log := range logs {
            log.EncryptedOriginal = nil
        }
    }
    
    return &v1pb.SearchAuditLogsResponse{AuditLogs: logs}, nil
}
```

### 3.5 L8 — Database Changes

```sql
-- Add columns to existing audit_log table
ALTER TABLE audit_log ADD COLUMN detail_level TEXT NOT NULL DEFAULT 'STANDARD';
ALTER TABLE audit_log ADD COLUMN query_hash TEXT;
ALTER TABLE audit_log ADD COLUMN encrypted_original BYTEA;

CREATE INDEX idx_audit_log_detail_level ON audit_log (detail_level);
CREATE INDEX idx_audit_log_query_hash ON audit_log (query_hash);
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Redaction bypass (dynamic SQL) | Regex fallback for unparseable statements |
| Forensic key compromise | Separate encryption key, stored in External Secret Manager |
| Meta-audit recursion | Meta-audit entries exempt from further meta-auditing |
| Parser performance | Cache parsed ASTs, async redaction for large queries |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-003 (Audit Log Full) | Extends audit interceptor with privacy features |
| CR-PRV-008 (User Activity Privacy) | Privacy audit protects user activity data |
| CR-ENT-015 (External Secret) | Encryption key for FORENSIC level |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Query redactor + ANTLR integration | Sprint 1 |
| 2 | Audit detail levels + DB migration | Sprint 2 |
| 3 | Parameter filter + sensitive field stripping | Sprint 2 |
| 4 | Tiered access control + break-glass | Sprint 3 |
| 5 | FORENSIC encryption + key management | Sprint 3 |

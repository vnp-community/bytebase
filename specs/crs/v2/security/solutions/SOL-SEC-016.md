# Solution: CR-SEC-016 — SQL Injection Deep Defense

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-016                |
| **Solution**   | SOL-SEC-016               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Defense-in-depth qua 3 layers: (1) Metadata store layer (L8) — audit tất cả queries dùng parameterized statements (pgx/v5 prepared), CI static analysis (gosec); (2) SQL Editor layer (L4) — statement type restriction, read-only connection pools via DBFactory (L5), row limits; (3) SQL AST validation (L7) — reuse existing ANTLR parsers để detect dangerous patterns trước execution.

---

## 2. Architectural Alignment

```
Layer 1: Metadata Store Protection (L8)
  store/*.go ──► pgx/v5 parameterized queries (already used)
  CI pipeline ──► gosec + sqlc static analysis

Layer 2: SQL Editor Controls (L4 + L5)
  SQLService ──► Statement type check
              ──► Row limit enforcement
              ──► DBFactory: read-only connection pool

Layer 3: SQL AST Validation (L7)
  plugin/parser/ ──► ANTLR4 AST analysis
  plugin/advisor/ ──► Security lint rules
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4** | `sql_service.go` (77KB) | Statement type restriction, row limits |
| **L5** | `component/dbfactory/` | Read-only connection pools |
| **L7** | `plugin/parser/` | ANTLR4 AST analysis for dangerous patterns |
| **L7** | `plugin/advisor/` (200+ rules) | Security-focused SQL lint rules |
| **L8** | `store/*.go` (74 files) | Parameterized query audit |
| **L10** | CI pipeline | gosec static analysis |

---

## 3. Chi tiết Implementation

### 3.1 L4 — Statement Type Restriction

**File**: `backend/api/v1/sql_service.go` (extend existing 77KB)

```go
func (s *SQLService) Query(ctx context.Context, req *v1pb.QueryRequest) (*v1pb.QueryResponse, error) {
    user := getUserFromContext(ctx)

    // Parse SQL to determine statement type using existing ANTLR parser (L7)
    stmtType, _ := s.parser.GetStatementType(req.Statement, instanceEngine)

    // Check permission for statement type
    policy := s.getQueryPolicy(ctx, user, instanceEnvironment)
    if !policy.AllowedStatements[stmtType] {
        return nil, status.Errorf(codes.PermissionDenied,
            "statement type %s not allowed for your role", stmtType)
    }

    // Block dangerous patterns
    if dangers := s.detectDangerousPatterns(req.Statement, instanceEngine); len(dangers) > 0 {
        if !policy.AllowDangerousStatements {
            return nil, status.Errorf(codes.FailedPrecondition,
                "dangerous statement detected: %s", strings.Join(dangers, ", "))
        }
        // Emit security event for admin review
        s.emitSecurityEvent(ctx, "dangerous_sql", dangers)
    }

    // Enforce row limit
    if policy.MaxRows > 0 {
        req.Statement = addRowLimit(req.Statement, policy.MaxRows, instanceEngine)
    }

    // Use read-only connection for DQL
    if stmtType == "SELECT" {
        return s.executeReadOnly(ctx, req)
    }

    return s.executeReadWrite(ctx, req)
}
```

### 3.2 L5 — Read-Only Connection Pool

**File**: `backend/component/dbfactory/factory.go` (extend existing)

```go
func (f *Factory) GetReadOnlyDriver(ctx context.Context, instance *store.InstanceMessage) (db.Driver, error) {
    // Use read-only DataSource if available
    roDS := instance.ReadOnlyDataSource()
    if roDS == nil {
        // Fallback: use primary with read-only transaction
        driver, _ := f.GetDriver(ctx, instance)
        return &ReadOnlyWrapper{driver: driver}, nil
    }
    return db.Open(ctx, instance.Engine, f.buildConfig(roDS))
}

type ReadOnlyWrapper struct {
    driver db.Driver
}

func (w *ReadOnlyWrapper) Execute(ctx context.Context, stmt string, opts db.ExecuteOptions) (int64, error) {
    return 0, status.Errorf(codes.FailedPrecondition, "read-only connection: write operations not allowed")
}

func (w *ReadOnlyWrapper) QueryConn(ctx context.Context, conn *sql.Conn, stmt string, qctx *db.QueryContext) ([]*v1pb.QueryResult, error) {
    // Execute in read-only transaction
    return w.driver.QueryConn(ctx, conn, "SET TRANSACTION READ ONLY; " + stmt, qctx)
}
```

### 3.3 L7 — Dangerous Pattern Detection

**File**: `backend/plugin/parser/security.go` (new — reuses existing ANTLR parsers)

```go
type SecurityAnalyzer struct {
    parsers map[storepb.Engine]parser.Parser // existing ANTLR parsers
}

func (a *SecurityAnalyzer) DetectDangerousPatterns(sql string, engine storepb.Engine) []string {
    var dangers []string
    p := a.parsers[engine]
    ast, _ := p.Parse(sql)

    // Walk AST for dangerous patterns
    walk(ast, func(node parser.Node) {
        switch n := node.(type) {
        case *parser.DropStatement:
            dangers = append(dangers, "DROP statement")
        case *parser.TruncateStatement:
            dangers = append(dangers, "TRUNCATE statement")
        case *parser.DeleteStatement:
            if n.WhereClause == nil {
                dangers = append(dangers, "DELETE without WHERE clause")
            }
        case *parser.UpdateStatement:
            if n.WhereClause == nil {
                dangers = append(dangers, "UPDATE without WHERE clause")
            }
        case *parser.GrantStatement, *parser.RevokeStatement:
            dangers = append(dangers, "DCL statement (GRANT/REVOKE)")
        case *parser.UnionStatement:
            // Potential UNION-based injection pattern
            if containsSubquery(n) {
                dangers = append(dangers, "UNION with subquery (potential injection)")
            }
        }
    })

    return dangers
}
```

### 3.4 L8 — Metadata Store Audit

Static analysis CI integration:

```yaml
# .github/workflows/security-scan.yml
- name: Run gosec
  run: |
    gosec -exclude=G101 -fmt=json ./backend/store/...
    # G101: Look for hardcoded credentials
    # Focus on SQL injection patterns in store layer

- name: Verify parameterized queries
  run: |
    # Custom check: no string concatenation in SQL
    grep -rn 'fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE\|fmt.Sprintf.*DELETE' \
      backend/store/ && exit 1 || echo "No SQL string concatenation found"
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-010 (SIEM) | Dangerous SQL events forwarded to SIEM |
| CR-SEC-017 (Vuln Scanning) | gosec integrated into CI pipeline |
| CR-ENT-012 (Data Masking) | Masking applied to query results |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Statement type restriction + read-only pool | Sprint 1 |
| 2 | Dangerous pattern detection (AST) | Sprint 2 |
| 3 | Metadata store audit + CI integration | Sprint 2 |
| 4 | Query logging + anomaly detection | Sprint 3 |

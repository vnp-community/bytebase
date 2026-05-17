# Solution: CR-PRV-001 — PII Data Discovery & Inventory

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-001                |
| **Solution**   | SOL-PRV-001               |
| **Status**     | Proposed                  |
| **Complexity** | Very High                 |

---

## 1. Tóm tắt giải pháp

Xây dựng **PII Scanner** như một component mới tại L5 (`component/privacy/`), tích hợp vào SchemaSync runner (L6) để quét incremental. Scanner tận dụng DB Driver plugin (L7) cho `SyncDBSchema()` metadata + `QueryConn()` sample analysis. Kết quả lưu vào L8 Store qua bảng mới `pii_scan_result`. Dashboard tại L1 (React 19) hiển thị PII Inventory.

---

## 2. Architectural Alignment

```
L6 Runner (SchemaSync)
  │  trigger on schema change
  ▼
L5 Component (privacy/scanner.go)
  │  ├─ Column Name Matcher (regex patterns)
  │  ├─ Data Type Heuristic
  │  └─ Sample Analyzer (max 100 rows)
  │        │
  │        ▼
  │  L7 Plugin (DB Driver: QueryConn for sampling)
  │
  ▼
L8 Store (pii_scan_result, pii_scan_job)
  │
  ▼
L4 Service (pii_discovery_service.go)
  │  gRPC API: StartScan, GetScanStatus, ListResults
  ▼
L1 Presentation (PIIInventory.tsx dashboard)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/privacy/scanner.go` | Core PII scanner engine |
| **L5 — Component** | `component/privacy/patterns.go` | PII pattern registry (regex + heuristics) |
| **L6 — Runner** | `runner/schemasync/syncer.go` | Hook: trigger incremental scan after schema sync |
| **L7 — Plugin** | `plugin/db/*/` (22 drivers) | `SyncDBSchema()` metadata + `QueryConn()` sampling |
| **L4 — Service** | `api/v1/pii_discovery_service.go` | gRPC CRUD for scan management |
| **L8 — Store** | `store/pii_inventory.go` | Persist scan results + jobs |
| **L9 — Enterprise** | `feature.go` | `FeaturePIIDiscovery` gate |
| **L1 — Presentation** | `PIIInventory.tsx`, `PIIHeatmap.tsx` | Dashboard UI |

---

## 3. Chi tiết Implementation

### 3.1 L5 — PII Scanner Engine

**File**: `backend/component/privacy/scanner.go`

```go
type PIIScanner struct {
    store      *store.Store
    dbFactory  *dbfactory.DBFactory
    patterns   *PatternRegistry
}

type ScanConfig struct {
    DatabaseUID   int64
    ScanType      ScanType  // FULL | INCREMENTAL
    SampleSize    int       // max rows to sample (default 100)
    ChangedColumns []ColumnRef // for incremental scan
}

type ScanResult struct {
    Column          ColumnRef
    PIICategory     string    // EMAIL, PHONE, NATIONAL_ID, etc.
    DetectionMethod string    // COLUMN_NAME, DATA_TYPE, SAMPLE_ANALYSIS
    Confidence      float64   // 0.0 — 1.0
}

func (s *PIIScanner) Scan(ctx context.Context, config ScanConfig) ([]ScanResult, error) {
    // 1. Get schema metadata via store (already synced by SchemaSync)
    dbSchema, _ := s.store.GetDBSchema(ctx, config.DatabaseUID)
    
    // 2. Column name matching (fast, no DB connection needed)
    results := s.patterns.MatchColumns(dbSchema.Columns)
    
    // 3. Sample analysis (requires DB connection)
    driver, _ := s.dbFactory.GetDriver(ctx, instance)
    defer driver.Close(ctx)
    for _, col := range lowConfidenceColumns {
        samples, _ := driver.QueryConn(ctx, conn, 
            fmt.Sprintf("SELECT %s FROM %s LIMIT %d", col.Name, col.Table, config.SampleSize),
            nil)
        results = append(results, s.patterns.AnalyzeSamples(col, samples)...)
        // CRITICAL: samples NOT persisted — only analysis result stored
    }
    
    // 4. Store results
    s.store.UpsertPIIScanResults(ctx, results)
    return results, nil
}
```

### 3.2 L5 — Pattern Registry

**File**: `backend/component/privacy/patterns.go`

```go
type PatternRegistry struct {
    columnPatterns []ColumnPattern
    dataPatterns   []DataPattern
}

type ColumnPattern struct {
    Regex       *regexp.Regexp
    Category    string
    Confidence  float64
}

// Built-in patterns
var defaultPatterns = []ColumnPattern{
    {regexp.MustCompile(`(?i)(e[-_]?mail|email_addr)`), "EMAIL", 0.95},
    {regexp.MustCompile(`(?i)(phone|mobile|tel_?num)`), "PHONE", 0.90},
    {regexp.MustCompile(`(?i)(ssn|social_sec|cmnd|cccd)`), "NATIONAL_ID", 0.95},
    {regexp.MustCompile(`(?i)(password|passwd|pwd|hash)`), "CREDENTIALS", 0.99},
    {regexp.MustCompile(`(?i)(credit_?card|card_?num)`), "FINANCIAL", 0.95},
    {regexp.MustCompile(`(?i)(first_?name|last_?name|full_?name)`), "FULL_NAME", 0.85},
    {regexp.MustCompile(`(?i)(address|street|city|zip|postal)`), "ADDRESS", 0.80},
    {regexp.MustCompile(`(?i)(dob|birth_?date|birthday)`), "DOB", 0.90},
    {regexp.MustCompile(`(?i)(ip_?addr|user_?agent|device_?id)`), "IP_DEVICE", 0.85},
    {regexp.MustCompile(`(?i)(salary|balance|account_?num)`), "FINANCIAL", 0.85},
}
```

### 3.3 L6 — SchemaSync Hook

**File**: `backend/runner/schemasync/syncer.go` (modify existing)

```go
// In existing SyncDatabase method, after schema sync completes:
func (s *Syncer) syncDatabase(ctx context.Context, instance, database) {
    // ... existing schema sync code ...
    
    // NEW: Trigger incremental PII scan for changed columns
    if s.piiScanner != nil && s.licenseService.IsFeatureEnabled(ctx, FeaturePIIDiscovery) {
        changedCols := diffColumns(oldSchema, newSchema)
        if len(changedCols) > 0 {
            go s.piiScanner.Scan(ctx, ScanConfig{
                DatabaseUID:    database.UID,
                ScanType:       ScanTypeIncremental,
                ChangedColumns: changedCols,
            })
        }
    }
}
```

### 3.4 L8 — Database Schema

```sql
CREATE TABLE pii_scan_result (
    id BIGSERIAL PRIMARY KEY,
    database_uid BIGINT NOT NULL,
    schema_name TEXT NOT NULL DEFAULT '',
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    pii_category TEXT NOT NULL,
    detection_method TEXT NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL,
    is_confirmed BOOLEAN NOT NULL DEFAULT false,
    confirmed_by BIGINT,
    regulation_tags TEXT[] DEFAULT '{}',
    has_masking_policy BOOLEAN NOT NULL DEFAULT false,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (database_uid, schema_name, table_name, column_name)
);

CREATE INDEX idx_pii_scan_result_db ON pii_scan_result (database_uid);
CREATE INDEX idx_pii_scan_result_category ON pii_scan_result (pii_category);
CREATE INDEX idx_pii_scan_result_unconfirmed ON pii_scan_result (is_confirmed) WHERE NOT is_confirmed;

CREATE TABLE pii_scan_job (
    id BIGSERIAL PRIMARY KEY,
    database_uid BIGINT NOT NULL,
    scan_type TEXT NOT NULL CHECK (scan_type IN ('FULL', 'INCREMENTAL')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','RUNNING','DONE','FAILED')),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    columns_scanned INT DEFAULT 0,
    pii_detected INT DEFAULT 0,
    error_message TEXT
);
```

### 3.5 L4 — gRPC Service

**Proto**: `proto/v1/v1/pii_discovery_service.proto`

```protobuf
service PIIDiscoveryService {
  rpc StartScan(StartScanRequest) returns (PIIScanJob);
  rpc GetScanJob(GetScanJobRequest) returns (PIIScanJob);
  rpc ListScanResults(ListScanResultsRequest) returns (ListScanResultsResponse);
  rpc ConfirmClassification(ConfirmClassificationRequest) returns (PIIScanResult);
  rpc GetPIIInventorySummary(GetPIIInventorySummaryRequest) returns (PIIInventorySummary);
}
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Sample data leaks | Samples processed in-memory only, never persisted |
| Scanner overloads target DB | `LIMIT 100` + configurable sample size + read-only connection |
| Scan results exposure | Feature-gated (ENTERPRISE) + IAM permission `bb.piiDiscovery.*` |
| False positives | Human-in-the-loop: `is_confirmed=false` by default |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-013 (Data Classification) | PII results feed classification system |
| CR-PRV-002 (Anonymization) | Discovery results drive anonymization policy |
| CR-PRV-007 (Env Isolation) | PII inventory drives isolation enforcement |
| CR-ENT-012 (Data Masking) | Discovered PII auto-suggests masking rules |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Pattern registry + column name scanner | Sprint 1 |
| 2 | Sample data analysis + DB driver integration | Sprint 2 |
| 3 | SchemaSync hook + incremental scanning | Sprint 2 |
| 4 | PII Inventory dashboard (React) | Sprint 3 |
| 5 | Compliance mapping + reports | Sprint 3 |
| 6 | Multi-engine testing + optimization | Sprint 4 |

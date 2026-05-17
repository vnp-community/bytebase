# Change Request: PII Data Discovery & Inventory

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-001                                               |
| **Feature ID**     | SEC-16 (extends), UR-S06                                 |
| **Title**          | PII Data Discovery & Inventory                           |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-013, CR-PRV-002, CR-PRV-003                      |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **tự động phát hiện, phân loại và quản lý PII** trên toàn bộ database instances. Quét metadata (column name, data type) và sample data để phát hiện dữ liệu nhạy cảm, xây dựng **PII Inventory** toàn diện.

### 1.2 Bối cảnh từ PRD/URD
- **PRD SEC-16**: Data Classification chỉ hỗ trợ manual/heuristic — thiếu deep scanning
- **URD UR-S06**: Security Officer cần automated PII discovery
- **URD UR-S02**: Masking cần biết column nào chứa PII — discovery là prerequisite

### 1.3 Mục tiêu
- Automated PII scanning trên 22+ database engines
- Deep heuristic + NLP-based detection (column name + data sampling)
- PII Inventory dashboard với real-time coverage metrics
- Compliance mapping (GDPR Article 30, PDPA, HIPAA)
- Incremental scanning on schema sync

---

## 2. Yêu cầu chức năng

### FR-001: PII Scanner Engine
- **Detection Methods**: Column Name Match, Data Type Heuristic, Sample Analysis (max 100 rows), Comment/Annotation scan, Cross-reference patterns
- **PII Categories**: Email, Phone, National ID, Full Name, Address, DOB, Financial, Health, Credentials, IP/Device
- **Acceptance Criteria**:
  - AC-1: Scan chạy asynchronous, không block application
  - AC-2: Sample data KHÔNG lưu trữ sau scan
  - AC-3: Support 22+ database engines
  - AC-4: Confidence score (0-100%) cho mỗi detection

### FR-002: PII Inventory Dashboard
- Coverage Overview, PII Heatmap, Unscanned Report, Risk Score, Compliance Gaps
- **Acceptance Criteria**:
  - AC-1: Real-time dashboard, drill-down database → table → column
  - AC-2: Export inventory (CSV, JSON, PDF)
  - AC-3: Filter by PII category, classification level, engine

### FR-003: Incremental Scan on Schema Sync
- Trigger on: new table, new column, column rename, type change
- **Acceptance Criteria**:
  - AC-1: ≤ 30s cho typical schema change
  - AC-2: Skip confirmed classifications
  - AC-3: Alert on new PII detected

### FR-004: Compliance Mapping
- GDPR Article 30, PDPA, HIPAA PHI tracking, PCI-DSS cardholder data flow
- **Acceptance Criteria**: Per-regulation report template, gap analysis

---

## 3. Yêu cầu kỹ thuật

| Component                | File/Package                              | Thay đổi                       |
|--------------------------|-------------------------------------------|--------------------------------|
| PII Scanner Service      | `backend/component/privacy/scanner.go`    | Core scanning engine           |
| PII Inventory Store      | `backend/store/pii_inventory.go`          | CRUD for PII inventory         |
| Schema Sync Hook         | `backend/runner/schemasync/`              | Hook incremental scan          |
| PII Discovery API        | `backend/api/v1/pii_discovery_service.go` | gRPC service                   |
| Feature Gate             | `enterprise/feature.go`                   | `FeaturePIIDiscovery`          |

---

## 4. Rollout Plan

| Phase   | Mô tả                                  | Timeline       |
|---------|------------------------------------------|----------------|
| Phase 1 | Scanner engine + column name matching    | Sprint 1       |
| Phase 2 | Sample data analysis + dashboard         | Sprint 2       |
| Phase 3 | Incremental scan integration             | Sprint 3       |
| Phase 4 | Compliance mapping & reporting           | Sprint 3       |

# Change Request: Restrict Copying Data

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-005                                               |
| **Feature ID**     | SQL-15                                                   |
| **Title**          | Restrict Copying Data from SQL Editor                    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Ngăn chặn việc sao chép (copy) dữ liệu từ SQL Editor results, bảo vệ dữ liệu nhạy cảm khỏi bị exfiltrate qua clipboard. Tính năng này kết hợp với Data Masking (SEC-15) và Watermark (ADM-07) để tạo lớp bảo vệ dữ liệu toàn diện.

### 1.3 Mục tiêu
- Ngăn copy/paste dữ liệu từ SQL query results
- Disable right-click context menu cho data cells
- Disable keyboard shortcuts (Ctrl+C, Ctrl+A) trên result grid
- Policy-based: configurable per environment/project

---

## 2. Yêu cầu chức năng

### FR-001: Copy Restriction Policy
- **Mô tả**: Admin có thể configure copy restriction policy theo scope.
- **Policy Scopes**:
  - Workspace-level (global default)
  - Environment-level (e.g., restrict in Production only)
  - Project-level override
- **Policy Options**:
  - `ALLOW` — Cho phép copy (default for non-production)
  - `RESTRICT` — Block tất cả copy actions
  - `RESTRICT_WITH_MASKING` — Block copy nhưng cho phép copy masked data
- **Acceptance Criteria**:
  - AC-1: Policy configurable qua Settings UI
  - AC-2: Environment-level policy override workspace default
  - AC-3: Chỉ workspace admin có thể thay đổi policy

### FR-002: Frontend Copy Prevention
- **Mô tả**: Triển khai copy prevention trên SQL Editor result grid.
- **Prevention Mechanisms**:
  - Disable `Ctrl+C` / `Cmd+C` keyboard shortcut trên result grid
  - Disable right-click context menu (Copy, Select All)
  - Disable `Ctrl+A` / `Cmd+A` select all trên result grid
  - Disable drag-and-drop text selection
  - CSS `user-select: none` trên data cells
- **Acceptance Criteria**:
  - AC-1: Copy attempts bị block với visual feedback (toast notification)
  - AC-2: Column headers và metadata vẫn có thể copy
  - AC-3: SQL query text vẫn có thể copy (chỉ restrict result data)
  - AC-4: Copy prevention áp dụng cho cả Admin Mode

### FR-003: Audit Copy Attempts
- **Mô tả**: Log các attempted copy actions cho audit trail.
- **Acceptance Criteria**:
  - AC-1: Mỗi copy attempt tạo audit log entry (nếu Full Audit Log enabled)
  - AC-2: Log chứa: user, timestamp, database, estimated data size

### FR-004: Export Restriction Integration
- **Mô tả**: Copy restriction phải tương thích với Data Export (SQL-09).
- **Acceptance Criteria**:
  - AC-1: Khi copy restricted, Data Export cũng tuân theo policy
  - AC-2: Export yêu cầu explicit approval nếu restricted
  - AC-3: Export qua API vẫn subject to same restrictions

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                            | Thay đổi                                      |
|------------------------------|------------------------------------------|-------------------------------------------------|
| Policy Service               | `backend/api/v1/org_policy_service.go`   | Add `COPY_DATA` policy type                    |
| Feature Gate                 | `enterprise/feature.go`                 | Define `FeatureRestrictCopyData`               |
| SQL Service                  | `backend/api/v1/sql_service.go`          | Include copy policy in query response           |

### 3.2 Frontend Changes

| Component             | File                                            | Thay đổi                                    |
|-----------------------|-------------------------------------------------|----------------------------------------------|
| SQL Editor Results    | `frontend/src/components/SQLResultTable.vue`     | Copy prevention handlers                    |
| Policy Settings       | `frontend/src/views/PolicySettings.vue`          | Copy policy configuration UI                |
| Monaco Editor         | `frontend/src/components/MonacoEditor.vue`       | Ensure query text still copyable            |

### 3.3 Proto Changes

```protobuf
message CopyDataPolicy {
  CopyDataRestriction restriction = 1;
}

enum CopyDataRestriction {
  COPY_DATA_RESTRICTION_UNSPECIFIED = 0;
  ALLOW = 1;
  RESTRICT = 2;
  RESTRICT_WITH_MASKING = 3;
}
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Copy result data khi policy = RESTRICT                  | Copy blocked, toast notification      |
| TC-002     | Copy query text khi policy = RESTRICT                   | Copy allowed (query not restricted)   |
| TC-003     | Right-click on result cell                              | Context menu disabled                 |
| TC-004     | Ctrl+A on result grid                                   | Select all disabled                   |
| TC-005     | Policy = ALLOW                                          | Normal copy behavior                  |
| TC-006     | Non-ENTERPRISE plan                                     | Copy always allowed (feature gated)   |
| TC-007     | Environment override: dev=ALLOW, prod=RESTRICT          | Correct policy per environment        |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Policy backend + proto              | Sprint 1       |
| Phase 2 | Frontend copy prevention            | Sprint 1       |
| Phase 3 | Audit integration                   | Sprint 2       |
| Phase 4 | Export restriction integration       | Sprint 2       |

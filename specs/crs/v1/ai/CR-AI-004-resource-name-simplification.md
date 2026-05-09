# Change Request: Resource Name Parser Simplification

| Field              | Value                                                            |
|--------------------|------------------------------------------------------------------|
| **CR ID**          | CR-AI-004                                                        |
| **Issue IDs**      | AI-BLOCKER-005                                                   |
| **Title**          | Typed Resource Refs & Unified Parser for AIP Resource Names      |
| **Category**       | API Design (API)                                                 |
| **Priority**       | P2 — Medium                                                      |
| **Status**         | Draft                                                            |
| **Created**        | 2026-05-09                                                       |
| **Author**         | VNP AI Ops Team                                                  |
| **PRD Refs**       | DCM-01 (Issue/Plan/Rollout), ADM-08 (API Integration), §2 (API Layer) |

---

## 1. Tổng quan

### 1.1 Mô tả
Refactor `common/resource_name.go` (736 LOC, 50+ functions) thành typed struct-based parsers, giảm decision space cho AI agents từ 50+ function choices xuống ~15 typed parsers.

### 1.2 Bối cảnh
Bytebase API tuân thủ AIP (API Improvement Proposals) resource naming conventions (PRD §2 — API Layer). Resource names có cấu trúc phân cấp sâu:
```
projects/{project}/plans/{plan}/rollout/stages/{stage}/tasks/{task}/taskRuns/{taskRun}
```

Hiện tại có 7 variants cho cùng một resource hierarchy (`GetProjectIDPlanID*`), với return types không nhất quán (some return `string`, others `int64`, others `*string` nullable).

### 1.3 Mục tiêu
- Giảm parser function count từ 50+ xuống ~15
- Return type nhất quán qua typed structs
- AI agent chỉ cần biết struct name để chọn đúng parser
- Reference documentation cho resource name patterns

---

## 2. Yêu cầu chức năng

### FR-001: Typed Resource Reference Structs
- **Mô tả**: Định nghĩa typed structs cho mỗi resource hierarchy
- **Logic**:
  ```go
  // ProjectRef identifies a project resource.
  type ProjectRef struct {
      ProjectID string
  }

  // PlanRef identifies a plan within a project.
  type PlanRef struct {
      ProjectID string
      PlanUID   int64
  }

  // RolloutStageRef identifies a rollout stage.
  type RolloutStageRef struct {
      ProjectID string
      PlanUID   int64
      StageID   *string  // nil = wildcard "-"
  }

  // RolloutTaskRef identifies a task within a rollout.
  type RolloutTaskRef struct {
      ProjectID string
      PlanUID   int64
      StageID   string
      TaskUID   *int64   // nil = wildcard "-"
  }

  // TaskRunRef identifies a task run.
  type TaskRunRef struct {
      ProjectID string
      PlanUID   int64
      StageID   string
      TaskUID   int64
      TaskRunUID int64
  }

  // IssueRef identifies an issue within a project.
  type IssueRef struct {
      ProjectID string
      IssueUID  int64
  }

  // DatabaseRef identifies a database on an instance.
  type DatabaseRef struct {
      InstanceID   string
      DatabaseName string
  }
  ```
- **PRD Alignment**: DCM-01 (Issue/Plan/Rollout resources), ADM-08 (API Integration)
- **Acceptance Criteria**:
  - AC-1: Structs cover all resource hierarchies used in `api/v1/`
  - AC-2: Optional fields use `*type` (pointer for nil = wildcard)
  - AC-3: Struct names match AIP resource patterns

### FR-002: Unified Parse Functions
- **Mô tả**: 1 parse function per typed struct
- **Logic**:
  ```go
  func ParseProjectRef(name string) (*ProjectRef, error)
  func ParsePlanRef(name string) (*PlanRef, error)
  func ParseRolloutStageRef(name string) (*RolloutStageRef, error)
  func ParseRolloutTaskRef(name string) (*RolloutTaskRef, error)
  func ParseTaskRunRef(name string) (*TaskRunRef, error)
  func ParseIssueRef(name string) (*IssueRef, error)
  func ParseDatabaseRef(name string) (*DatabaseRef, error)
  ```
- **Acceptance Criteria**:
  - AC-1: Each function returns single typed struct
  - AC-2: Error messages include expected format
  - AC-3: Backward compatible — old functions delegate to new

### FR-003: Backward Compatibility Wrapper
- **Mô tả**: Giữ old functions nhưng delegate sang new typed parsers
- **Logic**:
  ```go
  // Deprecated: Use ParsePlanRef instead.
  func GetProjectIDPlanID(name string) (string, int64, error) {
      ref, err := ParsePlanRef(name)
      if err != nil { return "", 0, err }
      return ref.ProjectID, ref.PlanUID, nil
  }
  ```
- **Acceptance Criteria**:
  - AC-1: All 50+ old functions still compile and work
  - AC-2: Old functions marked `// Deprecated`
  - AC-3: No caller breakage

### FR-004: Resource Name Reference Documentation
- **Mô tả**: Tạo `RESOURCE_NAMES.md` cho AI reference
- **Content**: AIP pattern examples, struct-to-pattern mapping, parser selection guide
- **Acceptance Criteria**:
  - AC-1: Document covers all resource hierarchies
  - AC-2: Each pattern has parse + format example

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                        | Thay đổi                                        |
|------------------------|---------------------------------------------|---------------------------------------------------|
| Resource Refs          | `backend/common/resource_ref.go`            | NEW — typed struct definitions                    |
| Typed Parsers          | `backend/common/resource_parser.go`         | NEW — `Parse*Ref()` functions                     |
| Legacy Wrappers        | `backend/common/resource_name.go`           | Add `// Deprecated` + delegate to new parsers     |
| Reference Doc          | `backend/common/RESOURCE_NAMES.md`          | NEW — AI reference documentation                  |
| Tests                  | `backend/common/resource_ref_test.go`       | NEW — comprehensive parse/format tests            |

### 3.2 Không có Database Changes
### 3.3 Không có API Contract Changes

---

## 4. Phụ thuộc

| Dependency             | Mô tả                                                     |
|------------------------|------------------------------------------------------------|
| AIP Resource Naming    | Follow google.aip.dev/122 conventions                      |
| Existing callers       | Must not break — backward compatibility required           |

---

## 5. Test Cases

| Test ID | Mô tả                                                         | Expected Result                     |
|---------|----------------------------------------------------------------|-------------------------------------|
| TC-001  | `ParsePlanRef("projects/foo/plans/123")` returns correct ref  | ProjectID="foo", PlanUID=123        |
| TC-002  | `ParseRolloutTaskRef` with wildcard stage "-"                  | StageID=nil                         |
| TC-003  | Old `GetProjectIDPlanID` produces same result as `ParsePlanRef`| Identical output                    |
| TC-004  | Invalid resource name → descriptive error message              | Error includes expected format      |
| TC-005  | Round-trip: `Format*` → `Parse*` → same values                | Lossless conversion                 |
| TC-006  | All existing `resource_name_test.go` tests pass               | No regression                       |

---

## 6. Rollout Plan

| Phase   | Mô tả                                               | Timeline  |
|---------|-------------------------------------------------------|-----------|
| Phase 1 | Define typed structs + `Parse*Ref()` functions        | Sprint 1  |
| Phase 2 | Add backward-compatible wrappers with `// Deprecated` | Sprint 1  |
| Phase 3 | Create `RESOURCE_NAMES.md` reference doc              | Sprint 1  |
| Phase 4 | Migrate callers incrementally (optional)              | Sprint 2+ |

---

## 7. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                       |
|-----------------------------------------------|--------|---------------------------------------------------|
| Caller migration creates churn                | MEDIUM | Wrappers preserve old API — migration is optional |
| Struct proliferation                          | LOW    | ~10 structs total — manageable                    |
| Performance overhead from struct allocation   | LOW    | Stack-allocated structs — negligible              |

---

## 8. Success Metrics

| Metric                          | Before | Target  |
|---------------------------------|--------|---------|
| Parser function count           | 50+    | ~15     |
| AI decision space for parsers   | 50+    | ~15     |
| Functions with ambiguous naming | 7      | 0       |
| Resource name documentation     | None   | Complete |

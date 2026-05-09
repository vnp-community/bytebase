# Change Request: Error Handling Hardening & Observability

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-003                                              |
| **Weakness ID**    | WEAK-003                                                 |
| **Title**          | Error Handling Hardening — Eliminate Silent Failures      |
| **Category**       | Reliability / Observability                              |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01 (IAM), SEC-10 (Audit Log), DCM-12 (Changelog)    |

---

## 1. Tổng quan

### 1.1 Mô tả
Loại bỏ error swallowing patterns trên toàn codebase — đặc biệt trong IAM permission checks, migration executor changelog updates, và masking evaluator — thay thế bằng proper error propagation và structured error metrics.

### 1.2 Bối cảnh
**50+ `nolint` directives** trong service layer, nhiều `nolint:nilerr` (intentional error swallowing), và IAM manager silently denies permissions khi store errors xảy ra. Đây là P0 vì:
- IAM silent failures có thể **lock users out** không lý do
- Changelog failures gây **data inconsistency** — migration done nhưng không có audit trail
- Masking evaluator blanket `nolint` che giấu security-critical bugs

### 1.3 Mục tiêu
- Zero blanket `//nolint` — chỉ cho phép specific lint IDs
- IAM: return error thay vì silent deny khi store unavailable
- Migration executor: propagate changelog/sync errors thành task warnings
- Error metrics exported cho Prometheus alerting

---

## 2. Yêu cầu chức năng

### FR-001: IAM Error Propagation
- **Mô tả**: `iam.Manager.CheckPermission()` phải return error khi store operations fail thay vì silently returning false (deny).
- **Hiện tại**:
  ```go
  getPermissions := func(role string) map[permission.Permission]bool {
      perms, _ := m.GetPermissions(ctx, workspaceID, role)  // ERROR IGNORED
      return perms
  }
  ```
- **Sửa thành**:
  ```go
  getPermissions := func(role string) (map[permission.Permission]bool, error) {
      return m.GetPermissions(ctx, workspaceID, role)  // ERROR PROPAGATED
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Store error → CheckPermission returns `(false, err)` thay vì `(false, nil)`
  - AC-2: API layer trả về 503 (Service Unavailable) thay vì 403 (Forbidden) khi store down
  - AC-3: Error metric `bytebase_iam_store_errors_total` incremented
  - AC-4: Existing permission logic không bị regression (only error path changes)

### FR-002: Migration Executor Error Visibility
- **Mô tả**: Changelog và schema sync errors phải visible trong task run result thay vì chỉ log.
- **Hiện tại**:
  ```go
  if err := exec.store.UpdateChangelog(ctx, update); err != nil {
      slog.Error("failed to update changelog", log.BBError(err))
      // SILENTLY CONTINUES
  }
  ```
- **Sửa thành**:
  ```go
  if err := exec.store.UpdateChangelog(ctx, update); err != nil {
      slog.Error("failed to update changelog", log.BBError(err))
      result.Warnings = append(result.Warnings, fmt.Sprintf("changelog update failed: %v", err))
      metrics.ChangelogUpdateErrors.Inc()
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Changelog update failures visible trong TaskRunResult.Warnings
  - AC-2: Schema sync failures visible trong TaskRunResult.Warnings
  - AC-3: Migration vẫn marked DONE nếu SQL execution thành công
  - AC-4: Prometheus alert khi changelog error rate > 0.1%

### FR-003: Blanket Nolint Replacement
- **Mô tả**: Replace tất cả blanket `//nolint` với specific lint rule IDs.
- **Scope**:
  | File                        | Current                | Target                           |
  |-----------------------------|------------------------|----------------------------------|
  | `masking_evaluator.go:52`   | `// nolint`            | `//nolint:unused` hoặc remove    |
  | `masking_evaluator.go:63`   | `// nolint`            | `//nolint:unused` hoặc remove    |
  | `masking_evaluator.go:78`   | `// nolint`            | `//nolint:unused` hoặc remove    |
  | `masker.go:620,628`         | `//nolint`             | `//nolint:gosec` (if justified)  |
  | `mysql.go:389,391,471`      | `//nolint`             | `//nolint:errcheck` + comment    |
  | `tidb.go:248,250,323`       | `//nolint`             | `//nolint:errcheck` + comment    |
- **Acceptance Criteria**:
  - AC-1: Zero blanket `//nolint` (without specific rule) in codebase
  - AC-2: Each nolint has justification comment
  - AC-3: CI lint check enforces specific nolint rules

### FR-004: Structured Error Metrics
- **Mô tả**: Export error counters cho tất cả swallowed-error locations.
- **Metrics**:
  ```
  bytebase_iam_store_errors_total{operation="get_permissions"}
  bytebase_iam_store_errors_total{operation="get_group_members"}
  bytebase_changelog_update_errors_total{instance="..."}
  bytebase_schema_sync_errors_total{instance="..."}
  bytebase_nilerr_swallowed_total{service="sql_service", location="..."}
  ```
- **Acceptance Criteria**:
  - AC-1: Tất cả error counters visible trên /metrics endpoint
  - AC-2: Grafana dashboard template cho error monitoring
  - AC-3: Alert rule cho error rate > threshold

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                  | File                                         | Thay đổi                                     |
|----------------------------|----------------------------------------------|-----------------------------------------------|
| IAM Manager                | `backend/component/iam/manager.go`           | Error propagation cho getPermissions/getGroupMembers |
| IAM Check function         | `backend/component/iam/manager.go:107`       | Accept error-returning closures               |
| ACL Interceptor            | `backend/api/v1/acl.go`                      | Handle IAM errors as 503 not 403              |
| Migration Executor         | `backend/runner/taskrun/database_migrate_executor.go` | Add warnings to TaskRunResult       |
| TaskRunResult proto        | `proto/store/task_run.proto`                 | Add `repeated string warnings` field          |
| Masking Evaluator          | `backend/api/v1/masking_evaluator.go`        | Replace blanket nolint                        |
| Error Metrics              | `backend/metrics/error_metrics.go`           | New Prometheus counters                       |
| Lint CI Config             | `.golangci.yml`                              | Enforce `nolint` must have specific rule      |

### 3.2 Database Changes

```protobuf
// proto/store/task_run.proto — Add warnings field
message TaskRunResult {
  bool has_prior_backup = 1;
  repeated string warnings = 2;  // NEW: non-fatal issues during execution
}
```

Đây là additive protobuf change — backward compatible.

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | IAM check khi store returns error                            | CheckPermission returns (false, err)     |
| TC-002     | API request khi IAM store error                               | HTTP 503, not 403                        |
| TC-003     | IAM check khi store healthy                                   | Normal permission evaluation             |
| TC-004     | Migration execution — changelog update fails                  | Task DONE, result has warning            |
| TC-005     | Migration execution — schema sync fails                       | Task DONE, result has warning            |
| TC-006     | Migration execution — SQL execution fails                     | Task FAILED, changelog FAILED            |
| TC-007     | Prometheus /metrics contains error counters                   | All error metrics present                |
| TC-008     | golangci-lint rejects blanket `//nolint`                      | CI fails if blanket nolint found         |
| TC-009     | Masking evaluator functions with replaced nolint              | No functional regression                 |
| TC-010     | Concurrent IAM checks during store flap                       | Errors returned consistently, no panic   |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------|
| Phase 1 | IAM error propagation + ACL 503 handling           | Sprint 1     |
| Phase 2 | Migration executor warning propagation             | Sprint 1     |
| Phase 3 | Blanket nolint audit + replacement                 | Sprint 2     |
| Phase 4 | Error metrics + Prometheus counters                | Sprint 2     |
| Phase 5 | Lint CI enforcement + Grafana dashboard            | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                          |
|-----------------------------------------------|--------|------------------------------------------------------|
| IAM error propagation breaks existing callers | HIGH   | Gradual rollout, test all API endpoints              |
| 503 responses confuse users (was 403)         | MEDIUM | Clear error message: "Permission check unavailable"  |
| Warning field increases TaskRunResult size     | LOW    | Repeated string field, negligible size impact        |
| Nolint replacement reveals real lint issues    | MEDIUM | Fix issues before removing nolint directives         |

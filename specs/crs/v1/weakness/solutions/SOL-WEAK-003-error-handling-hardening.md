# Solution: Error Handling Hardening — CR-WEAK-003

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-003                                             |
| **CR Reference**   | CR-WEAK-003                                              |
| **Title**          | Eliminate Silent Failures — IAM, Migration, Masking      |
| **Affected Layers**| L3 (Security), L5 (Component), L6 (Runner), L4 (Service)|
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md:
- **L3 (Security)**: ACL interceptor (`acl.go:177-179`) calls `doIAMPermissionCheck` → `iamManager.CheckPermission`
- **L5 (Component)**: IAM Manager (`manager.go:37-44`) silently drops store errors in closure functions
- **L6 (Runner)**: `database_migrate_executor.go:269-282` swallows changelog/sync errors via `slog.Error` + continue

Per TDD.md §7.2: `CheckPermission` returns `(bool, error)` — callers in ACL interceptor DO handle errors (line 178-179), but the closures inside CheckPermission silently swallow them.

---

## 2. Root Cause Analysis

### 2.1 IAM Manager — Silent Error in Closures (manager.go:37-44)

```go
// CURRENT: errors silently dropped
getPermissions := func(role string) map[permission.Permission]bool {
    perms, _ := m.GetPermissions(ctx, workspaceID, role)  // ← _ drops error
    return perms
}
getGroupMembers := func(groupName string) map[string]bool {
    members, _ := m.store.GetGroupMembersSnapshot(ctx, workspaceID, groupName)  // ← _ drops error
    return members
}
```

The `check()` function (line 107) receives these closures and has no way to know if they failed. A store error returns `nil` permissions → binding skipped → permission denied (false negative).

### 2.2 Migration Executor — Fire-and-Forget (lines 267-282)

```go
// Schema sync failure — logged but execution continues
if err != nil {
    slog.Error("failed to sync database schema", log.BBError(err))
}
// Changelog update failure — logged but ignored
if err := exec.store.UpdateChangelog(ctx, update); err != nil {
    slog.Error("failed to update changelog", log.BBError(err))
}
```

The `TaskRunResult` (line 288-290) has no field to surface these warnings.

### 2.3 Blanket nolint (masking_evaluator.go:52,63,78)

```go
// nolint   ← No specific lint rule — suppresses ALL linters
```

---

## 3. Solution Design

### 3.1 IAM Error Propagation

**Modified file**: `backend/component/iam/manager.go`

```go
// NEW: check function signature accepts error-returning closures
func checkWithErrors(
    user *store.UserMessage,
    p permission.Permission,
    policy *storepb.IamPolicy,
    getPermissions func(role string) (map[permission.Permission]bool, error),
    getGroupMembers func(groupName string) (map[string]bool, error),
    skipAllUsers bool,
) (bool, error) {
    userName := formatUserNameByType(user)

    for _, binding := range policy.GetBindings() {
        if !utils.ValidateIAMBinding(binding) { continue }

        permissions, err := getPermissions(binding.GetRole())
        if err != nil {
            return false, errors.Wrapf(err, "failed to get permissions for role %q", binding.GetRole())
        }
        if permissions == nil || !permissions[p] { continue }

        for _, member := range binding.GetMembers() {
            if member == common.AllUsers && !skipAllUsers { return true, nil }
            if member == userName { return true, nil }
            if strings.HasPrefix(member, common.GroupPrefix) {
                members, err := getGroupMembers(member)
                if err != nil {
                    return false, errors.Wrapf(err, "failed to get group members for %q", member)
                }
                if members != nil && members[userName] { return true, nil }
            }
        }
    }
    return false, nil
}
```

**Modified `CheckPermission`**:

```go
func (m *Manager) CheckPermission(ctx context.Context, p permission.Permission, user *store.UserMessage, workspaceID string, projectIDs ...string) (bool, error) {
    getPermissions := func(role string) (map[permission.Permission]bool, error) {
        return m.GetPermissions(ctx, workspaceID, role)  // ERROR PROPAGATED
    }
    getGroupMembers := func(groupName string) (map[string]bool, error) {
        return m.store.GetGroupMembersSnapshot(ctx, workspaceID, groupName)  // ERROR PROPAGATED
    }

    policyMessage, err := m.store.GetWorkspaceIamPolicySnapshot(ctx, workspaceID)
    if err != nil { return false, err }

    ok, err := checkWithErrors(user, p, policyMessage.Policy, getPermissions, getGroupMembers, m.saas)
    if err != nil {
        iamStoreErrorsCounter.WithLabelValues("workspace_check").Inc()
        return false, err
    }
    if ok { return true, nil }

    // ...project-level check (same pattern)...
}
```

### 3.2 ACL Interceptor — 503 for Store Errors

**Modified file**: `backend/api/v1/acl.go` (line 177-179)

The existing code already handles errors correctly:
```go
ok, extra, err := doIAMPermissionCheck(ctx, in.iamManager, fullMethod, user, authContext)
if err != nil {
    return connect.NewError(connect.CodeInternal, ...)  // ← Already returns error
}
```

**Enhancement**: Differentiate store errors (503) from logic errors (500):

```go
if err != nil {
    // Check if this is a store/infrastructure error
    if isStoreError(err) {
        return connect.NewError(connect.CodeUnavailable,
            errors.Errorf("permission check temporarily unavailable for method %q: %v", fullMethod, err))
    }
    return connect.NewError(connect.CodeInternal, ...)
}
```

### 3.3 Migration Executor — Warning Propagation

**Proto change**: `proto/store/task_run.proto`

```protobuf
message TaskRunResult {
    bool has_prior_backup = 1;
    repeated string warnings = 2;  // NEW: non-fatal issues during execution
}
```

**Modified file**: `backend/runner/taskrun/database_migrate_executor.go`

```go
result := &storepb.TaskRunResult{
    HasPriorBackup: priorBackupDetail != nil && len(priorBackupDetail.Items) > 0,
}

// Schema sync — capture warning instead of silent continue
if err != nil {
    opts.LogDatabaseSyncEnd(err.Error())
    slog.Error("failed to sync database schema", log.BBError(err))
    result.Warnings = append(result.Warnings, fmt.Sprintf("schema sync failed: %v", err))
    schemaSyncErrorsCounter.Inc()
}

// Changelog update — capture warning
if err := exec.store.UpdateChangelog(ctx, update); err != nil {
    slog.Error("failed to update changelog", log.BBError(err))
    result.Warnings = append(result.Warnings, fmt.Sprintf("changelog update failed: %v", err))
    changelogUpdateErrorsCounter.Inc()
}

return result, nil
```

### 3.4 Blanket nolint Replacement

**masking_evaluator.go** — Replace blanket `// nolint` with specific rules:

```go
// Line 52: // nolint → //nolint:unused // evaluateMaskingLevelOfColumn is called via reflection
// Line 63: // nolint → //nolint:unused // maskingPolicyEvaluator is registered dynamically
// Line 78: // nolint → //nolint:unused // evaluateColumnMaskingWithPolicy called via interface
```

**Other locations**:
- `document_masking.go:693` — `//nolint:nilerr` already specific ✓ (keep, add comment)
- `user_service.go:766` — Change to `//nolint:nilerr // user lookup miss returns nil, not error`
- `sql_service.go:1857` — `//nolint:nilerr` → add justification comment

### 3.5 Error Metrics

**New file**: `backend/metrics/error_metrics.go`

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    IAMStoreErrorsCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytebase_iam_store_errors_total",
            Help: "IAM permission check failures due to store errors",
        },
        []string{"operation"},
    )
    ChangelogUpdateErrorsCounter = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_changelog_update_errors_total",
        Help: "Changelog update failures during migration execution",
    })
    SchemaSyncErrorsCounter = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_schema_sync_errors_total",
        Help: "Schema sync failures during migration execution",
    })
)

func init() {
    prometheus.MustRegister(IAMStoreErrorsCounter, ChangelogUpdateErrorsCounter, SchemaSyncErrorsCounter)
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/component/iam/manager.go` | L5 | MODIFY | Error-returning closures |
| `backend/api/v1/acl.go` | L3 | MODIFY | 503 for store errors |
| `backend/runner/taskrun/database_migrate_executor.go` | L6 | MODIFY | Warning propagation |
| `proto/store/task_run.proto` | — | MODIFY | Add `warnings` field |
| `backend/api/v1/masking_evaluator.go` | L4 | MODIFY | Replace blanket nolint |
| `backend/metrics/error_metrics.go` | L10 | NEW | Error counters |

## 5. Backward Compatibility

- `checkWithErrors` replaces `check` — old function removed
- Proto `warnings` field is additive — backward compatible
- ACL `CodeUnavailable` (503) vs `CodeInternal` (500) — client behavior may differ
- All nolint changes are comment-only — no functional change

## 6. Test Strategy

```go
func TestCheckPermission_StoreError(t *testing.T) {
    // Mock store returns error → CheckPermission returns (false, err)
}

func TestACL_StoreError_Returns503(t *testing.T) {
    // IAM returns error → ACL returns connect.CodeUnavailable
}

func TestMigrationExecutor_ChangelogWarning(t *testing.T) {
    // Mock UpdateChangelog fails → result.Warnings contains message
}
```

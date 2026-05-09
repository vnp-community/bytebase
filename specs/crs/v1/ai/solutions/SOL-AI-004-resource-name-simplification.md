# Solution: Resource Name Parser Simplification — CR-AI-004

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-AI-004                                               |
| **CR Reference**   | CR-AI-004                                                |
| **Title**          | Typed Resource Refs with Backward-Compatible Wrappers    |
| **Affected Layers**| L10 (Infrastructure/Common)                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §10 (L10 — Infrastructure): `backend/common/` contains CEL, resource names, error types, engine mapping. Resource name parsing is foundational for ALL upper layers (L3 ACL, L4 Services, L6 Runners).

Per TDD.md §7.1: Bytebase uses AIP-style resource names for two-level IAM. ACL interceptor (TDD.md §3.1, step 3) extracts project/resource from request messages using these parsers.

Per architecture.md §5 (L4): 30+ gRPC services use resource names like `projects/{project}/plans/{plan}/rollout/stages/{stage}/tasks/{task}/taskRuns/{taskRun}`.

**Key insight**: Resource name hierarchy maps directly to the DCM domain model (PRD DCM-01: Issue → Plan → Rollout → Stage → Task → TaskRun).

---

## 2. Solution Design

### 2.1 Typed Resource Reference Structs

Define structs that model the AIP resource hierarchy. Each struct maps to one level of the resource tree.

```go
// backend/common/resource_ref.go
package common

// ---- Project-scoped Resources ----

// ProjectRef identifies a project resource.
// Pattern: "projects/{project}"
type ProjectRef struct {
    ProjectID string  // workspace-unique project identifier
}

// PlanRef identifies a plan within a project.
// Pattern: "projects/{project}/plans/{planUID}"
type PlanRef struct {
    ProjectID string
    PlanUID   int64
}

// IssueRef identifies an issue within a project.
// Pattern: "projects/{project}/issues/{issueUID}"
type IssueRef struct {
    ProjectID string
    IssueUID  int64
}

// ReleaseRef identifies a release within a project.
// Pattern: "projects/{project}/releases/{releaseUID}"
type ReleaseRef struct {
    ProjectID string
    ReleaseUID int64
}

// ---- Rollout Hierarchy (per TDD.md §5.2 — Task Execution Pipeline) ----

// RolloutRef identifies a rollout within a project.
// Pattern: "projects/{project}/rollouts/{rolloutUID}"
type RolloutRef struct {
    ProjectID  string
    RolloutUID int64
}

// StageRef identifies a stage within a rollout.
// Pattern: "projects/{project}/rollouts/{rolloutUID}/stages/{stageID}"
type StageRef struct {
    ProjectID  string
    RolloutUID int64
    StageID    string   // can be "-" wildcard
}

// TaskRef identifies a task within a stage.
// Pattern: "projects/{project}/rollouts/{rolloutUID}/stages/{stageID}/tasks/{taskUID}"
type TaskRef struct {
    ProjectID  string
    RolloutUID int64
    StageID    string
    TaskUID    int64
}

// TaskRunRef identifies a task run within a task.
// Pattern: "projects/{project}/rollouts/{rolloutUID}/stages/{stageID}/tasks/{taskUID}/taskRuns/{taskRunUID}"
type TaskRunRef struct {
    ProjectID  string
    RolloutUID int64
    StageID    string
    TaskUID    int64
    TaskRunUID int64
}

// ---- Instance-scoped Resources ----

// InstanceRef identifies a database instance.
// Pattern: "instances/{instance}"
type InstanceRef struct {
    InstanceID string
}

// DatabaseRef identifies a database on an instance.
// Pattern: "instances/{instance}/databases/{database}"
type DatabaseRef struct {
    InstanceID   string
    DatabaseName string
}

// ---- Workspace-scoped Resources ----

// SettingRef identifies a workspace setting.
// Pattern: "settings/{setting}"
type SettingRef struct {
    SettingName string
}

// UserRef identifies a user.
// Pattern: "users/{userUID}" or "users/{email}"
type UserRef struct {
    UserUID string
}

// GroupRef identifies a group.
// Pattern: "groups/{groupEmail}"
type GroupRef struct {
    GroupEmail string
}
```

### 2.2 Unified Parse Functions

One parse function per typed struct. Each includes format description in error messages.

```go
// backend/common/resource_parser.go
package common

import (
    "fmt"
    "strconv"
    "strings"
)

// ParseProjectRef parses "projects/{project}" resource name.
func ParseProjectRef(name string) (*ProjectRef, error) {
    parts := strings.Split(name, "/")
    if len(parts) != 2 || parts[0] != "projects" {
        return nil, fmt.Errorf("invalid project resource name %q, expected format: projects/{project}", name)
    }
    return &ProjectRef{ProjectID: parts[1]}, nil
}

// ParsePlanRef parses "projects/{project}/plans/{planUID}" resource name.
func ParsePlanRef(name string) (*PlanRef, error) {
    parts := strings.Split(name, "/")
    if len(parts) != 4 || parts[0] != "projects" || parts[2] != "plans" {
        return nil, fmt.Errorf("invalid plan resource name %q, expected format: projects/{project}/plans/{planUID}", name)
    }
    uid, err := strconv.ParseInt(parts[3], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid plan UID %q in resource name %q: %w", parts[3], name, err)
    }
    return &PlanRef{ProjectID: parts[1], PlanUID: uid}, nil
}

// ParseIssueRef parses "projects/{project}/issues/{issueUID}" resource name.
func ParseIssueRef(name string) (*IssueRef, error) {
    parts := strings.Split(name, "/")
    if len(parts) != 4 || parts[0] != "projects" || parts[2] != "issues" {
        return nil, fmt.Errorf("invalid issue resource name %q, expected format: projects/{project}/issues/{issueUID}", name)
    }
    uid, err := strconv.ParseInt(parts[3], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid issue UID %q in resource name %q: %w", parts[3], name, err)
    }
    return &IssueRef{ProjectID: parts[1], IssueUID: uid}, nil
}

// ParseTaskRunRef parses deep rollout hierarchy.
// Pattern: "projects/{p}/rollouts/{r}/stages/{s}/tasks/{t}/taskRuns/{tr}"
func ParseTaskRunRef(name string) (*TaskRunRef, error) {
    parts := strings.Split(name, "/")
    if len(parts) != 10 {
        return nil, fmt.Errorf("invalid task run resource name %q, expected 10 segments", name)
    }
    if parts[0] != "projects" || parts[2] != "rollouts" || parts[4] != "stages" || parts[6] != "tasks" || parts[8] != "taskRuns" {
        return nil, fmt.Errorf("invalid task run resource name %q, expected format: projects/{p}/rollouts/{r}/stages/{s}/tasks/{t}/taskRuns/{tr}", name)
    }

    rolloutUID, err := strconv.ParseInt(parts[3], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid rollout UID in %q: %w", name, err)
    }
    taskUID, err := strconv.ParseInt(parts[7], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid task UID in %q: %w", name, err)
    }
    taskRunUID, err := strconv.ParseInt(parts[9], 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid task run UID in %q: %w", name, err)
    }

    return &TaskRunRef{
        ProjectID:  parts[1],
        RolloutUID: rolloutUID,
        StageID:    parts[5],
        TaskUID:    taskUID,
        TaskRunUID: taskRunUID,
    }, nil
}

// ParseDatabaseRef parses "instances/{instance}/databases/{database}".
func ParseDatabaseRef(name string) (*DatabaseRef, error) {
    parts := strings.Split(name, "/")
    if len(parts) != 4 || parts[0] != "instances" || parts[2] != "databases" {
        return nil, fmt.Errorf("invalid database resource name %q, expected format: instances/{instance}/databases/{database}", name)
    }
    return &DatabaseRef{InstanceID: parts[1], DatabaseName: parts[3]}, nil
}

// ... similar for StageRef, TaskRef, RolloutRef, InstanceRef, etc.
```

### 2.3 Format Functions (Round-trip Support)

```go
// backend/common/resource_parser.go (continued)

func (r *ProjectRef) String() string {
    return fmt.Sprintf("projects/%s", r.ProjectID)
}

func (r *PlanRef) String() string {
    return fmt.Sprintf("projects/%s/plans/%d", r.ProjectID, r.PlanUID)
}

func (r *IssueRef) String() string {
    return fmt.Sprintf("projects/%s/issues/%d", r.ProjectID, r.IssueUID)
}

func (r *TaskRunRef) String() string {
    return fmt.Sprintf("projects/%s/rollouts/%d/stages/%s/tasks/%d/taskRuns/%d",
        r.ProjectID, r.RolloutUID, r.StageID, r.TaskUID, r.TaskRunUID)
}

func (r *DatabaseRef) String() string {
    return fmt.Sprintf("instances/%s/databases/%s", r.InstanceID, r.DatabaseName)
}
```

### 2.4 Backward Compatibility Wrappers

```go
// backend/common/resource_name.go — MODIFIED (add deprecated wrappers)
// Existing 50+ functions remain but delegate to typed parsers.

// Deprecated: Use ParsePlanRef instead.
func GetProjectIDPlanID(name string) (string, int64, error) {
    ref, err := ParsePlanRef(name)
    if err != nil {
        return "", 0, err
    }
    return ref.ProjectID, ref.PlanUID, nil
}

// Deprecated: Use ParseIssueRef instead.
func GetProjectIDIssueUID(name string) (string, int64, error) {
    ref, err := ParseIssueRef(name)
    if err != nil {
        return "", 0, err
    }
    return ref.ProjectID, ref.IssueUID, nil
}

// Deprecated: Use ParseDatabaseRef instead.
func GetInstanceDatabaseID(name string) (string, string, error) {
    ref, err := ParseDatabaseRef(name)
    if err != nil {
        return "", "", err
    }
    return ref.InstanceID, ref.DatabaseName, nil
}

// ... similar wrappers for remaining 47+ functions
```

### 2.5 RESOURCE_NAMES.md Reference Documentation

```markdown
<!-- backend/common/RESOURCE_NAMES.md -->
# AIP Resource Name Patterns

## Quick Reference: Which Parser to Use

| Resource Pattern | Parse Function | Ref Struct |
|-----------------|----------------|------------|
| `projects/{p}` | `ParseProjectRef()` | `ProjectRef` |
| `projects/{p}/plans/{uid}` | `ParsePlanRef()` | `PlanRef` |
| `projects/{p}/issues/{uid}` | `ParseIssueRef()` | `IssueRef` |
| `projects/{p}/rollouts/{uid}` | `ParseRolloutRef()` | `RolloutRef` |
| `projects/{p}/rollouts/{r}/stages/{s}` | `ParseStageRef()` | `StageRef` |
| `projects/{p}/rollouts/{r}/stages/{s}/tasks/{t}` | `ParseTaskRef()` | `TaskRef` |
| `projects/{p}/rollouts/{r}/stages/{s}/tasks/{t}/taskRuns/{tr}` | `ParseTaskRunRef()` | `TaskRunRef` |
| `instances/{i}` | `ParseInstanceRef()` | `InstanceRef` |
| `instances/{i}/databases/{d}` | `ParseDatabaseRef()` | `DatabaseRef` |
| `settings/{name}` | `ParseSettingRef()` | `SettingRef` |
| `users/{uid}` | `ParseUserRef()` | `UserRef` |
| `groups/{email}` | `ParseGroupRef()` | `GroupRef` |

## DCM Workflow Hierarchy
(per TDD.md §5.2 — Task Execution Pipeline)

```
ProjectRef
├── PlanRef
│   └── RolloutRef
│       └── StageRef
│           └── TaskRef
│               └── TaskRunRef
└── IssueRef
```

## Usage Examples

```go
// Parse a plan resource name
ref, err := common.ParsePlanRef("projects/my-project/plans/12345")
// ref.ProjectID = "my-project"
// ref.PlanUID = 12345

// Format back to string
name := ref.String() // "projects/my-project/plans/12345"

// Parse a deep task run reference
taskRef, _ := common.ParseTaskRunRef("projects/p1/rollouts/100/stages/s1/tasks/200/taskRuns/300")
// taskRef.ProjectID = "p1", taskRef.RolloutUID = 100, ...
```
```

---

## 3. Execution Order

| Step | Files | Risk | Verification |
|------|-------|------|-------------|
| 1 | `resource_ref.go` — struct definitions | None | `go build` |
| 2 | `resource_parser.go` — parse + format | None | `go build` |
| 3 | `resource_ref_test.go` — comprehensive tests | None | `go test` |
| 4 | `resource_name.go` — deprecated wrappers | Low | Existing tests pass |
| 5 | `RESOURCE_NAMES.md` — documentation | None | Manual review |

---

## 4. File Change Manifest

| File | Action | LOC |
|------|--------|-----|
| `backend/common/resource_ref.go` | NEW | ~120 |
| `backend/common/resource_parser.go` | NEW | ~250 |
| `backend/common/resource_ref_test.go` | NEW | ~200 |
| `backend/common/resource_name.go` | MODIFY | Add `// Deprecated` + delegate |
| `backend/common/RESOURCE_NAMES.md` | NEW | Documentation |

---

## 5. Layer Compliance Check

Per architecture.md §13:
- L10 (Infrastructure/Common) → ✅ Resource refs stay in `common/` package
- L3 (Security/ACL) → L10: ✅ ACL uses common parsers — no change needed
- L4 (Service) → L10: ✅ Services call common parsers — new API is additive

---

## 6. Integration with CR-AI-005 (ACL Contract)

The typed resource refs directly improve ACL resource extraction (CR-AI-005 FR-004). ACL extractors can return `[]ResourceRef` instead of `[]string`, making the security contract type-safe:

```go
// Future: ACL extractor returns typed refs
type ResourceExtractorFunc func(proto.Message) []ResourceRef

// ResourceRef is a union type for any resource reference
type ResourceRef interface {
    ResourceType() string  // "project", "database", "instance"
    ProjectID() string     // project scope (may be empty)
}
```

---

## 7. Rollback Strategy

- New files: Delete `resource_ref.go`, `resource_parser.go` — no callers initially
- Modified file: `git revert` removes `// Deprecated` annotations
- Zero impact on existing behavior

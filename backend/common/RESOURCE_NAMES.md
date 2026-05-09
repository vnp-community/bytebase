# Resource Names — Quick Reference

> Internal reference for the Bytebase AIP resource naming system.

## Resource Pattern → Parse Function → Ref Struct

| Pattern | Parse Function | Ref Struct | Fields |
|---------|---------------|------------|--------|
| `projects/{project}` | `ParseProjectRef` | `ProjectRef` | `ProjectID` |
| `projects/{project}/plans/{planUID}` | `ParseResourcePlanRef` | `ResourcePlanRef` | `ProjectID`, `PlanUID` |
| `projects/{project}/issues/{issueUID}` | `ParseIssueResourceRef` | `IssueResourceRef` | `ProjectID`, `IssueUID` |
| `projects/{project}/releases/{releaseID}` | — | `ReleaseRef` | `ProjectID`, `ReleaseID` |
| `projects/{project}/plans/{planUID}/rollout` | — | `RolloutRef` | `ProjectID`, `PlanUID` |
| `projects/{project}/plans/{planUID}/rollout/stages/{stageID}` | — | `StageRef` | `ProjectID`, `PlanUID`, `StageID` |
| `projects/{project}/plans/{planUID}/rollout/stages/{stageID}/tasks/{taskUID}` | — | `TaskResourceRef` | `ProjectID`, `PlanUID`, `StageID`, `TaskUID` |
| `projects/{project}/plans/{planUID}/rollout/stages/{stageID}/tasks/{taskUID}/taskRuns/{taskRunUID}` | `ParseTaskRunResourceRef` | `TaskRunResourceRef` | `ProjectID`, `PlanUID`, `StageID`, `TaskUID`, `TaskRunUID` |
| `instances/{instanceID}` | `ParseInstanceRef` | `InstanceRef` | `InstanceID` |
| `instances/{instanceID}/databases/{databaseName}` | `ParseDatabaseResourceRef` | `DatabaseResourceRef` | `InstanceID`, `DatabaseName` |
| `settings/{settingName}` | `ParseSettingRef` | `SettingRef` | `SettingName` |
| `users/{email}` | `ParseUserResourceRef` | `UserResourceRef` | `Email` |

## DCM Workflow Hierarchy

```
projects/{project}
├── plans/{planUID}
│   ├── planCheckRuns/{planCheckRunUID}
│   └── rollout/
│       └── stages/{stageID}
│           └── tasks/{taskUID}
│               └── taskRuns/{taskRunUID}
├── issues/{issueUID}
│   └── issueComments/{commentID}
├── releases/{releaseID}
│   └── files/{fileName}
├── databaseGroups/{dbGroupID}
├── webhooks/{webhookID}
├── worksheets/{worksheetID}
└── accessGrants/{grantID}
```

## Usage Examples

### Parsing a resource name (new typed API)

```go
// Parse a plan name into a typed ref
ref, err := common.ParseResourcePlanRef("projects/my-project/plans/42")
if err != nil {
    return err
}
fmt.Println(ref.ProjectID)  // "my-project"
fmt.Println(ref.PlanUID)    // 42

// Round-trip: ref → string → ref
name := ref.String()  // "projects/my-project/plans/42"
ref2, _ := common.ParseResourcePlanRef(name)
// ref2 == ref ✅
```

### Parsing a complex task run name

```go
ref, err := common.ParseTaskRunResourceRef(
    "projects/prod/plans/100/rollout/stages/env-staging/tasks/200/taskRuns/300",
)
if err != nil {
    return err
}
// ref.ProjectID  = "prod"
// ref.PlanUID    = 100
// ref.StageID    = "env-staging"
// ref.TaskUID    = 200
// ref.TaskRunUID = 300
```

### Legacy API (deprecated, still works)

```go
// Old style — still works, delegates to Parse*Ref internally
projectID, err := common.GetProjectID("projects/my-project")
instanceID, dbName, err := common.GetInstanceDatabaseID("instances/pg-1/databases/mydb")
```

## Format Functions

| Function | Pattern | Example |
|----------|---------|---------|
| `FormatProject(id)` | `projects/{id}` | `projects/my-project` |
| `FormatDatabase(inst, db)` | `instances/{inst}/databases/{db}` | `instances/pg-1/databases/mydb` |
| `FormatInstance(id)` | `instances/{id}` | `instances/pg-1` |
| `FormatUser(email)` | `users/{email}` | `users/admin@example.com` |
| `FormatAccessGrant(proj, id)` | `projects/{proj}/accessGrants/{id}` | `projects/p1/accessGrants/ag1` |

## Implementation Files

| File | Purpose |
|------|---------|
| `backend/common/resource_ref.go` | Typed Ref structs + Parse functions |
| `backend/common/resource_ref_test.go` | Round-trip tests |
| `backend/common/resource_name.go` | Legacy Get* functions (deprecated wrappers) + Format functions |

# Solution: Event Bus Interface & ACL Contract Extraction — CR-AI-005

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-AI-005                                               |
| **CR Reference**   | CR-AI-005                                                |
| **Title**          | Typed EventBus Interface & Static ACL Resource Map       |
| **Affected Layers**| L3 (Security), L5 (Component/Bus), L6 (Runner)           |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

### 1.1 Bus Layer

Per architecture.md §6 (L5 Component): Bus is an in-process message bus using Go channels for runner coordination. It has 5 typed channels + 2 sync.Map for cancel functions.

Per TDD.md §5.1: Bus design decision — buffered Go channels instead of external MQ. Trade-off: messages lost on crash, but low-latency for monolith.

Per TDD.md §5.2: Task execution pipeline:
```
Plan Created → PlanCheckTickleChan → PlanCheck Scheduler
PlanCheck Done → ApprovalCheckChan → Approval Runner
Approved → RolloutCreationChan → Rollout Creator
Rollout Created → TaskRunTickleChan → TaskRun Scheduler
TaskRun Done → PlanCompletionCheckChan → Plan completion check
```

Per architecture.md §7 (L6 Runner): 8 runners consume bus channels. Each runner is a goroutine started by `Server.Run()`.

### 1.2 ACL Layer

Per architecture.md §4 (L3 Security): ACL interceptor (19.3KB) is step 3 in the interceptor chain.

Per TDD.md §3.1: ACL interceptor resolves resource from request → checks IAM permission via Manager.

Per TDD.md §7: Two-level permission model (Workspace + Project). ACL must extract project resource from protobuf messages to determine authorization scope.

---

## 2. Solution Design — Part A: EventBus Interface

### 2.1 Interface Definition

Extract a testable interface from the concrete `Bus` struct. The interface models the public contract without exposing channel implementation details.

```go
// backend/component/bus/interface.go
package bus

import (
    "context"
)

// EventBus defines the contract for internal event communication.
// Per architecture.md §6 (L5): Components provide shared business logic.
// Per TDD.md §5.1: Bus uses Go channels — this interface hides that detail.
//
// Consumers:
//   - L6 Runners (TaskRun, PlanCheck, Approval, etc.)
//   - L4 Services (plan_service, rollout_service trigger events)
type EventBus interface {
    // ---- Scheduler Triggers ----
    // TicklePlanCheck signals the PlanCheck scheduler to process pending checks.
    // Producers: PlanService (create/update plan), NotifyListener (PG NOTIFY)
    // Consumer:  plancheck.Scheduler
    TicklePlanCheck()

    // TickleTaskRun signals the TaskRun scheduler to process pending tasks.
    // Producers: RolloutService (approve), NotifyListener (PG NOTIFY)
    // Consumer:  taskrun.Scheduler
    TickleTaskRun()

    // ---- Typed Event Dispatchers ----
    // RequestApprovalCheck dispatches an issue for approval template matching.
    // Producer: PlanService (plan check completed successfully)
    // Consumer: approval.Runner
    RequestApprovalCheck(projectID string, issueUID int64)

    // RequestRolloutCreation dispatches a plan for rollout creation.
    // Producer: IssueService (issue approved)
    // Consumer: taskrun.RolloutCreator
    RequestRolloutCreation(projectID string, planUID int64)

    // RequestPlanCompletionCheck checks if all tasks in a plan are done.
    // Producer: taskrun.Scheduler (task completed)
    // Consumer: plan completion checker
    RequestPlanCompletionCheck(projectID string, planUID int64)

    // ---- Cancellation Registry ----
    // RegisterTaskRunCancel stores a cancel function for a running task run.
    RegisterTaskRunCancel(taskRunUID int64, cancel context.CancelFunc)
    // CancelTaskRun cancels a specific running task run. Returns true if found.
    CancelTaskRun(taskRunUID int64) bool
    // DeregisterTaskRunCancel removes the cancel function after completion.
    DeregisterTaskRunCancel(taskRunUID int64)

    // RegisterPlanCheckCancel stores a cancel function for a running plan check.
    RegisterPlanCheckCancel(planCheckRunUID int64, cancel context.CancelFunc)
    // CancelPlanCheck cancels a specific running plan check. Returns true if found.
    CancelPlanCheck(planCheckRunUID int64) bool
    // DeregisterPlanCheckCancel removes the cancel function after completion.
    DeregisterPlanCheckCancel(planCheckRunUID int64)

    // ---- Subscriber Channels (for runner loops) ----
    // PlanCheckChan returns the channel that PlanCheck scheduler listens on.
    PlanCheckChan() <-chan struct{}
    // TaskRunChan returns the channel that TaskRun scheduler listens on.
    TaskRunChan() <-chan struct{}
    // ApprovalChan returns the channel for approval check events.
    ApprovalChan() <-chan ApprovalEvent
    // RolloutCreationChan returns the channel for rollout creation events.
    RolloutCreationChan() <-chan RolloutEvent
    // PlanCompletionChan returns the channel for plan completion check events.
    PlanCompletionChan() <-chan PlanCompletionEvent
}
```

### 2.2 Typed Event Structs

Replace `chan int` (untyped) and raw struct channels with typed events carrying diagnostic context.

```go
// backend/component/bus/events.go
package bus

import "time"

// ApprovalEvent carries context for approval check requests.
type ApprovalEvent struct {
    ProjectID string
    IssueUID  int64
    EmittedAt time.Time
    Source    string  // e.g., "plan_service", "api_handler"
}

// RolloutEvent carries context for rollout creation requests.
type RolloutEvent struct {
    ProjectID string
    PlanUID   int64
    EmittedAt time.Time
    Source    string
}

// PlanCompletionEvent carries context for plan completion checks.
type PlanCompletionEvent struct {
    ProjectID string
    PlanUID   int64
    EmittedAt time.Time
    Source    string
}
```

### 2.3 Concrete Implementation

```go
// backend/component/bus/bus.go — REFACTORED
package bus

import (
    "context"
    "sync"
    "time"
)

// Compile-time verification
var _ EventBus = (*Bus)(nil)

// Bus implements EventBus using buffered Go channels.
// Per TDD.md §5.1: messages are lost on crash — acceptable trade-off for monolith.
type Bus struct {
    planCheckCh     chan struct{}          // buffer: 1000
    taskRunCh       chan struct{}          // buffer: 1000
    approvalCh      chan ApprovalEvent     // buffer: 1000
    rolloutCh       chan RolloutEvent      // buffer: 100
    planCompleteCh  chan PlanCompletionEvent // buffer: 1000

    taskRunCancelFuncs      sync.Map  // int64 → context.CancelFunc
    planCheckCancelFuncs    sync.Map  // int64 → context.CancelFunc
}

// New creates a new Bus with default buffer sizes.
func New() *Bus {
    return &Bus{
        planCheckCh:    make(chan struct{}, 1000),
        taskRunCh:      make(chan struct{}, 1000),
        approvalCh:     make(chan ApprovalEvent, 1000),
        rolloutCh:      make(chan RolloutEvent, 100),
        planCompleteCh: make(chan PlanCompletionEvent, 1000),
    }
}

// ---- Scheduler Triggers ----

func (b *Bus) TicklePlanCheck() {
    select {
    case b.planCheckCh <- struct{}{}:
    default:
        // Channel full — scheduler will process on next cycle
    }
}

func (b *Bus) TickleTaskRun() {
    select {
    case b.taskRunCh <- struct{}{}:
    default:
    }
}

// ---- Typed Event Dispatchers ----

func (b *Bus) RequestApprovalCheck(projectID string, issueUID int64) {
    select {
    case b.approvalCh <- ApprovalEvent{
        ProjectID: projectID,
        IssueUID:  issueUID,
        EmittedAt: time.Now(),
        Source:    "bus",
    }:
    default:
    }
}

func (b *Bus) RequestRolloutCreation(projectID string, planUID int64) {
    select {
    case b.rolloutCh <- RolloutEvent{
        ProjectID: projectID,
        PlanUID:   planUID,
        EmittedAt: time.Now(),
        Source:    "bus",
    }:
    default:
    }
}

func (b *Bus) RequestPlanCompletionCheck(projectID string, planUID int64) {
    select {
    case b.planCompleteCh <- PlanCompletionEvent{
        ProjectID: projectID,
        PlanUID:   planUID,
        EmittedAt: time.Now(),
        Source:    "bus",
    }:
    default:
    }
}

// ---- Cancellation Registry ----

func (b *Bus) RegisterTaskRunCancel(taskRunUID int64, cancel context.CancelFunc) {
    b.taskRunCancelFuncs.Store(taskRunUID, cancel)
}

func (b *Bus) CancelTaskRun(taskRunUID int64) bool {
    if v, ok := b.taskRunCancelFuncs.Load(taskRunUID); ok {
        v.(context.CancelFunc)()
        return true
    }
    return false
}

func (b *Bus) DeregisterTaskRunCancel(taskRunUID int64) {
    b.taskRunCancelFuncs.Delete(taskRunUID)
}

func (b *Bus) RegisterPlanCheckCancel(planCheckRunUID int64, cancel context.CancelFunc) {
    b.planCheckCancelFuncs.Store(planCheckRunUID, cancel)
}

func (b *Bus) CancelPlanCheck(planCheckRunUID int64) bool {
    if v, ok := b.planCheckCancelFuncs.Load(planCheckRunUID); ok {
        v.(context.CancelFunc)()
        return true
    }
    return false
}

func (b *Bus) DeregisterPlanCheckCancel(planCheckRunUID int64) {
    b.planCheckCancelFuncs.Delete(planCheckRunUID)
}

// ---- Subscriber Channels ----

func (b *Bus) PlanCheckChan() <-chan struct{}             { return b.planCheckCh }
func (b *Bus) TaskRunChan() <-chan struct{}               { return b.taskRunCh }
func (b *Bus) ApprovalChan() <-chan ApprovalEvent         { return b.approvalCh }
func (b *Bus) RolloutCreationChan() <-chan RolloutEvent   { return b.rolloutCh }
func (b *Bus) PlanCompletionChan() <-chan PlanCompletionEvent { return b.planCompleteCh }
```

### 2.4 Runner Migration Pattern

Per architecture.md §7 (L6): Runners receive `*Bus` directly. After migration, they receive `EventBus` interface.

```go
// backend/runner/taskrun/scheduler.go — BEFORE
type Scheduler struct {
    bus *bus.Bus
    // ...
}

func NewScheduler(bus *bus.Bus, /*...*/) *Scheduler {
    return &Scheduler{bus: bus}
}

func (s *Scheduler) Run(ctx context.Context) {
    for {
        select {
        case <-s.bus.TaskRunTickleChan:  // ← direct channel access
            s.processTaskRuns(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

```go
// backend/runner/taskrun/scheduler.go — AFTER
type Scheduler struct {
    bus bus.EventBus  // ← interface
    // ...
}

func NewScheduler(bus bus.EventBus, /*...*/) *Scheduler {
    return &Scheduler{bus: bus}
}

func (s *Scheduler) Run(ctx context.Context) {
    for {
        select {
        case <-s.bus.TaskRunChan():  // ← typed method
            s.processTaskRuns(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

### 2.5 EVENT_FLOWS.md

```markdown
<!-- backend/component/bus/EVENT_FLOWS.md -->
# Event Flow Documentation

## DCM Workflow Pipeline
(Per TDD.md §5.2 — Task Execution Pipeline)

```
┌─────────────┐     TicklePlanCheck()     ┌──────────────────┐
│ PlanService  │──────────────────────────▶│ PlanCheck Runner │
│ (L4)         │                          │ (L6)             │
└─────────────┘                          └────────┬─────────┘
                                                   │
                                                   │ RequestApprovalCheck()
                                                   ▼
                                          ┌──────────────────┐
                                          │ Approval Runner  │
                                          │ (L6)             │
                                          └────────┬─────────┘
                                                   │
                                    RequestRolloutCreation()
                                                   ▼
                                          ┌──────────────────┐
                                          │ Rollout Creator   │
                                          │ (L6/taskrun)     │
                                          └────────┬─────────┘
                                                   │
                                       TickleTaskRun()
                                                   ▼
                                          ┌──────────────────┐
                                          │ TaskRun Scheduler │
                                          │ (L6)             │
                                          └────────┬─────────┘
                                                   │
                                RequestPlanCompletionCheck()
                                                   ▼
                                          ┌──────────────────┐
                                          │ Completion Check │
                                          └──────────────────┘
```

## Channel Buffer Sizes

| Channel | Buffer | Rationale |
|---------|--------|-----------|
| PlanCheckCh | 1000 | High throughput during bulk plan updates |
| TaskRunCh | 1000 | High throughput during bulk rollouts |
| ApprovalCh | 1000 | Batch approval scenarios |
| RolloutCh | 100 | Lower frequency — rollout creation |
| PlanCompleteCh | 1000 | Completion checks after each task |

## PG NOTIFY Integration
(Per TDD.md §5.4)

```
PG NOTIFY 'bytebase:plan_check' → NotifyListener → bus.TicklePlanCheck()
PG NOTIFY 'bytebase:task_run'   → NotifyListener → bus.TickleTaskRun()
```
```

---

## 3. Solution Design — Part B: ACL Static Resource Map

### 3.1 Static Extractor Map

Replace reflection-based field probing in `acl.go` with an explicit, auditable map.

```go
// backend/api/v1/acl_extractors.go
package v1

import (
    "google.golang.org/protobuf/proto"
    v1pb "github.com/bytebase/bytebase/proto/generated-go/v1"
)

// ResourceExtractorFunc extracts resource names from a protobuf request message.
// Returns the resource names that should be checked for IAM permissions.
type ResourceExtractorFunc func(msg proto.Message) ([]string, error)

// aclResourceExtractors is the static, auditable map of RPC method → resource extractor.
// Per architecture.md §4 (L3): ACL interceptor checks IAM permissions.
// Per TDD.md §7.1: Two-level permission model (workspace + project).
//
// SECURITY: Every new RPC method MUST have an entry here.
// Missing entries will return CodeInternal error (fail-closed).
var aclResourceExtractors = map[string]ResourceExtractorFunc{
    // ---- Auth Service (no resource check needed — public endpoints) ----
    "/bytebase.v1.AuthService/Login":                extractNone,
    "/bytebase.v1.AuthService/Logout":               extractNone,
    "/bytebase.v1.AuthService/GetUser":              extractFromName,
    "/bytebase.v1.AuthService/CreateUser":           extractNone,

    // ---- Database Service ----
    "/bytebase.v1.DatabaseService/GetDatabase":      extractFromName,
    "/bytebase.v1.DatabaseService/ListDatabases":    extractFromParent,
    "/bytebase.v1.DatabaseService/UpdateDatabase":   extractFromDatabaseUpdate,
    "/bytebase.v1.DatabaseService/BatchUpdateDatabases": extractFromBatchParent,
    "/bytebase.v1.DatabaseService/GetDatabaseSchema": extractFromName,
    "/bytebase.v1.DatabaseService/DiffSchema":       extractFromName,

    // ---- Plan Service ----
    "/bytebase.v1.PlanService/CreatePlan":           extractFromParent,
    "/bytebase.v1.PlanService/GetPlan":              extractFromName,
    "/bytebase.v1.PlanService/ListPlans":            extractFromParent,
    "/bytebase.v1.PlanService/UpdatePlan":           extractFromPlanUpdate,

    // ---- Rollout Service ----
    "/bytebase.v1.RolloutService/GetRollout":        extractFromName,
    "/bytebase.v1.RolloutService/CreateRollout":     extractFromParent,
    "/bytebase.v1.RolloutService/ListTaskRuns":      extractFromParent,
    "/bytebase.v1.RolloutService/RunTasks":          extractFromParent,
    "/bytebase.v1.RolloutService/CancelTaskRun":     extractFromName,

    // ---- Issue Service ----
    "/bytebase.v1.IssueService/GetIssue":            extractFromName,
    "/bytebase.v1.IssueService/CreateIssue":         extractFromParent,
    "/bytebase.v1.IssueService/ListIssues":          extractFromParent,
    "/bytebase.v1.IssueService/UpdateIssue":         extractFromIssueUpdate,
    "/bytebase.v1.IssueService/BatchUpdateIssuesStatus": extractFromBatchIssues,
    "/bytebase.v1.IssueService/ApproveIssue":        extractFromName,

    // ---- Project Service ----
    "/bytebase.v1.ProjectService/GetProject":        extractFromName,
    "/bytebase.v1.ProjectService/ListProjects":      extractNone,
    "/bytebase.v1.ProjectService/CreateProject":     extractNone,
    "/bytebase.v1.ProjectService/UpdateProject":     extractFromProjectUpdate,

    // ---- SQL Service ----
    "/bytebase.v1.SQLService/Query":                 extractFromSQLQuery,
    "/bytebase.v1.SQLService/AdminExecute":          extractNone, // streaming — checked inline
    "/bytebase.v1.SQLService/Check":                 extractFromSQLCheck,
    "/bytebase.v1.SQLService/Export":                extractFromSQLExport,

    // ---- Instance Service ----
    "/bytebase.v1.InstanceService/GetInstance":       extractFromName,
    "/bytebase.v1.InstanceService/ListInstances":     extractNone,
    "/bytebase.v1.InstanceService/CreateInstance":     extractNone,

    // ---- Setting/Workspace (workspace-level only) ----
    "/bytebase.v1.SettingService/GetSetting":         extractNone,
    "/bytebase.v1.SettingService/UpdateSetting":      extractNone,
    "/bytebase.v1.WorkspaceService/GetWorkspace":     extractNone,

    // ---- Subscription (workspace-level) ----
    "/bytebase.v1.SubscriptionService/GetSubscription": extractNone,
    "/bytebase.v1.SubscriptionService/UpdateSubscription": extractNone,

    // ... complete for all 30+ services
}

// ---- Helper Extractors ----

func extractNone(msg proto.Message) ([]string, error) {
    return nil, nil
}

func extractFromName(msg proto.Message) ([]string, error) {
    // Use proto reflection to get "name" field — but this is the ONLY
    // reflection point, and it's explicit per-method.
    return extractField(msg, "name")
}

func extractFromParent(msg proto.Message) ([]string, error) {
    return extractField(msg, "parent")
}

func extractFromDatabaseUpdate(msg proto.Message) ([]string, error) {
    req, ok := msg.(*v1pb.UpdateDatabaseRequest)
    if !ok {
        return nil, fmt.Errorf("expected UpdateDatabaseRequest, got %T", msg)
    }
    resources := []string{req.GetDatabase().GetName()}
    // Per TDD.md §7.1: if project field is in update mask, check both projects
    for _, path := range req.GetUpdateMask().GetPaths() {
        if path == "project" {
            resources = append(resources, req.GetDatabase().GetProject())
        }
    }
    return resources, nil
}

func extractFromBatchIssues(msg proto.Message) ([]string, error) {
    // Explicit handler — replaces the HACK comment in original code
    req, ok := msg.(*v1pb.BatchUpdateIssuesStatusRequest)
    if !ok {
        return nil, fmt.Errorf("expected BatchUpdateIssuesStatusRequest, got %T", msg)
    }
    return []string{req.GetParent()}, nil
}

func extractField(msg proto.Message, fieldName string) ([]string, error) {
    // Controlled reflection for a single known field
    md := msg.ProtoReflect().Descriptor()
    fd := md.Fields().ByName(protoreflect.Name(fieldName))
    if fd == nil {
        return nil, nil
    }
    val := msg.ProtoReflect().Get(fd).String()
    if val == "" {
        return nil, nil
    }
    return []string{val}, nil
}
```

### 3.2 ACL Interceptor Integration

```go
// backend/api/v1/acl.go — MODIFIED

func (in *ACLInterceptor) getResourcesFromRequest(
    ctx context.Context,
    fullMethod string,
    req proto.Message,
) ([]string, error) {
    // Static lookup — fail-closed for unknown methods
    extractor, ok := aclResourceExtractors[fullMethod]
    if !ok {
        // SECURITY: Unknown method defaults to error, not skip
        return nil, status.Errorf(codes.Internal,
            "ACL: no resource extractor registered for method %s. "+
            "Add an entry to aclResourceExtractors in acl_extractors.go", fullMethod)
    }
    return extractor(req)
}
```

### 3.3 CI Sync Check

Verify the extractor map covers all registered gRPC methods:

```go
// backend/api/v1/acl_extractors_test.go
func TestACLExtractors_CoverAllMethods(t *testing.T) {
    // Get all registered ConnectRPC service methods
    registeredMethods := getRegisteredMethods() // from grpc_routes.go

    for _, method := range registeredMethods {
        if _, ok := aclResourceExtractors[method]; !ok {
            t.Errorf("method %s has no ACL resource extractor. "+
                "Add entry to aclResourceExtractors in acl_extractors.go", method)
        }
    }
}
```

### 3.4 ACL_CONTRACT.md

```markdown
<!-- backend/api/v1/ACL_CONTRACT.md -->
# ACL Resource Extraction Contract

## Security Model
Per TDD.md §7: Two-level permission model.
- Workspace-level: checked for all requests
- Project-level: checked when resource extraction returns project-scoped names

## Extraction Patterns

| Pattern | Field | Used By |
|---------|-------|---------|
| `extractNone` | — | Public endpoints, workspace-level only |
| `extractFromName` | `name` | Get/Delete single resource |
| `extractFromParent` | `parent` | List/Create under parent |
| `extractFrom*Update` | Custom | Update with field masks |
| `extractFromBatch*` | Custom | Batch operations |

## Adding New RPC Methods

1. Add method to proto definition
2. Add extractor entry to `aclResourceExtractors` map
3. Run `go test ./backend/api/v1/... -run TestACLExtractors_CoverAllMethods`
4. If test fails, the new method is missing from the map
```

---

## 4. Execution Order

| Step | Part | Files | Risk | Verification |
|------|------|-------|------|-------------|
| 1 | A | `bus/interface.go` — EventBus interface | None | `go build` |
| 2 | A | `bus/events.go` — typed event structs | None | `go build` |
| 3 | A | `bus/bus.go` — implement EventBus on Bus | Medium | `go build` + tests |
| 4 | A | `bus/EVENT_FLOWS.md` — documentation | None | Manual review |
| 5 | A | Runners — migrate to EventBus interface | Medium | Integration tests |
| 6 | B | `acl_extractors.go` — static map | High (security) | ACL tests |
| 7 | B | `acl.go` — switch to static map lookup | High (security) | Full ACL tests |
| 8 | B | `acl_extractors_test.go` — coverage check | None | Tests pass |
| 9 | B | `ACL_CONTRACT.md` — documentation | None | Manual review |

---

## 5. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/component/bus/interface.go` | NEW | EventBus interface |
| `backend/component/bus/events.go` | NEW | Typed event structs |
| `backend/component/bus/bus.go` | MODIFY | Implement EventBus, replace chan int |
| `backend/component/bus/EVENT_FLOWS.md` | NEW | Event flow documentation |
| `backend/api/v1/acl_extractors.go` | NEW | Static resource extractor map |
| `backend/api/v1/acl.go` | MODIFY | Use static map instead of reflection |
| `backend/api/v1/acl_extractors_test.go` | NEW | Method coverage verification |
| `backend/api/v1/ACL_CONTRACT.md` | NEW | Security contract documentation |
| `backend/runner/taskrun/scheduler.go` | MODIFY | Use EventBus interface |
| `backend/runner/plancheck/scheduler.go` | MODIFY | Use EventBus interface |
| `backend/runner/approval/runner.go` | MODIFY | Use EventBus interface |

---

## 6. Layer Compliance Check

Per architecture.md §13 (Dependency Matrix):
- L6 (Runner) → L5 (Bus): ✅ Runners depend on bus — through interface now
- L4 (Service) → L5 (Bus): ✅ Services trigger bus events — via EventBus interface
- L3 (Security/ACL) → L5 (IAM): ✅ ACL interceptor uses static map — no L5 change
- L3 → L8 (Store): ✅ ACL still reads IAM policies from store

**Cross-layer direction preserved** — no upward dependencies introduced.

---

## 7. Security Review Checklist

Per TDD.md §11 (Security Architecture):

- [ ] Every registered gRPC method has an ACL extractor entry
- [ ] Missing methods fail with `CodeInternal` (not `CodePermissionDenied`)
- [ ] `UpdateDatabase` with project change checks both source and target projects
- [ ] `BatchUpdateIssuesStatus` has explicit extractor (no HACK)
- [ ] CI test verifies extractor map completeness
- [ ] `AdminExecute` streaming has inline ACL check (not extractor-based)

---

## 8. Rollback Strategy

**Part A (Bus)**: 
- Revert to direct channel access — `git revert`
- EventBus interface is additive — removing it requires reverting runner constructors

**Part B (ACL)**:
- Revert to reflection-based probing — `git revert`
- CRITICAL: ACL changes must be tested thoroughly before merge
- Recommendation: Feature branch with full integration test suite

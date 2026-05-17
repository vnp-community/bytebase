# TASK-101: Embed NATS Server + NATSBus

| Field | Value |
|-------|-------|
| Task ID | TASK-101 |
| Phase | 1 — NATS Bus + Transport |
| Estimated | 1 day |
| Dependencies | TASK-000 |
| Status | ✅ DONE |

## Objective

Tạo embedded NATS server + NATSBus (implement `EventBus` interface). Đây là **thay thế duy nhất** cho Go channels bus — tất cả runners và services tiếp tục dùng `EventBus` interface không đổi.

## Files to Create

### 1. `backend/component/bus/nats_bus.go`

```go
package bus

import (
    "context"
    "encoding/json"
    "sync"
    "time"

    natsserver "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
)

// Compile-time check
var _ EventBus = (*NATSBus)(nil)

const (
    SubjectPlanCheck       = "bytebase.plancheck.tickle"
    SubjectTaskRun         = "bytebase.taskrun.tickle"
    SubjectApprovalCheck   = "bytebase.approval.check"
    SubjectRolloutCreation = "bytebase.rollout.create"
    SubjectPlanCompletion  = "bytebase.plan.completion"
)

type NATSBus struct {
    ns *natsserver.Server
    nc *nats.Conn

    planCheckCh    chan int
    taskRunCh      chan int
    approvalCh     chan IssueRef
    rolloutCh      chan PlanRef
    planCompleteCh chan PlanRef

    runningTaskRunsCancelFunc      sync.Map
    runningPlanCheckRunsCancelFunc sync.Map
}

func NewNATSBus() (*NATSBus, error) {
    // Start embedded NATS server
    opts := &natsserver.Options{
        Host:           "127.0.0.1",
        Port:           -1, // Random port
        NoLog:          true,
        NoSigs:         true,
        MaxControlLine: 4096,
    }
    ns, err := natsserver.NewServer(opts)
    if err != nil {
        return nil, err
    }
    go ns.Start()
    if !ns.ReadyForConnections(5 * time.Second) {
        return nil, fmt.Errorf("NATS server failed to start")
    }

    nc, err := nats.Connect(ns.ClientURL())
    if err != nil {
        return nil, err
    }

    b := &NATSBus{
        ns:             ns,
        nc:             nc,
        planCheckCh:    make(chan int, 1000),
        taskRunCh:      make(chan int, 1000),
        approvalCh:     make(chan IssueRef, 1000),
        rolloutCh:      make(chan PlanRef, 100),
        planCompleteCh: make(chan PlanRef, 1000),
    }

    // Subscribe NATS → Go channels (bridge for existing runners)
    nc.Subscribe(SubjectPlanCheck, func(msg *nats.Msg) {
        select { case b.planCheckCh <- 0: default: }
    })
    nc.Subscribe(SubjectTaskRun, func(msg *nats.Msg) {
        select { case b.taskRunCh <- 0: default: }
    })
    nc.Subscribe(SubjectApprovalCheck, func(msg *nats.Msg) {
        var ref IssueRef
        json.Unmarshal(msg.Data, &ref)
        select { case b.approvalCh <- ref: default: }
    })
    nc.Subscribe(SubjectRolloutCreation, func(msg *nats.Msg) {
        var ref PlanRef
        json.Unmarshal(msg.Data, &ref)
        select { case b.rolloutCh <- ref: default: }
    })
    nc.Subscribe(SubjectPlanCompletion, func(msg *nats.Msg) {
        var ref PlanRef
        json.Unmarshal(msg.Data, &ref)
        select { case b.planCompleteCh <- ref: default: }
    })

    return b, nil
}

// --- Producer methods (publish to NATS) ---

func (b *NATSBus) TicklePlanCheck() {
    b.nc.Publish(SubjectPlanCheck, nil)
}

func (b *NATSBus) TickleTaskRun() {
    b.nc.Publish(SubjectTaskRun, nil)
}

func (b *NATSBus) RequestApprovalCheck(ref IssueRef) {
    data, _ := json.Marshal(ref)
    b.nc.Publish(SubjectApprovalCheck, data)
}

func (b *NATSBus) RequestRolloutCreation(ref PlanRef) {
    data, _ := json.Marshal(ref)
    b.nc.Publish(SubjectRolloutCreation, data)
}

func (b *NATSBus) RequestPlanCompletionCheck(ref PlanRef) {
    data, _ := json.Marshal(ref)
    b.nc.Publish(SubjectPlanCompletion, data)
}

// --- Cancel function registry (same in-memory as before) ---

func (b *NATSBus) RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc) {
    b.runningTaskRunsCancelFunc.Store(ref, cancel)
}
func (b *NATSBus) CancelTaskRun(ref TaskRunRef) bool {
    v, ok := b.runningTaskRunsCancelFunc.LoadAndDelete(ref)
    if !ok { return false }
    if cancel, ok := v.(context.CancelFunc); ok { cancel(); return true }
    return false
}
func (b *NATSBus) DeregisterTaskRunCancel(ref TaskRunRef) {
    b.runningTaskRunsCancelFunc.Delete(ref)
}
func (b *NATSBus) RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc) {
    b.runningPlanCheckRunsCancelFunc.Store(ref, cancel)
}
func (b *NATSBus) CancelPlanCheck(ref PlanCheckRunRef) bool {
    v, ok := b.runningPlanCheckRunsCancelFunc.LoadAndDelete(ref)
    if !ok { return false }
    if cancel, ok := v.(context.CancelFunc); ok { cancel(); return true }
    return false
}
func (b *NATSBus) DeregisterPlanCheckCancel(ref PlanCheckRunRef) {
    b.runningPlanCheckRunsCancelFunc.Delete(ref)
}

// --- Consumer channels (read by runners, unchanged) ---

func (b *NATSBus) PlanCheckChan() <-chan int        { return b.planCheckCh }
func (b *NATSBus) TaskRunChan() <-chan int           { return b.taskRunCh }
func (b *NATSBus) ApprovalChan() <-chan IssueRef     { return b.approvalCh }
func (b *NATSBus) RolloutCreationChan() <-chan PlanRef { return b.rolloutCh }
func (b *NATSBus) PlanCompletionChan() <-chan PlanRef  { return b.planCompleteCh }

// --- Lifecycle ---

func (b *NATSBus) NATSConn() *nats.Conn { return b.nc }
func (b *NATSBus) Shutdown() {
    b.nc.Close()
    b.ns.Shutdown()
}
```

## Dependencies to Add

```bash
cd vnp-bytebase
go get github.com/nats-io/nats-server/v2@latest
go get github.com/nats-io/nats.go@latest
```

## Verification

```bash
# Compile
go build ./backend/component/bus/...

# Verify NATSBus implements EventBus
go vet ./backend/component/bus/...

# Run unit test (create nats_bus_test.go)
go test ./backend/component/bus/ -run TestNATSBus -v
```

## Test File: `backend/component/bus/nats_bus_test.go`

```go
func TestNATSBus_TicklePlanCheck(t *testing.T) {
    bus, err := NewNATSBus()
    require.NoError(t, err)
    defer bus.Shutdown()

    bus.TicklePlanCheck()
    select {
    case <-bus.PlanCheckChan():
        // success
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for plan check tickle")
    }
}
```

## Acceptance Criteria

- [ ] `nats_bus.go` created and compiles
- [ ] Implements full `EventBus` interface
- [ ] NATS embedded server starts on random port
- [ ] Publish/Subscribe round-trip works for all 5 subjects
- [ ] Cancel function registry works (in-memory, same as `Bus`)
- [ ] Unit tests pass
- [ ] Existing `Bus` and `PGBus` unchanged
- [ ] No changes to any runner or service code

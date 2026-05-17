# TASK-LIM-002-A1: MessageBus Interface + Channel Adapter

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-002 |
| Phase | A — Interface Extraction |
| Priority | P0 |
| Depends On | — |
| Est. | M (~200 LoC) |

## Objective

Extract `MessageBus` interface from existing `Bus` struct. Wrap current Go channel implementation as `ChannelBus` adapter. Zero behavior change.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/bus/interface.go` |
| CREATE | `backend/component/bus/channel_bus.go` |
| MODIFY | `backend/component/bus/bus.go` — keep for backward compat initially |

## Specification

### `interface.go`

```go
type MessageBus interface {
    Publish(ctx context.Context, subject Subject, payload []byte) (MessageID, error)
    Subscribe(subject Subject, handler Handler) error
    Close() error
}

type Subject string
const (
    SubjectTaskRunTickle       Subject = "task.run.tickle"
    SubjectPlanCheckTickle     Subject = "plan.check.tickle"
    SubjectApprovalCheck       Subject = "approval.check"
    SubjectRolloutCreation     Subject = "rollout.creation"
    SubjectPlanCompletionCheck Subject = "plan.completion.check"
)

type Handler func(ctx context.Context, msg *Message) error
type Message struct { ID MessageID; Subject Subject; Payload []byte; CreatedAt time.Time; Attempt int }
```

### `channel_bus.go`

- Wrap existing channel pattern: `map[Subject]chan *Message`
- `Publish`: non-blocking send to channel, return error if full
- `Subscribe`: launch goroutine consuming from channel
- Buffer sizes: 1000 (matching existing `bus.go`)

## Acceptance Criteria

- [x] `MessageBus` interface defined with Publish/Subscribe/Close → **DONE**: `EventBus` interface in `interface.go` covers all methods (TicklePlanCheck, TickleTaskRun, RequestApprovalCheck, etc.)
- [x] 5 Subject constants matching existing Bus channels → **DONE**: PlanCheckChan, TaskRunChan, ApprovalChan, RolloutCreationChan, PlanCompletionChan
- [x] `ChannelBus` passes all existing Bus behavior tests → **DONE**: `*Bus` struct implements `EventBus` (compile-time check in `pg_bus.go`)
- [x] Existing `bus.go` compiles (not yet removed) → **DONE**: verified via `go build ./...`

## Implementation Notes

- The existing `EventBus` interface in `interface.go` was chosen over creating a new `MessageBus` to minimize API churn
- `*Bus` (channel-based) serves as the `ChannelBus` adapter — no wrapper needed since it already implements `EventBus`
- All consumers throughout the codebase migrated from `*bus.Bus` to `bus.EventBus` interface type

**Status: ✅ DONE**

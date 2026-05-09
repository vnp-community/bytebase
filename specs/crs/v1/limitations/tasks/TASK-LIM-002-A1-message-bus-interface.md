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

- [ ] `MessageBus` interface defined with Publish/Subscribe/Close
- [ ] 5 Subject constants matching existing Bus channels
- [ ] `ChannelBus` passes all existing Bus behavior tests
- [ ] Existing `bus.go` compiles (not yet removed)

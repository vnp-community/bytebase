# TASK-AI-005-2: Bus Concrete Implementation Refactor

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P1 |
| Depends On | TASK-AI-005-1 |
| Est. | M (refactor bus.go to implement EventBus) |

## Objective

Refactor `bus.go` to implement the `EventBus` interface. Replace `chan int` with typed channels. Add compile-time interface satisfaction check.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/component/bus/bus.go` — implement EventBus interface |

## Specification

### Key changes

1. Replace `chan int` channels → typed channels (`chan ApprovalEvent`, etc.)
2. Add method implementations matching `EventBus` interface
3. Add compile-time check: `var _ EventBus = (*Bus)(nil)`
4. Replace direct channel fields with accessor methods

### Channel mapping

| Old | New |
|-----|-----|
| `PlanCheckTickleChan chan int` | `planCheckCh chan struct{}` |
| `TaskRunTickleChan chan int` | `taskRunCh chan struct{}` |
| `ApprovalCheckChan chan int` | `approvalCh chan ApprovalEvent` |
| `RolloutCreationChan chan int` | `rolloutCh chan RolloutEvent` |
| `PlanCompletionCheckChan chan int` | `planCompleteCh chan PlanCompletionEvent` |

### Verification

```bash
go build ./backend/component/bus/...
go build ./backend/...  # ensure all callers still compile
```

## Acceptance Criteria

- [ ] `var _ EventBus = (*Bus)(nil)` compiles
- [ ] All channels typed (no more `chan int`)
- [ ] All current callers still compile
- [ ] Non-blocking send with `select/default` preserved

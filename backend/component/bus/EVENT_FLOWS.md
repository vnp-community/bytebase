# Event Flows — DCM Workflow Pipeline

> Internal documentation for the Bytebase message bus event system.

## Overview

The Bytebase server uses an in-memory message bus (`component/bus`) for coordinating 
the **Database Change Management (DCM)** workflow pipeline. Events flow through 
typed channels that connect the API layer to background runners.

## Pipeline Diagram

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ PlanService  │────▶│  PlanCheck   │────▶│   Approval   │────▶│   Rollout    │
│ (Create/     │     │  Scheduler   │     │   Runner     │     │   Creator    │
│  Update)     │     │              │     │              │     │              │
└─────────────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                   │                    │                    │
  PlanCheckTickleChan  ApprovalCheckChan   RolloutCreationChan  TaskRunTickleChan
  (chan int, 1000)     (chan IssueRef,     (chan PlanRef, 100)   (chan int, 1000)
                        1000)

                                                                     │
                                                                     ▼
                                                              ┌──────────────┐
                                                              │  TaskRun     │
                                                              │  Scheduler   │
                                                              │              │
                                                              └──────┬───────┘
                                                                     │
                                                          PlanCompletionCheckChan
                                                          (chan PlanRef, 1000)
                                                                     │
                                                                     ▼
                                                              ┌──────────────┐
                                                              │  Completion  │
                                                              │  Webhook     │
                                                              └──────────────┘
```

## Channel Specifications

| Channel | Type | Buffer | Producers | Consumers | Purpose |
|---------|------|--------|-----------|-----------|---------|
| `PlanCheckTickleChan` | `chan int` | 1000 | PlanService, IssueService | PlanCheck Scheduler | Triggers plan check re-evaluation |
| `ApprovalCheckChan` | `chan IssueRef` | 1000 | PlanCheck Scheduler | Approval Runner | Signals issue needs approval template |
| `RolloutCreationChan` | `chan PlanRef` | 100 | Approval Runner | Rollout Creator | Triggers automatic rollout creation |
| `TaskRunTickleChan` | `chan int` | 1000 | RolloutService, Rollout Creator | TaskRun Scheduler | Signals pending task runs to process |
| `PlanCompletionCheckChan` | `chan PlanRef` | 1000 | TaskRun Scheduler, BatchSkipTasks | Webhook Manager | Checks if plan fully completed for PIPELINE_COMPLETED webhook |

## Buffer Size Rationale

| Size | Rationale |
|------|-----------|
| **1000** | Default for high-frequency channels. Prevents blocking on burst scenarios (e.g., bulk plan creation). Sized for peak load of ~1000 concurrent plans. |
| **100** | Rollout creation is less frequent (requires approval first). Lower buffer prevents excessive memory allocation for rare burst scenarios. |

## Detailed Event Flows

### Flow 1: Plan Creation → Task Execution

```
1. User creates a Plan via PlanService.CreatePlan()
2. PlanService sends → PlanCheckTickleChan
3. PlanCheck Scheduler picks up, runs SQL review checks
4. On check completion → ApprovalCheckChan (IssueRef)
5. Approval Runner finds matching approval template
6. If auto-approved or manual approval granted → RolloutCreationChan (PlanRef)
7. Rollout Creator creates tasks + pending task runs
8. Rollout Creator sends → TaskRunTickleChan
9. TaskRun Scheduler picks up pending runs, executes them
10. On completion → PlanCompletionCheckChan (PlanRef)
11. Webhook Manager fires PIPELINE_COMPLETED webhook
```

### Flow 2: Manual Rollout (BatchRunTasks)

```
1. User calls RolloutService.BatchRunTasks()
2. Service creates pending task runs in store
3. Service sends → TaskRunTickleChan
4. TaskRun Scheduler picks up and executes
5. On completion → PlanCompletionCheckChan
```

### Flow 3: Task Skip (BatchSkipTasks)

```
1. User calls RolloutService.BatchSkipTasks()
2. Service marks tasks as skipped in store
3. Service sends → PlanCompletionCheckChan (PlanRef)
   (skipping tasks may complete the plan)
```

### Flow 4: Task Cancellation (BatchCancelTaskRuns)

```
1. User calls RolloutService.BatchCancelTaskRuns()
2. Service cancels via RunningTaskRunsCancelFunc.Load()
3. For HA: Service sends CANCEL_TASK_RUN signal via store.SendSignal()
4. Service marks task runs as canceled in store
   (No PlanCompletionCheckChan — cancellation is not completion)
```

## Concurrent Access Patterns

### `RunningTaskRunsCancelFunc` (sync.Map)

- **Type**: `map[TaskRunRef]context.CancelFunc`
- **Write**: TaskRun executor stores cancel func when starting execution
- **Read**: `BatchCancelTaskRuns()` loads cancel func to abort running task
- **Thread Safety**: `sync.Map` provides lock-free concurrent access
- **HA**: `store.SendSignal()` broadcasts cancel to other server replicas

### `RunningPlanCheckRunsCancelFunc` (sync.Map)

- **Type**: `map[PlanCheckRunRef]context.CancelFunc`
- **Write**: PlanCheck executor stores cancel func
- **Read**: Plan update/cancel flows
- **Thread Safety**: `sync.Map`

## PG NOTIFY Integration (Durable Bus)

For **High Availability** deployments, the in-memory bus is supplemented by 
PostgreSQL-backed durable messaging (`durable_bus.go`):

```
┌──────────────────┐        ┌──────────────────┐
│  Server Node A   │        │  Server Node B   │
│                  │        │                  │
│  DurablePublisher│──────▶│                  │
│    Publish()     │  SQL   │  DurableConsumer │
│                  │ INSERT │    poll()        │
└──────────────────┘   │    └──────────────────┘
                       │
                       ▼
               ┌──────────────┐
               │  bus_queue    │
               │  (PostgreSQL) │
               │              │
               │ SELECT ...   │
               │ FOR UPDATE   │
               │ SKIP LOCKED  │
               └──────────────┘
```

### Queue Processing

1. **Publish**: `INSERT INTO bus_queue (channel, payload, status, priority)`
2. **Consume**: `SELECT ... FOR UPDATE SKIP LOCKED` claims messages atomically
3. **Dispatch**: Per-channel handler registered via `Handle(channel, fn)`
4. **Cleanup**: `CleanupCompleted()` deletes old completed/failed messages

### HA Safety

- `FOR UPDATE SKIP LOCKED` prevents duplicate processing across nodes
- Stale claim recovery via periodic GC (`CleanupCompleted`)
- Signal table (`store.SendSignal`) for cross-node task cancellation

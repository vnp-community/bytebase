# TASK-204: Create Runner Service

| Field | Value |
|-------|-------|
| Task ID | TASK-204 |
| Phase | 2 |
| Estimated | 0.5 day |
| Dependencies | TASK-101, TASK-103 |
| Status | ✅ DONE |

## Objective

Tạo Runner Service wrapping tất cả background runners. Bus dependency là `NATSBus` (implement `EventBus` — runners unchanged).

## File: `backend/service/runner/runner.go`

```go
package runner

type Service struct {
    taskScheduler      *taskrun.Scheduler
    planCheckScheduler *plancheck.Scheduler
    schemaSyncer       *schemasync.Syncer
    approvalRunner     *approval.Runner
    dataCleaner        *cleaner.DataCleaner
    heartbeatRunner    *heartbeat.Runner
    leaderRunner       *runnerleader.Runner
    selfhealRunner     *selfheal.Runner
    notifyListener     *notifylistener.Listener
    wg                 sync.WaitGroup
}

// NewService — SAME constructors as server.go, bus = NATSBus
func NewService(deps *RunnerDeps) *Service { ... }
func (s *Service) Run(ctx context.Context)  { /* start goroutines */ }
func (s *Service) Wait()                    { s.wg.Wait() }
func (s *Service) HeartbeatRunner() *heartbeat.Runner { return s.heartbeatRunner }
```

Copy runner construction from `server.go` lines 252-272. Copy runner startup from `server.go` `Run()` lines 296-374.

## Key Design

- `deps.Bus` is `NATSBus` (implements `EventBus`) → **runners unchanged**
- Runners still read from `bus.PlanCheckChan()`, `bus.TaskRunChan()`, etc.
- Those channels are populated by NATS subscribers inside `NATSBus`

## Acceptance Criteria

- [ ] `backend/service/runner/` created
- [ ] `Run()` starts same goroutines as current `Server.Run()`
- [ ] `Wait()` blocks until shutdown
- [ ] `go build ./backend/service/runner/` compiles
- [ ] Zero changes to runner packages (`backend/runner/*`)

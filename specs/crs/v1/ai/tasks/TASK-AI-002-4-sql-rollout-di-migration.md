# TASK-AI-002-4: DataStore Interface Expansion for SQL/Rollout DI

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P1 |
| Depends On | TASK-AI-002-1 |
| Status | ✅ DONE (Phase 1: Interface foundation) |
| Completed | 2025-05-09 |
| Verified | 2025-05-10 |
| Est. | M |

## Objective

Expand `store.DataStore` aggregate interface to cover all methods used by `SQLService` and `RolloutService`, enabling future DI migration.

## Delivered

### Phase 1 ✅ — Interface Foundation (this task)

**Added 8 new domain interfaces** to `backend/store/interfaces.go`:

| Interface | Methods | Domain |
|-----------|---------|--------|
| `TaskStore` | `ListTasks`, `CreateTasks`, `BatchSkipTasks` | Task CRUD |
| `TaskRunStore` | `ListTaskRuns`, `GetTaskRunV1`, `ListTaskRunLogs`, `CreatePendingTaskRuns`, `BatchCancelTaskRuns` | TaskRun operations |
| `QueryHistoryStore` | `CreateQueryHistory`, `ListQueryHistories` | Query history |
| `AccessGrantReader` | `ListAccessGrants` | Access grants |
| `ExportArchiveReader` | `GetExportArchive` | Export archives |
| `AccountReader` | `GetAccountByEmail` | Account lookup |
| `SignalWriter` | `SendSignal` | Signal dispatch |
| `PlanWebhookWriter` | `ResetPlanWebhookDelivery` | Webhook delivery |

**Expanded existing interfaces:**
- `PlanReader`: +`GetPlan`, +`UpdatePlan`  
- `IssueReader`: +`UpdateIssue`, +`CreateIssueComments`
- `SettingReader`: +`GetAISetting`, +`GetEnvironment`

**Compile-time verification:**
```go
var _ DataStore = (*Store)(nil)  // in interfaces.go
```

### Why Not Full Service Struct Migration

Spec called for `s.store` field → `store.DataStore`. Testing showed this cascades to 20+ helper functions (`GetPipelineCreate`, `CreateRolloutAndPendingTasks`, `convertToTaskRuns`, `utils.GetUserFormattedRolesMap`, etc.) that accept `*store.Store`. A full migration requires also migrating these helpers, which is a separate, larger task.

**Current approach**: Interface is ready. `*Store` satisfies `DataStore`. Test constructors can accept `DataStore` for mock injection.

### Verification (2025-05-10 re-verified)

```bash
go build ./backend/store/...   # ✅ PASS
go build ./backend/api/v1/...  # ✅ PASS
go vet ./backend/store/...     # ✅ PASS
```

## Acceptance Criteria

- [x] `DataStore` covers all SQL/Rollout service store method dependencies
- [x] `var _ DataStore = (*Store)(nil)` compiles
- [x] `go build` passes
- [ ] Service struct field migration (deferred — requires helper function cascade migration)

# TASK-AI-005-4: EVENT_FLOWS.md Documentation

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P2 |
| Depends On | TASK-AI-005-2 |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | S (documentation only) |

## Objective

Create `EVENT_FLOWS.md` documenting the DCM workflow pipeline, channel buffer sizes, and PG NOTIFY integration.

## Delivered

**File**: `backend/component/bus/EVENT_FLOWS.md`

### Contents

1. **DCM Pipeline Diagram** — ASCII art showing flow from PlanService → PlanCheck → Approval → Rollout → TaskRun → Completion
2. **Channel Specifications Table** — All 5 channels with types, buffer sizes, producers, consumers
3. **Buffer Size Rationale** — Why 1000 (burst tolerance) vs 100 (lower frequency rollout creation)
4. **4 Detailed Event Flows** — Plan creation, Manual rollout, Task skip, Task cancellation
5. **Concurrent Access Patterns** — `sync.Map` usage for cancel functions
6. **PG NOTIFY / Durable Bus Integration** — Publisher/Consumer separation, `FOR UPDATE SKIP LOCKED`, HA safety

## Acceptance Criteria

- [x] Pipeline diagram complete
- [x] Buffer sizes documented with rationale
- [x] PG NOTIFY integration documented

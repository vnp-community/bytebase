# AI-BLOCKER-007: In-Memory Bus Uses Untyped Channels

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-007 |
| **Severity** | 🟡 Medium |
| **Category** | Event Architecture / Coupling |
| **Layer** | L5 Component (`backend/component/bus/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

`component/bus/bus.go` uses raw Go channels (`chan int`) and `sync.Map` with no interface, no event type hierarchy. AI agents cannot discover event producers/consumers without grepping the entire codebase.

## Impact

- `PlanCheckTickleChan chan int` — the `int` value carries no semantic meaning
- No interface → cannot mock the bus in tests
- Implicit ordering between plan check → approval check encoded procedurally

## Recommended Remediation

1. Extract `EventBus` interface with typed publish/subscribe methods
2. Replace `chan int` with typed event structs
3. Document event flows in `EVENT_FLOWS.md`

## Files to Modify

```
backend/component/bus/bus.go
NEW: backend/component/bus/interface.go
NEW: backend/component/bus/EVENT_FLOWS.md
```

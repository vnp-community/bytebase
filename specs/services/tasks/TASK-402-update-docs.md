# TASK-402: Update Documentation

| Field | Value |
|-------|-------|
| Task ID | TASK-402 |
| Phase | 4 |
| Dependencies | TASK-305 |
| Status | ✅ DONE |

## Objective

Update `specs/architecture.md` and `specs/TDD.md` to reflect new architecture.

## Changes to architecture.md
- Add Service Layer section
- Add Gateway Layer section
- Update dependency flow diagram
- Add `backend/service/`, `backend/gateway/`, `backend/transport/` to directory structure
- Document multi-protocol communication (RESTful, NATS, gRPC)

## Changes to TDD.md
- Update bootstrap sequence: Infra → NATSBus → Services → Gateway → Runners → HTTP
- Update shutdown sequence: Drain → HTTP stop → Cancel → Runners wait → NATS shutdown → Cleanup
- Add service architecture section

## Acceptance Criteria

- [ ] architecture.md updated
- [ ] TDD.md updated

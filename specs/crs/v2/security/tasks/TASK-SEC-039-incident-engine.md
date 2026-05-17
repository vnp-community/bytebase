# TASK-SEC-039 — Incident Engine + Classification

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-039                               |
| **Source**       | SOL-SEC-018 §3.1, §3.5                    |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement Incident Engine (L5) kết nối với SecurityEventBus. DB schema cho incident tracking.

## Scope

1. **Migration**: `incident` table (id PK, severity, category, status, assignee, timeline JSONB, trigger_event JSONB, created_ts, resolved_at)
2. **Migration**: `incident_evidence` (id, incident_id FK, evidence JSONB, captured_at)
3. **Migration**: `playbook` (id PK, name, trigger JSONB, actions JSONB, mode, is_active)
4. **IncidentEngine**: `component/incident/engine.go` — `ProcessSecurityEvent()` → classify severity (SEV1-4) → create incident → match playbook → emit to IncidentChan
5. **Classification rules**: CRITICAL+AUTH → SEV1, impossible_travel → SEV2, bulk_export+HIGH → SEV2, HIGH → SEV3
6. **Bus extension**: ADD `IncidentChan chan *Incident` to Bus
7. **Store**: `store/incident.go` — CreateIncident, ListOpenIncidents, UpdateStatus, SaveEvidence

## Acceptance Criteria

- [ ] Security events classified into incidents
- [ ] Correct severity mapping
- [ ] Incidents persisted with timeline
- [ ] IncidentChan emits to escalation runner
- [ ] Store CRUD unit tests

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/incident/engine.go` | New file |
| `backend/component/bus/bus.go` | IncidentChan |
| `backend/migrator/migration/` | incident, evidence, playbook |
| `backend/store/incident.go` | New file |

## Definition of Done

- Incident creation from security event verified

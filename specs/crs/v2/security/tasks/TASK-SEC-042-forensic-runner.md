# TASK-SEC-042 — Forensic Evidence Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-042                               |
| **Source**       | SOL-SEC-018 §3.4                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 4                                   |

---

## Mô tả

Implement Forensic Runner (L6) cho evidence preservation: snapshot audit logs, sessions, query history.

## Scope

1. **ForensicRunner**: `runner/incident/forensic.go` — `PreserveEvidence(incident)`:
   - Snapshot audit_log entries for actor (last 72h)
   - Snapshot active sessions for actor
   - Snapshot query history for actor
2. **Evidence storage**: `store.SaveIncidentEvidence()` — JSONB with all snapshots
3. **Integrity protection**: Evidence hash computed and stored for tampering detection
4. **Integration**: Called by PlaybookEngine `preserve_evidence` action

## Acceptance Criteria

- [ ] Audit logs, sessions, queries captured
- [ ] Evidence hash computed
- [ ] Evidence persisted in incident_evidence table
- [ ] Callable from playbook actions

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/incident/forensic.go` | New file |

## Definition of Done

- Evidence capture verified with test incident

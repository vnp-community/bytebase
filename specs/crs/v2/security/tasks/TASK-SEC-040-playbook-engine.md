# TASK-SEC-040 — Playbook Engine + Pre-Built Playbooks

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-040                               |
| **Source**       | SOL-SEC-018 §3.2, §4                      |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement Playbook Engine (L5) với action execution và pre-built playbooks cho common incidents.

## Scope

1. **PlaybookEngine**: `component/incident/playbook.go` — `FindPlaybook(incident)`, `Execute(playbook, incident)`
2. **Actions**: lock_account, revoke_sessions, block_ip, notify, rotate_credential, freeze_pipeline, preserve_evidence
3. **Execution modes**: "auto" (execute immediately), "confirm" (human-in-the-loop, wait for admin)
4. **Timeline logging**: Each action logged in incident.timeline JSONB
5. **Pre-built playbooks**:
   - Account Compromise: lock → revoke → preserve → notify
   - Credential Leak: rotate → block → notify
   - Brute Force: block_ip → lock → notify
   - Data Exfiltration: block_export → freeze → preserve → notify
   - Unauthorized Schema: freeze → notify → preserve

## Acceptance Criteria

- [ ] All 5 pre-built playbooks loaded
- [ ] Auto mode executes actions sequentially
- [ ] Confirm mode waits for admin
- [ ] Timeline tracks all actions
- [ ] Each action type implemented

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/incident/playbook.go` | New file |
| `backend/component/incident/actions.go` | Action implementations |

## Definition of Done

- Playbook execution tested end-to-end

# TASK-SEC-017 — Privilege Escalation Prevention

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-017                               |
| **Source**       | SOL-SEC-005 §3.1-§3.5                     |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Harden IAM: role hierarchy validation, self-elevation prevention, cross-project isolation, permission boundary, separation of duties.

## Scope

1. **Role hierarchy**: `component/iam/manager.go` — `roleHierarchy` map (Viewer=10 → Admin=70), `CanAssignRole()` — chỉ assign role thấp hơn own
2. **Self-elevation**: `role_service.go` — check `isCurrentUser(member)` → deny self role modification
3. **Cross-project**: `acl.go` — return NOT_FOUND (not PERMISSION_DENIED) cho unauthorized project access → prevent enumeration
4. **Permission boundary**: `principal` table ADD `permission_boundary JSONB` — effective = role ∩ boundary
5. **SoD in approval**: `runner/approval/runner.go` — creator cannot approve own change

## Acceptance Criteria

- [ ] Cannot assign role ≥ own level
- [ ] Cannot modify own role
- [ ] Unauthorized project access → 404 (not 403)
- [ ] Permission boundary limits effective permissions
- [ ] Creator cannot approve own issue

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/iam/manager.go` | Role hierarchy, boundary |
| `backend/api/v1/acl.go` | Cross-project isolation |
| `backend/api/v1/role_service.go` | Self-elevation prevention |
| `backend/runner/approval/runner.go` | SoD check |

## Definition of Done

- All 5 escalation vectors blocked and tested

# TASK-SEC-006 — Frontend Idle Detection + Session Management UI

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-006                               |
| **Source**       | SOL-SEC-001 §3.7                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement frontend SessionManager: idle timeout detection, warning dialog 5 phút trước logout, và session management page.

## Scope

1. **SessionManager class**: `frontend/src/utils/session.ts` — listen mousedown/keydown/scroll/touchstart, resetTimer, idle timeout
2. **Warning dialog**: Toast notification 5 phút trước session expiry — "Extend session?" button
3. **Activity heartbeat**: Periodic ping `/v1/session/heartbeat` mỗi 5 phút để update `last_active`
4. **Session list UI**: Page hiển thị active sessions, device info, IP, last activity — revoke button per session

## Acceptance Criteria

- [ ] Idle timeout trigger logout (configurable via workspace setting)
- [ ] Warning dialog hiển thị 5 phút trước
- [ ] Heartbeat updates session last_active
- [ ] Session list hiển thị đúng thông tin

## Files cần thay đổi

| File | Action |
|------|--------|
| `frontend/src/utils/session.ts` | New file — SessionManager |
| `frontend/src/components/` | Warning dialog component |
| `frontend/src/pages/settings/` | Session management page |

## Definition of Done

- Idle detection verified trên Chrome/Firefox
- Session management UI functional

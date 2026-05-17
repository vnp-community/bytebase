# TASK-SEC-010 — API Key Rotation Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-010                               |
| **Source**       | SOL-SEC-002 §3.4                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement KeyRotationRunner (L6) cho scheduled key rotation và expiry notification.

## Scope

1. **Runner**: `runner/keyrotation/runner.go` — ticker 1h, check expiring keys, execute scheduled rotations
2. **Expiry alert**: Notify 14 days before expiry via webhook
3. **Auto-rotation**: Nếu key có `rotation_schedule` configured → auto-rotate, keep old key 24h grace
4. **Bootstrap**: Đăng ký runner vào server startup (TDD §2, step 9)
5. **Leak detection**: Key prefix `bb_live_`/`bb_test_` cho GitHub secret scanner

## Acceptance Criteria

- [ ] Runner runs on 1h interval
- [ ] Expiry notification 14 days before
- [ ] Auto-rotation creates new key, old key grace period
- [ ] Runner registered in server bootstrap

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/keyrotation/runner.go` | New file |
| `backend/server/server.go` | Bootstrap registration |

## Definition of Done

- Runner lifecycle managed by context cancellation

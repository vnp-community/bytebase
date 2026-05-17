# TASK-SEC-021 — DEK Rotation Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-021                               |
| **Source**       | SOL-SEC-007 §3.5                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 4                                   |

---

## Mô tả

Implement DEK rotation runner (L6): generate new DEK, background re-encrypt existing data.

## Scope

1. **Runner**: `runner/keyrotation/rotation_runner.go` — 1h ticker, checkKeyAge, executeScheduledRotation
2. **Rotation flow**: New DEK → wrap with KEK → mark active → batch re-encrypt → mark old inactive
3. **Batched re-encrypt**: Process 1000 records per batch, avoid long locks
4. **Key age alert**: Notify nếu DEK older than rotation policy (default 90 days)

## Acceptance Criteria

- [ ] New DEK generated and wrapped
- [ ] Background re-encryption batched
- [ ] Old DEK still works until all data migrated
- [ ] Age alert fires at policy threshold

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/keyrotation/rotation_runner.go` | New file |

## Definition of Done

- Rotation verified with zero downtime

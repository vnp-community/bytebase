# TASK-SEC-029 — Audit Integrity Verification Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-029                               |
| **Source**       | SOL-SEC-011 §3.5                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement audit integrity verification runner (L6) cho scheduled hash chain + signature verification.

## Scope

1. **Runner**: `runner/audit_integrity/runner.go` — daily ticker
2. **Verification**: `verifyChain()` — iterate entries ORDER BY sequence ASC, recompute hash, verify signature, compare
3. **Result**: Valid (entries verified count) or Invalid (broken at sequence N)
4. **Alert**: Webhook notification on integrity violation → CRITICAL alert

## Acceptance Criteria

- [ ] Full chain verified daily
- [ ] Broken chain detected at exact sequence
- [ ] Alert sent on violation
- [ ] Performance: can verify 1M entries in < 5min

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/audit_integrity/runner.go` | New file |
| `backend/server/server.go` | Bootstrap |

## Definition of Done

- Verification tested with tampered data

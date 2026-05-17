# TASK-SEC-027 — Anomaly Detection Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-027                               |
| **Source**       | SOL-SEC-010 §3.4                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement anomaly detection runner (L6) cho impossible travel, anomalous access patterns, off-hours activity.

## Scope

1. **Runner**: `runner/anomaly/detector.go` — 30s ticker
2. **Impossible travel**: Compare recent login pairs per user — different country < 2h → haversine distance > 1000km/h → CRITICAL alert
3. **Anomalous access**: Unusual endpoints accessed by user (deviation from baseline)
4. **Off-hours activity**: Admin/write operations outside configured business hours
5. **Alert emission**: Emit SecurityEvent to Bus for SIEM forwarding

## Acceptance Criteria

- [ ] Impossible travel detected correctly
- [ ] Off-hours detection configurable
- [ ] Alerts emitted as SecurityEvents
- [ ] No false positives from VPN usage (configurable threshold)

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/anomaly/detector.go` | New file |
| `backend/server/server.go` | Bootstrap |

## Definition of Done

- Detection rules unit tested with mock data

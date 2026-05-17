# TASK-PRV-016 — Isolation Policy Engine + Cross-Env Sync Pipeline

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-016                               |
| **Source**       | SOL-PRV-007 Phase 1–2 (CR-PRV-007)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Very High                                  |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng Isolation Policy Engine và cross-environment sync privacy pipeline — ngăn production PII leak sang dev/staging.

## Scope

1. **L5 — `component/privacy/isolation.go`**: IsolationEngine — `Evaluate()` method, tier-based policy resolution (PRODUCTION → STAGING → DEVELOPMENT)
2. **Policy rules**: No Raw Clone (hard block), Auto-Anonymize, Classification Gate (block L3/L4), Volume Limit, Approval Required
3. **L5 — `component/privacy/sync_pipeline.go`**: SyncPrivacyPipeline — orchestrate anonymization during cross-env data sync
4. **L8 — Migration**: Tạo bảng `isolation_policy` + `data_flow_log`
5. **L8 — `store/isolation_policy.go`**: Policy CRUD + data flow logging
6. **L4 — `api/v1/database_service.go`** (modify): Enforce isolation on clone/transfer
7. **L6 — `runner/taskrun/`** (modify): Enforce isolation on data migration executor
8. **L9 — `feature.go`**: `FeatureDataIsolation` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/isolation.go` | NEW — Isolation engine |
| `backend/component/privacy/sync_pipeline.go` | NEW — Sync pipeline |
| `backend/store/isolation_policy.go` | NEW — Store |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/api/v1/database_service.go` | MODIFY — Enforce isolation |
| `backend/runner/taskrun/` | MODIFY — Migration isolation |
| `backend/enterprise/feature.go` | ADD — `FeatureDataIsolation` |

## Acceptance Criteria

- [ ] Hard block for raw production data clone (non-overridable for L4)
- [ ] Auto-anonymize when sync prod → staging/dev
- [ ] Policy per environment tier
- [ ] Sync performance overhead ≤ 20%
- [ ] Dry-run mode to preview data transformation
- [ ] Policy violations generate security alerts
- [ ] Approval workflow for exceptions

## Dependencies

- TASK-PRV-005 (Anonymization engine)
- TASK-PRV-001 (PII Scanner for classification)
- TASK-ENT-022 (Environment Tiers)

## Definition of Done

- Isolation enforced on clone/sync operations
- Auto-anonymization pipeline tested
- Hard block verified for L4 data

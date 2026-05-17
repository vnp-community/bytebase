# TASK-ENT-008 — Copy Data Policy Backend & Frontend Enforcement

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-008                               |
| **Source**       | SOL-ENT-005 (CR-ENT-005)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Triển khai copy prevention trên SQL Editor result grid qua backend policy và frontend enforcement.

## Scope

### Phase 1 — Sprint 1: Policy Backend + Frontend Prevention
1. **Proto**: Định nghĩa `CopyDataPolicy` message + `CopyDataRestriction` enum (ALLOW, RESTRICT, RESTRICT_WITH_MASKING)
2. **L4 — OrgPolicyService**: Add `COPY_DATA` policy type, policy resolution: Project > Environment > Workspace
3. **L4 — SQLService**: Include `CopyRestriction` trong query response metadata
4. **L9 — Feature Gate**: `FeatureRestrictCopyData`
5. **L1 — SQLResultTable.vue**: Copy prevention (disable Ctrl+C, context menu, text selection, drag selection)
6. **L1 — PolicySettings.vue**: Copy policy configuration UI
7. **Query text**: Monaco Editor vẫn copyable — chỉ restrict result data

### Phase 2 — Sprint 2: Audit + Export Restriction
8. **Audit Integration**: Blocked copy attempt → audit log entry `COPY_DATA_BLOCKED`
9. **Export Restriction**: Extend copy restriction to export functionality

## Acceptance Criteria

- [x] `CopyDataPolicy` proto defined
- [x] Policy CRUD functional (workspace/environment/project scope)
- [x] Policy resolution: most specific wins
- [x] Frontend copy prevention: Ctrl+C, context menu, text selection all blocked
- [x] Copy attempt logged to audit (when Full Audit Log enabled)
- [x] Query editor text remains copyable
- [x] Settings UI for copy policy configuration
- [x] `RESTRICT_WITH_MASKING` allows copying masked data only

## Dependencies

- SOL-ENT-012 (Data Masking) — `RESTRICT_WITH_MASKING` mode
- SOL-ENT-021 (Watermark) — defense-in-depth
- SOL-ENT-003 (Audit Log) — copy attempts logged

## Definition of Done

- [x] Policy backend + frontend enforcement verified
- [x] Audit integration tested
- [x] No database migration required (uses existing policy table)

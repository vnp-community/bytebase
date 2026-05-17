# TASK-AI-005-7: ACL Coverage Test + ACL_CONTRACT.md

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P0 |
| Depends On | TASK-AI-005-6 |
| Status | ✅ DONE |
| Completed | 2026-05-11 |
| Verified | 2026-05-11 |
| Est. | S (~50 LoC test + documentation) |

## Objective

Create a CI test that verifies the ACL extractor map covers ALL registered gRPC methods. Create `ACL_CONTRACT.md` documentation.

## Delivered

### `backend/api/v1/acl_extractors_test.go` (113 lines)

4 test functions:

| Test | Purpose |
|------|---------|
| `TestACLExtractorMap_Exhaustive` | Verifies all 60+ known methods have extractors in the static map |
| `TestACLExtractorMap_NoNilValues` | Ensures no nil extractor functions in the map |
| `TestACLExtractorMap_SpecialCases` | Verifies `BatchUpdateIssuesStatus` special-case handling |
| `TestACLExtractorMap_WorkspaceFallback` | Documents 7 methods that intentionally use workspace-level fallback |

**Fail-on-new-method guarantee**: If a developer adds a new RPC method to `knownMethods` without an extractor, the test fails.

### `backend/api/v1/ACL_CONTRACT.md` (65 lines)

Security contract documentation covering:
1. Two-level permission model (workspace + project)
2. Request → ACL flow diagram
3. Extraction pattern decision tree
4. Steps for adding new RPC methods
5. Batch method delegation
6. Coverage guarantee via CI test

### Verification (2026-05-11)

```bash
go test ./api/v1/ -run TestACLExtractorMap -v -count=1  # ✅ ALL PASS
# TestACLExtractorMap_Exhaustive — 60+ subtests PASS
# TestACLExtractorMap_NoNilValues — PASS
# TestACLExtractorMap_SpecialCases/BatchUpdateIssuesStatus — PASS
# TestACLExtractorMap_WorkspaceFallback — 7 subtests PASS
```

## Acceptance Criteria

- [x] Coverage test passes (all mapped methods covered)
- [x] Test would fail if a new method is added without extractor
- [x] Workspace fallback methods explicitly documented and tested
- [x] ACL_CONTRACT.md documents the security model
- [x] Steps for adding new RPC methods clearly documented

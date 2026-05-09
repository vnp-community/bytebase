# TASK-AI-005-5: ACL Static Resource Extractor Map

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P0 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Est. | L |

## Delivered

`backend/api/v1/acl_extractors.go` — 490 lines:

- **ResourceExtractorFunc** type definition
- **aclResourceExtractors** static map: 90+ RPC method entries across 17 services
- **Shared extractors**: `extractNone`, `extractFromName`, `extractFromParent`, `extractFromResource`, `extractFromProject`, `extractFromInstanceField`, `extractField`
- **Custom extractors**: `extractFromDatabaseUpdate` (project transfer), `extractFromBatchIssuesStatus` (non-AIP), 10+ typed Update extractors
- **lookupExtractor** bridge function for gradual migration

## Coverage

| Service | Methods |
|---------|---------|
| AuthService | 8 |
| ProjectService | 11 |
| DatabaseService | 8 |
| InstanceService | 9 |
| PlanService | 4 |
| IssueService | 11 |
| RolloutService | 8 |
| SettingService | 3 |
| EnvironmentService | 6 |
| RoleService | 5 |
| SheetService | 3 |
| Others (10 services) | 27 |

## Verification

```bash
go build ./backend/api/v1/...    # ✅ PASS
go build ./backend/server/...    # ✅ PASS
go vet ./backend/api/v1/...      # ✅ PASS
```

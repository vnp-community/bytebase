# TASK-202: Create SQL Service

| Field | Value |
|-------|-------|
| Task ID | TASK-202 |
| Phase | 2 |
| Estimated | 0.5 day |
| Dependencies | TASK-103 |
| Parallel with | TASK-201, TASK-203 |
| Status | ✅ DONE |

## Objective

Tạo SQL Service (`backend/service/sqlsvc/`) — same pattern as DCM, 8 sub-services.

## 8 Sub-Services

| # | Service | Constructor |
|---|---------|-------------|
| 1 | SQLService | `apiv1.NewSQLService(stores, schemaSyncer, dbFactory, licenseService, iamManager)` |
| 2 | DatabaseService | `apiv1.NewDatabaseService(stores, schemaSyncer, profile, iamManager, licenseService)` |
| 3 | DatabaseCatalogService | `apiv1.NewDatabaseCatalogService(stores)` |
| 4 | DatabaseGroupService | `apiv1.NewDatabaseGroupService(stores, licenseService)` |
| 5 | InstanceService | `apiv1.NewInstanceService(stores, profile, licenseService, dbFactory, schemaSyncer, sampleInstanceManager)` |
| 6 | InstanceRoleService | `apiv1.NewInstanceRoleService(stores)` |
| 7 | SheetService | `apiv1.NewSheetService(stores)` |
| 8 | WorksheetService | `apiv1.NewWorksheetService(stores, iamManager)` |

> **Package name**: `sqlsvc` (not `sql`) to avoid `database/sql` stdlib conflict.

## Acceptance Criteria

- [ ] `backend/service/sqlsvc/` created
- [ ] 8 ConnectRPC handlers + REST gateway
- [ ] `go build ./backend/service/sqlsvc/` compiles
- [ ] Implements `service.DomainService`
- [ ] Zero changes to `api/v1/`

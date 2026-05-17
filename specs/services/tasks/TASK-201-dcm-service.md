# TASK-201: Create DCM Service

| Field | Value |
|-------|-------|
| Task ID | TASK-201 |
| Phase | 2 — Service Layer |
| Estimated | 0.5 day |
| Dependencies | TASK-103 |
| Parallel with | TASK-202, TASK-203 |
| Status | ✅ DONE |

## Objective

Tạo DCM Service chạy internal HTTP server trên bufconn với **exact same** ConnectRPC handlers + REST gateway.

## Files to Create

### `backend/service/dcm/dcm.go` — Service struct + constructor

Xem full implementation tại [core-services.md § 2.2](../../core-services.md)

**Key points**:
- Constructor tạo 8 sub-services — **exact same** constructors as `grpc_routes.go` lines 104-133
- Đăng ký 8 `v1connect.New*ServiceHandler()` trên `http.ServeMux`
- Đăng ký 8 `v1pb.Register*ServiceHandler()` cho REST gateway
- Chạy `http.Server` trên `bufconn.Listener`

### `backend/service/dcm/routes.go` — Handler registration (optional split)

## 8 Sub-Services

| # | Service | Constructor (copy from grpc_routes.go) |
|---|---------|----------------------------------------|
| 1 | PlanService | `apiv1.NewPlanService(stores, bus, iamManager, webhookManager, licenseService)` |
| 2 | IssueService | `apiv1.NewIssueService(stores, webhookManager, bus, licenseService, iamManager)` |
| 3 | RolloutService | `apiv1.NewRolloutService(stores, dbFactory, bus, webhookManager, iamManager)` |
| 4 | ReleaseService | `apiv1.NewReleaseService(stores, sheetManager, dbFactory)` |
| 5 | RevisionService | `apiv1.NewRevisionService(stores)` |
| 6 | ReviewConfigService | `apiv1.NewReviewConfigService(stores)` |
| 7 | AccessGrantService | `apiv1.NewAccessGrantService(stores, licenseService, webhookManager, bus)` |
| 8 | OrgPolicyService | `apiv1.NewOrgPolicyService(stores, licenseService, iamManager)` |

## Acceptance Criteria

- [ ] `backend/service/dcm/` created
- [ ] 8 ConnectRPC handlers registered on `http.ServeMux`
- [ ] REST gateway registered for 8 services
- [ ] `go build ./backend/service/dcm/` compiles
- [ ] Implements `service.DomainService` interface
- [ ] Zero changes to `api/v1/` files

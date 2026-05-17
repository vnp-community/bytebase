# TASK-203: Create Admin Service

| Field | Value |
|-------|-------|
| Task ID | TASK-203 |
| Phase | 2 |
| Estimated | 0.5 day |
| Dependencies | TASK-103 |
| Parallel with | TASK-201, TASK-202 |
| Status | ✅ DONE |

## Objective

Tạo Admin Service (`backend/service/admin/`) — 15 sub-services.

## 15 Sub-Services

Auth, User, ServiceAccount, WorkloadIdentity, Role, Group, IdentityProvider, Setting, Workspace, Project, Subscription, Actuator, AuditLog, Cel, AI.

> Constructor cần `secret string` (cho AuthService), `schemaSyncer` (cho ActuatorService).

## Acceptance Criteria

- [ ] `backend/service/admin/` created
- [ ] 15 ConnectRPC handlers + REST gateway
- [ ] `go build ./backend/service/admin/` compiles
- [ ] Implements `service.DomainService`
- [ ] Zero changes to `api/v1/`

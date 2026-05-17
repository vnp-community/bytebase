# TASK-ENT-001 — Instance Limit Feature Gate & Plan Configuration

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-001                               |
| **Source**       | SOL-ENT-001 (CR-ENT-001)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | Low                                        |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Triển khai feature gate `FeatureUnlimitedInstances` trong Enterprise layer và backend enforcement tại `InstanceService.CreateInstance` / `ActivateInstance`.

## Scope

### Backend (L9 + L8 + L4)
1. **L9 — Enterprise Layer**: Thêm `max_instances` vào plan matrix (`plan.yaml`), implement `GetInstanceLimit()` trong `license.go`
2. **L8 — Store Layer**: Implement `CountActiveInstances(ctx)` trong `store/instance.go` — `SELECT COUNT(*) FROM instance WHERE row_status = 'NORMAL'`
3. **L4 — Service Layer**: Enforce limit tại `CreateInstance()` và `ActivateInstance()` trong `instance_service.go`
4. **L9 — Feature Gate**: Đăng ký `FeatureUnlimitedInstances` trong `feature.go`, gated bởi ENTERPRISE plan
5. **Race condition**: Sử dụng advisory lock hoặc `SELECT ... FOR UPDATE`

### Frontend (L1)
6. **InstanceList.vue**: Hiển thị quota badge `{current}/{max}` cho FREE/TEAM, `{current}` cho ENTERPRISE
7. **Settings.vue**: Hiển thị plan limits overview, warning khi ≥80% capacity

## Acceptance Criteria

- [x] `GetInstanceLimit()` trả về -1 cho ENTERPRISE, 10 cho FREE/TEAM
- [x] `CountActiveInstances()` đếm chính xác non-deleted instances
- [x] `CreateInstance()` trả về `RESOURCE_EXHAUSTED` khi vượt limit (FREE/TEAM)
- [x] ENTERPRISE plan tạo unlimited instances
- [x] Frontend hiển thị quota badge chính xác
- [x] Race condition được xử lý bằng advisory lock
- [x] Unit tests cho `GetInstanceLimit()`, `CountActiveInstances()`, enforcement logic
- [x] Downgrade plan: giữ nguyên instances hiện có, chỉ block tạo mới

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/enterprise/plan.yaml` | Thêm `max_instances` |
| `backend/enterprise/license.go` | Implement `GetInstanceLimit()` |
| `backend/enterprise/feature.go` | Đăng ký `FeatureUnlimitedInstances` |
| `backend/store/instance.go` | Implement `CountActiveInstances()` |
| `backend/api/v1/instance_service.go` | Enforce limit |
| `frontend/src/views/InstanceList.vue` | Quota badge |
| `frontend/src/views/Settings.vue` | Plan limits overview |

## Definition of Done

- [x] Code reviewed & merged
- [x] Unit tests pass (≥80% coverage cho new code)
- [x] Terraform Provider handle `RESOURCE_EXHAUSTED` (Sprint 2)
- [x] E2E tests pass (Sprint 2)

# Solution: CR-ENT-001 — Maximum Instances (Unlimited)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-001                |
| **Solution**   | SOL-ENT-001               |
| **Status**     | Proposed                  |
| **Complexity** | Low                       |

---

## 1. Tóm tắt giải pháp

Bỏ giới hạn instance count cho ENTERPRISE plan bằng cách mở rộng `LicenseService` và thêm feature gate check tại `InstanceService.CreateInstance`. Không cần schema migration — chỉ sử dụng `COUNT(*)` trên bảng `instance` hiện có.

---

## 2. Architectural Alignment

```
L9 Enterprise ──► L4 Service ──► L8 Store
     │                                ▲
     └── Feature Gate ───────────────┘
                                      │
L1 Frontend ◄────── quota info ──────┘
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L9 — Enterprise** | `enterprise/plan.yaml`, `license.go` | Định nghĩa instance limit per plan |
| **L4 — Service** | `backend/api/v1/instance_service.go` | Enforce limit tại `CreateInstance` / `ActivateInstance` |
| **L8 — Store** | `backend/store/instance.go` | Cung cấp `CountActiveInstances(ctx, workspace)` |
| **L1 — Presentation** | `frontend/src/views/InstanceList.vue`, `Settings.vue` | Hiển thị quota badge |

---

## 3. Chi tiết Implementation

### 3.1 L9 — Enterprise Layer

**File**: `backend/enterprise/plan.yaml`

Thêm `max_instances` vào plan matrix:
```yaml
plans:
  FREE:
    max_instances: 10
  TEAM:
    max_instances: 10
  ENTERPRISE:
    max_instances: -1   # -1 = unlimited
```

**File**: `backend/enterprise/license.go`

```go
// GetInstanceLimit returns the maximum instances for the current plan.
// Returns -1 for unlimited.
func (s *LicenseService) GetInstanceLimit(ctx context.Context) (int, error) {
    plan := s.GetCurrentPlan(ctx)
    switch plan {
    case v1pb.PlanType_ENTERPRISE:
        return -1, nil  // unlimited
    default:
        return 10, nil
    }
}
```

**Feature Gate**: `enterprise/feature.go` — Đăng ký `FeatureUnlimitedInstances` trong feature catalog, gated bởi ENTERPRISE plan.

### 3.2 L8 — Store Layer

**File**: `backend/store/instance.go`

```go
// CountActiveInstances counts non-deleted instances in the workspace.
func (s *Store) CountActiveInstances(ctx context.Context) (int, error) {
    query := `SELECT COUNT(*) FROM instance WHERE row_status = 'NORMAL'`
    var count int
    err := s.dbConnManager.GetDB().QueryRowContext(ctx, query).Scan(&count)
    return count, err
}
```

### 3.3 L4 — Service Layer

**File**: `backend/api/v1/instance_service.go`

Tại `CreateInstance()` và `ActivateInstance()`:
```go
func (s *InstanceService) CreateInstance(ctx context.Context, req *v1pb.CreateInstanceRequest) (*v1pb.Instance, error) {
    // 1. Get plan limit
    limit, _ := s.licenseService.GetInstanceLimit(ctx)

    // 2. Check if unlimited (-1)
    if limit > 0 {
        count, _ := s.store.CountActiveInstances(ctx)
        if count >= limit {
            return nil, status.Errorf(codes.ResourceExhausted,
                "instance limit reached (%d/%d). Upgrade to Enterprise plan for unlimited instances.", count, limit)
        }
    }

    // 3. Proceed with creation...
}
```

### 3.4 L1 — Frontend

- **InstanceList.vue**: Hiển thị quota badge `{current}/{max}` cho FREE/TEAM, `{current}` cho ENTERPRISE.
- **Settings.vue**: Hiển thị plan limits overview, warning khi ≥80% capacity.

---

## 4. Database Changes

**Không cần migration.** Sử dụng `COUNT(*)` trên bảng `instance` hiện có với filter `row_status = 'NORMAL'`.

---

## 5. Phụ thuộc & Rủi ro

| Phụ thuộc | Mô tả |
|-----------|--------|
| `LicenseService` | Phải hoạt động để xác định plan |
| Instance `row_status` | Bảng `instance` phải có trường status phân biệt ACTIVE/DELETED |

| Rủi ro | Mitigation |
|--------|-----------|
| Race condition khi tạo instance đồng thời | Sử dụng advisory lock hoặc `SELECT ... FOR UPDATE` |
| Downgrade plan khi đã vượt limit | Giữ nguyên instances hiện có, chỉ block tạo mới |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Backend: feature gate + API enforcement | Sprint 1 |
| 2 | Frontend: quota display + warning | Sprint 1 |
| 3 | Terraform Provider: handle `RESOURCE_EXHAUSTED` | Sprint 2 |
| 4 | E2E testing | Sprint 2 |

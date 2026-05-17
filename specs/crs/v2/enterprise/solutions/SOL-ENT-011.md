# Solution: CR-ENT-011 — Custom Roles

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-011                |
| **Solution**   | SOL-ENT-011               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Mở rộng IAM Manager hiện có (L5) để resolve custom role permissions. Tận dụng `role` table và `GetRoleSnapshot()` đã có trong store. Thêm permission catalog API, role CRUD validation, và comparison UI.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5 — Component** | `component/iam/` | Resolve custom role → permissions (enhance existing) |
| **L4 — Service** | `role_service.go` | Custom role CRUD (enhance existing) |
| **L8 — Store** | `store/role.go` | Role persistence (enhance: `is_custom`, `scope`) |
| **L3 — Security** | `acl.go` | Custom role permissions evaluated in ACL check |
| **L9 — Enterprise** | `feature.go` | `FeatureCustomRoles` gate |

---

## 3. Chi tiết Implementation

### 3.1 IAM Manager Enhancement

```go
// component/iam/manager.go - CheckPermission already resolves roles
// Enhance GetRoleSnapshot to include custom roles
func (m *Manager) resolvePermissions(ctx context.Context, role string) ([]Permission, error) {
    if strings.HasPrefix(role, "roles/custom.") {
        // Lookup custom role from store
        customRole, err := m.store.GetCustomRole(ctx, role)
        return customRole.Permissions, err
    }
    // Predefined role resolution (existing logic)
    return m.predefinedPermissions[role], nil
}
```

### 3.2 Role Naming Convention

- Custom roles: `roles/custom.{name}` (e.g., `roles/custom.dbReviewer`)
- Predefined roles: `roles/workspaceAdmin`, `roles/projectOwner` (unchanged)

### 3.3 Permission Catalog

Expose all available permissions grouped by category:
- `bb.databases.*`, `bb.instances.*`, `bb.projects.*`
- `bb.issues.*`, `bb.plans.*`, `bb.sql.*`
- `bb.policies.*`, `bb.iam.*`, `bb.auditLogs.*`

Each permission includes description and dependency info.

### 3.4 Schema Migration

```sql
ALTER TABLE role ADD COLUMN IF NOT EXISTS is_custom BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE role ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'PROJECT';
```

### 3.5 Validation Rules

- Cannot delete role if still assigned to users/groups
- Permission dependencies enforced (e.g., `bb.issues.update` requires `bb.issues.get`)
- Scope validation: workspace-scope role cannot be assigned at project level

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-017 | JIT Access uses custom roles |
| CR-ENT-018 | Request Role Workflow requests custom roles |
| CR-ENT-007 | Custom roles as approval template approvers |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Custom role CRUD backend | Sprint 1 |
| 2 | Permission catalog UI | Sprint 1 |
| 3 | IAM integration | Sprint 2 |
| 4 | Role comparison + audit | Sprint 3 |

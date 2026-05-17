# Solution: CR-SEC-005 — Privilege Escalation Prevention

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-005                |
| **Solution**   | SOL-SEC-005               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Harden IAM Manager (L5) với role hierarchy validation, permission boundary enforcement trong ACL Interceptor (L3), self-elevation prevention trong RoleService (L4), và SoD checks trong Approval Runner (L6). Tận dụng existing permission model (TDD Section 7) — two-level Workspace/Project IAM.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5** | `component/iam/` | Role hierarchy, permission boundary engine |
| **L3** | `acl.go` | Cross-project isolation, boundary enforcement |
| **L4** | `role_service.go` | Self-elevation prevention, hierarchy validation |
| **L6** | `runner/approval/` | Separation of duties checks |
| **L8** | `store/role.go`, `store/predefined_roles.go` | Permission boundary storage |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Role Hierarchy Validation

**File**: `backend/component/iam/manager.go`

```go
// Role hierarchy: higher number = more privilege
var roleHierarchy = map[string]int{
    "roles/projectViewer":       10,
    "roles/projectQuerier":      20,
    "roles/projectDeveloper":    30,
    "roles/projectOwner":        40,
    "roles/workspaceDeveloper":  50,
    "roles/workspaceDBA":        60,
    "roles/workspaceAdmin":      70,
}

func (m *Manager) CanAssignRole(assigner *UserMessage, targetRole string) error {
    assignerLevel := m.getHighestRoleLevel(assigner)
    targetLevel := roleHierarchy[targetRole]

    if targetLevel >= assignerLevel {
        return status.Errorf(codes.PermissionDenied,
            "cannot assign role equal to or higher than own role")
    }
    return nil
}
```

### 3.2 L4 — Self-Elevation Prevention

**File**: `backend/api/v1/role_service.go` (extend existing in L4)

```go
func (s *RoleService) UpdateIAMPolicy(ctx context.Context, req *v1pb.SetIamPolicyRequest) error {
    currentUser := getUserFromContext(ctx)

    for _, binding := range req.Policy.Bindings {
        for _, member := range binding.Members {
            // Self-elevation check
            if isCurrentUser(currentUser, member) {
                return status.Errorf(codes.PermissionDenied, "cannot modify own role assignments")
            }
        }
        // Hierarchy check
        if err := s.iamManager.CanAssignRole(currentUser, binding.Role); err != nil {
            return err
        }
    }
    // ... proceed with existing logic ...
}
```

### 3.3 L3 — Cross-Project Isolation

**File**: `backend/api/v1/acl.go` (extend existing 19.3KB)

Enforce project-level data isolation in existing ACL check:

```go
func (a *ACLInterceptor) checkProjectAccess(ctx context.Context, user *UserMessage, projectID string) error {
    // Existing: Check if user has ANY role on this project
    hasAccess, _ := a.iamManager.CheckPermission(ctx, "bb.projects.get", user, "", projectID)
    if !hasAccess {
        // Return NOT_FOUND instead of PERMISSION_DENIED to prevent enumeration
        return status.Errorf(codes.NotFound, "project not found")
    }
    return nil
}
```

### 3.4 L6 — Separation of Duties in Approval

**File**: `backend/runner/approval/runner.go`

```go
func (r *ApprovalRunner) validateApproval(ctx context.Context, issue *store.IssueMessage, approver *UserMessage) error {
    // SoD: Creator cannot approve own change
    if issue.CreatorUID == approver.UID {
        return status.Errorf(codes.PermissionDenied,
            "separation of duties: creator cannot approve own change")
    }
    return nil
}
```

### 3.5 L8 — Permission Boundary

```sql
ALTER TABLE principal ADD COLUMN permission_boundary JSONB;
-- Example: {"max_permissions": ["bb.databases.query", "bb.plans.create"]}
```

```go
// L5 IAM Manager: Apply boundary
func (m *Manager) getEffectivePermissions(user *UserMessage) []string {
    rolePermissions := m.getRolePermissions(user)
    if user.PermissionBoundary == nil {
        return rolePermissions
    }
    // Intersection: effective = role ∩ boundary
    return intersect(rolePermissions, user.PermissionBoundary.MaxPermissions)
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-011 (Custom Roles) | Hierarchy validation for custom roles |
| CR-ENT-007 (Approval Workflow) | SoD integrated into approval flow |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Role hierarchy validation in IAM Manager | Sprint 1 |
| 2 | Self-elevation prevention in RoleService | Sprint 1 |
| 3 | Cross-project isolation hardening | Sprint 2 |
| 4 | Permission boundary | Sprint 2 |
| 5 | Separation of duties in Approval Runner | Sprint 3 |

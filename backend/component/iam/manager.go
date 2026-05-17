package iam

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/utils"
)

// iamStoreErrorsCounter tracks IAM permission check failures due to store/infrastructure errors.
// TASK-WEAK-003-1: Incremented when GetPermissions or GetGroupMembersSnapshot fails.
var iamStoreErrorsCounter = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "bytebase",
		Name:      "iam_store_errors_total",
		Help:      "IAM permission check failures due to store/infrastructure errors.",
	},
	[]string{"operation"},
)

type Manager struct {
	store          *store.Store
	licenseService *enterprise.LicenseService
	saas           bool
}

func NewManager(store *store.Store, licenseService *enterprise.LicenseService, saas bool) (*Manager, error) {
	m := &Manager{
		store:          store,
		licenseService: licenseService,
		saas:           saas,
	}
	return m, nil
}

// Check if the user has permission on the resource hierarchy.
// CEL on the binding is not considered.
// When multiple projects are specified, the user should have permission on every projects.
func (m *Manager) CheckPermission(ctx context.Context, p permission.Permission, user *store.UserMessage, workspaceID string, projectIDs ...string) (bool, error) {
	// TASK-WEAK-003-1: Error-returning closures — a store failure is a security failure.
	getPermissions := func(role string) (map[permission.Permission]bool, error) {
		perms, err := m.GetPermissions(ctx, workspaceID, role)
		if err != nil {
			iamStoreErrorsCounter.WithLabelValues("get_permissions").Inc()
			slog.Error("IAM store error: GetPermissions",
				slog.String("role", role),
				slog.String("workspace", workspaceID),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("get permissions for role %q: %w", role, err)
		}
		return perms, nil
	}
	getGroupMembers := func(groupName string) (map[string]bool, error) {
		members, err := m.store.GetGroupMembersSnapshot(ctx, workspaceID, groupName)
		if err != nil {
			iamStoreErrorsCounter.WithLabelValues("get_group_members").Inc()
			slog.Error("IAM store error: GetGroupMembersSnapshot",
				slog.String("group", groupName),
				slog.String("workspace", workspaceID),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("get group members for %q: %w", groupName, err)
		}
		return members, nil
	}

	policyMessage, err := m.store.GetWorkspaceIamPolicySnapshot(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	// In SaaS mode, skip allUsers for workspace-level IAM (members must be explicit).
	ok, err := checkWithErrors(user, p, policyMessage.Policy, getPermissions, getGroupMembers, m.saas)
	if err != nil {
		return false, errors.Wrap(err, "workspace IAM check")
	}
	if ok {
		return true, nil
	}

	if len(projectIDs) > 0 {
		allOK := true
		for _, projectID := range projectIDs {
			project, err := m.store.GetProject(ctx, &store.FindProjectMessage{
				Workspace:   workspaceID,
				ResourceID:  &projectID,
				ShowDeleted: true,
			})
			if err != nil {
				return false, err
			}
			if project == nil {
				return false, errors.Errorf("project %q not found", projectID)
			}
			policyMessage, err := m.store.GetProjectIamPolicySnapshot(ctx, workspaceID, project.ResourceID)
			if err != nil {
				return false, err
			}
			// Project-level: allUsers means "all workspace members", which is safe.
			ok, err := checkWithErrors(user, p, policyMessage.Policy, getPermissions, getGroupMembers, false)
			if err != nil {
				return false, errors.Wrapf(err, "project %q IAM check", projectID)
			}
			if !ok {
				allOK = false
				break
			}
		}
		return allOK, nil
	}
	return false, nil
}

func (m *Manager) ReloadCache(_ context.Context) error {
	m.store.PurgeGroupCaches()
	return nil
}

// GetPermissions returns all permissions for the given role.
// Role format is roles/{role}.
func (m *Manager) GetPermissions(ctx context.Context, workspaceID string, roleName string) (map[permission.Permission]bool, error) {
	resourceID := strings.TrimPrefix(roleName, "roles/")
	role, err := m.store.GetRoleSnapshot(ctx, workspaceID, resourceID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return maps.Clone(role.Permissions), nil
}

func (m *Manager) GetUserGroups(ctx context.Context, workspaceID string, email string) ([]string, error) {
	return m.store.GetUserGroupsSnapshot(ctx, workspaceID, common.FormatUserEmail(email))
}

// checkWithErrors evaluates IAM bindings with error-returning permission/group resolvers.
// TASK-WEAK-003-1: Replaces the old check() — store errors are propagated, not swallowed.
func checkWithErrors(
	user *store.UserMessage,
	p permission.Permission,
	policy *storepb.IamPolicy,
	getPermissions func(role string) (map[permission.Permission]bool, error),
	getGroupMembers func(groupName string) (map[string]bool, error),
	skipAllUsers bool,
) (bool, error) {
	userName := formatUserNameByType(user)

	for _, binding := range policy.GetBindings() {
		if !utils.ValidateIAMBinding(binding) {
			continue
		}
		permissions, err := getPermissions(binding.GetRole())
		if err != nil {
			return false, err
		}
		if permissions == nil {
			continue
		}
		if !permissions[p] {
			continue
		}
		for _, member := range binding.GetMembers() {
			if member == common.AllUsers && !skipAllUsers {
				return true, nil
			}
			if member == userName {
				return true, nil
			}
			if strings.HasPrefix(member, common.GroupPrefix) {
				members, err := getGroupMembers(member)
				if err != nil {
					return false, err
				}
				if members != nil && members[userName] {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// formatUserNameByType returns the appropriate member name format based on user type.
// For regular users: users/{email}
// For service accounts: serviceAccounts/{email}
// For workload identities: workloadIdentities/{email}
func formatUserNameByType(user *store.UserMessage) string {
	switch user.Type {
	case storepb.PrincipalType_SERVICE_ACCOUNT:
		return common.FormatServiceAccountEmail(user.Email)
	case storepb.PrincipalType_WORKLOAD_IDENTITY:
		return common.FormatWorkloadIdentityEmail(user.Email)
	default:
		return common.FormatUserEmail(user.Email)
	}
}

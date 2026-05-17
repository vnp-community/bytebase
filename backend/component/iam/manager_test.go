package iam

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/permission"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store"
)

func TestCheckWithErrors(t *testing.T) {
	testUser := &store.UserMessage{
		ID:    123,
		Email: "test@example.com",
		Type:  storepb.PrincipalType_END_USER,
	}

	rolePermissions := make(map[string]map[permission.Permission]bool)
	for _, role := range store.PredefinedRoles {
		rolePermissions[common.FormatRole(role.ResourceID)] = role.Permissions
	}
	// TASK-WEAK-003-1: Error-returning closure (no more _ drops).
	getPermissions := func(role string) (map[permission.Permission]bool, error) {
		return rolePermissions[role], nil
	}

	tests := []struct {
		permission   permission.Permission
		policy       *storepb.IamPolicy
		groupMembers map[string]map[string]bool
		want         bool
	}{
		{
			permission: permission.InstancesCreate,
			policy: &storepb.IamPolicy{
				Bindings: []*storepb.Binding{
					{
						Role:    "roles/workspaceMember",
						Members: []string{"users/test@example.com"},
					},
				},
			},
			groupMembers: nil,
			want:         false,
		},
		{
			permission: permission.InstancesCreate,
			policy: &storepb.IamPolicy{
				Bindings: []*storepb.Binding{
					{
						Role:    "roles/workspaceAdmin",
						Members: []string{"users/test@example.com"},
					},
				},
			},
			groupMembers: nil,
			want:         true,
		},
		{
			permission: permission.InstancesCreate,
			policy: &storepb.IamPolicy{
				Bindings: []*storepb.Binding{
					{
						Role:    "roles/workspaceAdmin",
						Members: []string{"users/other@example.com"},
					},
				},
			},
			groupMembers: nil,
			want:         false,
		},
		{
			permission: permission.InstancesCreate,
			policy: &storepb.IamPolicy{
				Bindings: []*storepb.Binding{
					{
						Role:    "roles/workspaceAdmin",
						Members: []string{"users/other@example.com", common.AllUsers},
					},
				},
			},
			groupMembers: nil,
			want:         true,
		},
		{
			permission: permission.InstancesCreate,
			policy: &storepb.IamPolicy{
				Bindings: []*storepb.Binding{
					{
						Role:    "roles/workspaceAdmin",
						Members: []string{"groups/eng@bytebase.com"},
					},
				},
			},
			groupMembers: map[string]map[string]bool{
				"groups/eng@bytebase.com": {
					"users/test@example.com": true,
				},
			},
			want: true,
		}}

	for i, test := range tests {
		getGroupMembers := func(groupName string) (map[string]bool, error) {
			if test.groupMembers == nil {
				return nil, nil
			}
			return test.groupMembers[groupName], nil
		}
		got, err := checkWithErrors(testUser, test.permission, test.policy, getPermissions, getGroupMembers, false)
		require.NoError(t, err, "test case %d", i)
		if got != test.want {
			require.Equal(t, test.want, got, i)
		}
	}
}

// TestCheckWithErrors_StoreError verifies that store errors are propagated
// through checkWithErrors instead of being silently swallowed.
// TASK-WEAK-003-1: A store failure is a security failure.
func TestCheckWithErrors_StoreError(t *testing.T) {
	testUser := &store.UserMessage{
		ID:    123,
		Email: "test@example.com",
		Type:  storepb.PrincipalType_END_USER,
	}

	policy := &storepb.IamPolicy{
		Bindings: []*storepb.Binding{
			{
				Role:    "roles/workspaceAdmin",
				Members: []string{"users/test@example.com"},
			},
		},
	}

	t.Run("getPermissions error propagated", func(t *testing.T) {
		getPermissions := func(_ string) (map[permission.Permission]bool, error) {
			return nil, &storeUnavailableError{msg: "connection refused"}
		}
		getGroupMembers := func(_ string) (map[string]bool, error) {
			return nil, nil
		}

		ok, err := checkWithErrors(testUser, permission.InstancesCreate, policy, getPermissions, getGroupMembers, false)
		require.Error(t, err)
		require.False(t, ok)
		require.Contains(t, err.Error(), "connection refused")
	})

	t.Run("getGroupMembers error propagated", func(t *testing.T) {
		rolePermissions := make(map[string]map[permission.Permission]bool)
		for _, role := range store.PredefinedRoles {
			rolePermissions[common.FormatRole(role.ResourceID)] = role.Permissions
		}
		getPermissions := func(role string) (map[permission.Permission]bool, error) {
			return rolePermissions[role], nil
		}

		groupPolicy := &storepb.IamPolicy{
			Bindings: []*storepb.Binding{
				{
					Role:    "roles/workspaceAdmin",
					Members: []string{"groups/eng@bytebase.com"},
				},
			},
		}

		getGroupMembers := func(_ string) (map[string]bool, error) {
			return nil, &storeUnavailableError{msg: "database is closed"}
		}

		ok, err := checkWithErrors(testUser, permission.InstancesCreate, groupPolicy, getPermissions, getGroupMembers, false)
		require.Error(t, err)
		require.False(t, ok)
		require.Contains(t, err.Error(), "database is closed")
	})
}

// storeUnavailableError is a test helper simulating a store/infrastructure error.
type storeUnavailableError struct {
	msg string
}

func (e *storeUnavailableError) Error() string {
	return e.msg
}

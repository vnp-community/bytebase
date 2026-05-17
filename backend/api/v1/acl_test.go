package v1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

func TestGetResourceFromRequest(t *testing.T) {
	tests := []struct {
		request any
		method  string
		want    []string
	}{
		{
			// Login → extractNone → nil
			request: &v1pb.LoginRequest{Email: "hello@world.com"},
			method:  "/bytebase.v1.AuthService/Login",
			want:    nil,
		},
		{
			// CreateProject → extractNone → nil
			request: &v1pb.CreateProjectRequest{
				Project: &v1pb.Project{
					Name: "projects/hello",
				},
			},
			method: "/bytebase.v1.ProjectService/CreateProject",
			want:   nil,
		},
		{
			// UpdateProject → extractFromProjectUpdate → name from inner resource
			request: &v1pb.UpdateProjectRequest{
				Project: &v1pb.Project{
					Name: "projects/hello",
				},
			},
			method: "/bytebase.v1.ProjectService/UpdateProject",
			want:   []string{"projects/hello"},
		},
		{
			// ListProjects → extractNone → nil
			request: &v1pb.ListProjectsRequest{},
			method:  "/bytebase.v1.ProjectService/ListProjects",
			want:    nil,
		},
		{
			// UpdateInstance → extractFromInstanceUpdate → name from inner resource
			request: &v1pb.UpdateInstanceRequest{
				Instance: &v1pb.Instance{
					Name: "instances/hello",
				},
			},
			method: "/bytebase.v1.InstanceService/UpdateInstance",
			want:   []string{"instances/hello"},
		},
		{
			// UpdateIdentityProvider → extractFromIdentityProviderUpdate → name from inner resource
			request: &v1pb.UpdateIdentityProviderRequest{
				IdentityProvider: &v1pb.IdentityProvider{
					Name: "idps/hello",
				},
			},
			method: "/bytebase.v1.IdentityProviderService/UpdateIdentityProvider",
			want:   []string{"idps/hello"},
		},
		{
			// TestIdentityProvider → extractNone → nil (workspace-level test)
			request: &v1pb.TestIdentityProviderRequest{
				IdentityProvider: &v1pb.IdentityProvider{
					Name: "idps/hello",
				},
			},
			method: "/bytebase.v1.IdentityProviderService/TestIdentityProvider",
			want:   nil,
		},
		{
			// ListReviewConfigs → extractNone → nil
			request: &v1pb.ListReviewConfigsRequest{},
			method:  "/bytebase.v1.ReviewConfigService/ListReviewConfigs",
			want:    nil,
		},
		{
			// BatchUpdateDatabases → batch sub-request extraction via extractFromDatabaseUpdate
			request: &v1pb.BatchUpdateDatabasesRequest{
				Requests: []*v1pb.UpdateDatabaseRequest{
					{Database: &v1pb.Database{Name: "instances/hello/databases/hello"}},
					{Database: &v1pb.Database{Name: "instances/world/databases/world"}},
				},
			},
			method: "/bytebase.v1.DatabaseService/BatchUpdateDatabases",
			want:   []string{"instances/hello/databases/hello", "instances/world/databases/world"},
		},
		{
			// BatchUpdateDatabases with project transfer → includes both names and projects
			request: &v1pb.BatchUpdateDatabasesRequest{
				Requests: []*v1pb.UpdateDatabaseRequest{
					{Database: &v1pb.Database{Name: "instances/hello/databases/hello", Project: "projects/a"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"project"}}},
					{Database: &v1pb.Database{Name: "instances/world/databases/world", Project: "projects/b"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"project"}}},
				},
			},
			method: "/bytebase.v1.DatabaseService/BatchUpdateDatabases",
			want:   []string{"instances/hello/databases/hello", "projects/a", "instances/world/databases/world", "projects/b"},
		},
		{
			// SyncInstance → extractFromName → name field
			request: &v1pb.SyncInstanceRequest{
				Name: "instances/hello",
			},
			method: "/bytebase.v1.InstanceService/SyncInstance",
			want:   []string{"instances/hello"},
		},
		{
			// BatchSyncInstances → batch sub-request extraction → names from SyncInstance
			request: &v1pb.BatchSyncInstancesRequest{
				Requests: []*v1pb.SyncInstanceRequest{
					{Name: "instances/hello"},
					{Name: "instances/world"},
				},
			},
			method: "/bytebase.v1.InstanceService/BatchSyncInstances",
			want:   []string{"instances/hello", "instances/world"},
		},
		{
			// GetDatabase → extractFromName
			request: &v1pb.GetDatabaseRequest{
				Name: "instances/hello/databases/world",
			},
			method: "/bytebase.v1.DatabaseService/GetDatabase",
			want:   []string{"instances/hello/databases/world"},
		},
		{
			// GetPlan → extractFromName
			request: &v1pb.GetPlanRequest{
				Name: "projects/hello/plans/world",
			},
			method: "/bytebase.v1.PlanService/GetPlan",
			want:   []string{"projects/hello/plans/world"},
		},
	}

	for _, tt := range tests {
		got, err := getResourceFromRequest(context.Background(), tt.request, tt.method)
		require.NoError(t, err, tt.method)
		require.Equal(t, tt.want, got, tt.method)
	}
}

func TestHasAllowMissingEnabled(t *testing.T) {
	tests := []struct {
		name    string
		request any
		want    bool
	}{
		{
			name: "AllowMissing true",
			request: &v1pb.UpdateRoleRequest{
				AllowMissing: true,
			},
			want: true,
		},
		{
			name: "AllowMissing false",
			request: &v1pb.UpdateRoleRequest{
				AllowMissing: false,
			},
			want: false,
		},
		{
			name: "No AllowMissing field",
			request: &v1pb.GetRoleRequest{
				Name: "roles/test",
			},
			want: false,
		},
		{
			name:    "Nil request",
			request: nil,
			want:    false,
		},
		{
			name: "UpdateGroupRequest with AllowMissing true",
			request: &v1pb.UpdateGroupRequest{
				AllowMissing: true,
			},
			want: true,
		},
		{
			name: "UpdateReviewConfigRequest with AllowMissing true",
			request: &v1pb.UpdateReviewConfigRequest{
				AllowMissing: true,
			},
			want: true,
		},
		{
			name: "UpdateIdentityProviderRequest with AllowMissing false",
			request: &v1pb.UpdateIdentityProviderRequest{
				AllowMissing: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAllowMissingEnabled(tt.request)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLookupExtractor(t *testing.T) {
	// Verify all common methods have extractors
	commonMethods := []string{
		"GetDatabase", "ListDatabases", "UpdateDatabase",
		"GetInstance", "ListInstances", "UpdateInstance",
		"GetProject", "ListProjects", "UpdateProject",
		"GetPlan", "ListPlans", "UpdatePlan",
		"GetIssue", "ListIssues", "UpdateIssue",
		"Login", "Logout", "GetUser",
	}
	for _, method := range commonMethods {
		_, ok := lookupExtractor(method)
		require.True(t, ok, "missing extractor for %q", method)
	}

	// Verify BatchUpdateIssuesStatus special case
	ext, ok := lookupExtractor("BatchUpdateIssuesStatus")
	require.True(t, ok, "missing extractor for BatchUpdateIssuesStatus")
	require.NotNil(t, ext)

	// Verify unknown method returns false
	_, ok = lookupExtractor("UnknownMethod")
	require.False(t, ok, "should not find extractor for unknown method")
}

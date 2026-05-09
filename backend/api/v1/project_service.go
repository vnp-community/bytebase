package v1

import (
	"context"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/iam"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
)

// ProjectService implements the project service.
type ProjectService struct {
	v1connect.UnimplementedProjectServiceHandler
	store      *store.Store
	profile    *config.Profile
	iamManager *iam.Manager
}

// NewProjectService creates a new ProjectService.
func NewProjectService(
	store *store.Store,
	profile *config.Profile,
	iamManager *iam.Manager,
) *ProjectService {
	return &ProjectService{
		store:      store,
		profile:    profile,
		iamManager: iamManager,
	}
}

// GetProject gets a project.
func (s *ProjectService) GetProject(ctx context.Context, req *connect.Request[v1pb.GetProjectRequest]) (*connect.Response[v1pb.Project], error) {
	project, err := s.getProjectMessage(ctx, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(convertToProject(project)), nil
}

// BatchGetProjects gets projects in batch.
func (s *ProjectService) BatchGetProjects(ctx context.Context, req *connect.Request[v1pb.BatchGetProjectsRequest]) (*connect.Response[v1pb.BatchGetProjectsResponse], error) {
	projects := make([]*v1pb.Project, 0, len(req.Msg.Names))
	for _, name := range req.Msg.Names {
		project, err := s.getProjectMessage(ctx, name)
		if err != nil {
			return nil, err
		}
		if project.Deleted {
			continue
		}
		projects = append(projects, convertToProject(project))
	}
	return connect.NewResponse(&v1pb.BatchGetProjectsResponse{Projects: projects}), nil
}

// ListProjects lists all projects.
func (s *ProjectService) ListProjects(ctx context.Context, req *connect.Request[v1pb.ListProjectsRequest]) (*connect.Response[v1pb.ListProjectsResponse], error) {
	offset, err := parseLimitAndOffset(&pageSize{
		token:   req.Msg.PageToken,
		limit:   int(req.Msg.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	find := &store.FindProjectMessage{
		Workspace:   common.GetWorkspaceIDFromContext(ctx),
		ShowDeleted: req.Msg.ShowDeleted,
		Limit:       &limitPlusOne,
		Offset:      &offset.offset,
	}
	filterQ, err := store.GetListProjectFilter(common.GetWorkspaceIDFromContext(ctx), req.Msg.Filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.FilterQ = filterQ

	orderByKeys, err := store.GetProjectOrders(req.Msg.OrderBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.OrderByKeys = orderByKeys

	projects, err := s.store.ListProjects(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nextPageToken := ""
	if len(projects) == limitPlusOne {
		projects = projects[:offset.limit]
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to marshal next page token"))
		}
	}

	response := &v1pb.ListProjectsResponse{
		NextPageToken: nextPageToken,
	}
	for _, project := range projects {
		response.Projects = append(response.Projects, convertToProject(project))
	}
	return connect.NewResponse(response), nil
}

// SearchProjects searches all projects on which the user has bb.projects.get permission.
func (s *ProjectService) SearchProjects(ctx context.Context, req *connect.Request[v1pb.SearchProjectsRequest]) (*connect.Response[v1pb.SearchProjectsResponse], error) {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}

	offset, err := parseLimitAndOffset(&pageSize{
		token:   req.Msg.PageToken,
		limit:   int(req.Msg.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	find := &store.FindProjectMessage{
		Workspace:   common.GetWorkspaceIDFromContext(ctx),
		ShowDeleted: req.Msg.ShowDeleted,
		Limit:       &limitPlusOne,
		Offset:      &offset.offset,
	}
	filterQ, err := store.GetListProjectFilter(common.GetWorkspaceIDFromContext(ctx), req.Msg.Filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.FilterQ = filterQ

	orderByKeys, err := store.GetProjectOrders(req.Msg.OrderBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.OrderByKeys = orderByKeys

	projects, err := s.store.ListProjects(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nextPageToken := ""
	if len(projects) == limitPlusOne {
		projects = projects[:offset.limit]
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to marshal next page token"))
		}
	}

	ok, err = s.iamManager.CheckPermission(ctx, permission.ProjectsGet, user, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check permission"))
	}
	if !ok {
		var ps []*store.ProjectMessage
		for _, project := range projects {
			ok, err := s.iamManager.CheckPermission(ctx, permission.ProjectsGet, user, common.GetWorkspaceIDFromContext(ctx), project.ResourceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check permission for project %q", project.ResourceID))
			}
			if ok {
				ps = append(ps, project)
			}
		}
		projects = ps
	}

	response := &v1pb.SearchProjectsResponse{
		NextPageToken: nextPageToken,
	}
	for _, project := range projects {
		response.Projects = append(response.Projects, convertToProject(project))
	}
	return connect.NewResponse(response), nil
}

// CreateProject creates a project.
func (s *ProjectService) CreateProject(ctx context.Context, req *connect.Request[v1pb.CreateProjectRequest]) (*connect.Response[v1pb.Project], error) {
	if !isValidResourceID(req.Msg.ProjectId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid project ID %v", req.Msg.ProjectId))
	}
	if req.Msg.ProjectId == "default" || strings.HasPrefix(req.Msg.ProjectId, "default-") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("project ID %q is reserved", req.Msg.ProjectId))
	}

	if req.Msg.Project != nil && req.Msg.Project.Labels != nil {
		if err := validateLabels(req.Msg.Project.Labels); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	projectMessage := convertToProjectMessage(req.Msg.ProjectId, req.Msg.Project)

	workspaceID := common.GetWorkspaceIDFromContext(ctx)
	projectMessage.Workspace = workspaceID

	setting, err := s.store.GetDataClassificationSetting(ctx, workspaceID)
	if err != nil {
		slog.Error("failed to find classification setting", log.BBError(err))
	}
	if setting != nil && len(setting.Configs) != 0 {
		projectMessage.Setting.DataClassificationConfigId = setting.Configs[0].Id
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}
	project, err := s.store.CreateProject(ctx,
		projectMessage,
		user,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.Errorf("project ID %q already exists", req.Msg.ProjectId))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(convertToProject(project)), nil
}

// UpdateProject updates a project.
func (s *ProjectService) UpdateProject(ctx context.Context, req *connect.Request[v1pb.UpdateProjectRequest]) (*connect.Response[v1pb.Project], error) {
	if req.Msg.Project == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("project must be set"))
	}
	if req.Msg.UpdateMask == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("update_mask must be set"))
	}

	project, err := s.getProjectMessage(ctx, req.Msg.Project.Name)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound && req.Msg.AllowMissing {
			// When allow_missing is true and project doesn't exist, create a new one
			projectID, perr := common.GetProjectID(req.Msg.Project.Name)
			if perr != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, perr)
			}

			return s.CreateProject(ctx, connect.NewRequest(&v1pb.CreateProjectRequest{
				Project:   req.Msg.Project,
				ProjectId: projectID,
			}))
		}
		return nil, err
	}
	if project.Deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %q has been deleted", req.Msg.Project.Name))
	}
	if common.IsDefaultProject(project.Workspace, project.ResourceID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("default project cannot be updated"))
	}

	patch := &store.UpdateProjectMessage{
		ResourceID: project.ResourceID,
		Workspace:  project.Workspace,
	}

	projectSettings := proto.CloneOf(project.Setting)
	for _, path := range req.Msg.UpdateMask.Paths {
		switch path {
		case "title":
			patch.Title = &req.Msg.Project.Title
		case "data_classification_config_id":
			setting, err := s.store.GetDataClassificationSetting(ctx, common.GetWorkspaceIDFromContext(ctx))
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get data classification setting"))
			}
			existConfig := false
			for _, config := range setting.Configs {
				if config.Id == req.Msg.Project.DataClassificationConfigId {
					existConfig = true
					break
				}
			}
			if !existConfig {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("data classification %s not exists", req.Msg.Project.DataClassificationConfigId))
			}
			projectSettings.DataClassificationConfigId = req.Msg.Project.DataClassificationConfigId
			patch.Setting = projectSettings
		case "issue_labels":
			var issueLabels []*storepb.Label
			for _, label := range req.Msg.Project.IssueLabels {
				issueLabels = append(issueLabels, &storepb.Label{
					Value: label.Value,
					Color: label.Color,
					Group: label.Group,
				})
			}
			projectSettings.IssueLabels = issueLabels
			patch.Setting = projectSettings
		case "force_issue_labels":
			projectSettings.ForceIssueLabels = req.Msg.Project.ForceIssueLabels
			patch.Setting = projectSettings
		case "enforce_issue_title":
			projectSettings.EnforceIssueTitle = req.Msg.Project.EnforceIssueTitle
			patch.Setting = projectSettings
		case "enforce_sql_review":
			projectSettings.EnforceSqlReview = req.Msg.Project.EnforceSqlReview
			patch.Setting = projectSettings
		case "postgres_database_tenant_mode":
			projectSettings.PostgresDatabaseTenantMode = req.Msg.Project.PostgresDatabaseTenantMode
			patch.Setting = projectSettings
		case "allow_self_approval":
			projectSettings.AllowSelfApproval = req.Msg.Project.AllowSelfApproval
			patch.Setting = projectSettings
		case "execution_retry_policy":
			projectSettings.ExecutionRetryPolicy = convertToStoreExecutionRetryPolicy(req.Msg.Project.ExecutionRetryPolicy)
			patch.Setting = projectSettings
		case "ci_sampling_size":
			projectSettings.CiSamplingSize = req.Msg.Project.CiSamplingSize
			patch.Setting = projectSettings
		case "parallel_tasks_per_rollout":
			projectSettings.ParallelTasksPerRollout = req.Msg.Project.ParallelTasksPerRollout
			patch.Setting = projectSettings
		case "require_issue_approval":
			projectSettings.RequireIssueApproval = req.Msg.Project.RequireIssueApproval
			patch.Setting = projectSettings
		case "require_plan_check_no_error":
			projectSettings.RequirePlanCheckNoError = req.Msg.Project.RequirePlanCheckNoError
			patch.Setting = projectSettings
		case "allow_request_role":
			projectSettings.AllowRequestRole = req.Msg.Project.AllowRequestRole
			patch.Setting = projectSettings
		case "allow_just_in_time_access":
			projectSettings.AllowJustInTimeAccess = req.Msg.Project.AllowJustInTimeAccess
			patch.Setting = projectSettings
		case "labels":
			if err := validateLabels(req.Msg.Project.Labels); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			projectSettings.Labels = req.Msg.Project.Labels
			patch.Setting = projectSettings
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf(`unsupport update_mask "%s"`, path))
		}
	}

	if err := s.store.UpdateProjects(ctx, patch); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	project, err = s.store.GetProject(ctx, &store.FindProjectMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &patch.ResourceID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(convertToProject(project)), nil
}

// DeleteProject deletes a project.
func (s *ProjectService) DeleteProject(ctx context.Context, req *connect.Request[v1pb.DeleteProjectRequest]) (*connect.Response[emptypb.Empty], error) {
	project, err := s.getProjectMessage(ctx, req.Msg.Name)
	if err != nil {
		return nil, err
	}

	// Handle purge (hard delete)
	if req.Msg.Purge {
		if common.IsDefaultProject(project.Workspace, project.ResourceID) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("default project cannot be purged"))
		}

		// If project is not already soft-deleted, soft-delete it first
		if !project.Deleted {
			if err := s.store.UpdateProjects(ctx, &store.UpdateProjectMessage{
				ResourceID: project.ResourceID,
				Workspace:  project.Workspace,
				Delete:     &deletePatch,
			}); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		// Permanently delete the project and all related resources (moves databases to default project)
		if err := s.store.DeleteProject(ctx, project.Workspace, project.ResourceID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to purge project"))
		}

		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Regular soft delete (archive) flow
	if common.IsDefaultProject(project.Workspace, project.ResourceID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("default project cannot be deleted"))
	}
	// Idempotent: if already deleted, return success
	if project.Deleted {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// For archive (soft delete), just mark the project as deleted without touching databases or issues
	if err := s.store.UpdateProjects(ctx, &store.UpdateProjectMessage{
		ResourceID: project.ResourceID,
		Workspace:  project.Workspace,
		Delete:     &deletePatch,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// UndeleteProject undeletes a project.
func (s *ProjectService) UndeleteProject(ctx context.Context, req *connect.Request[v1pb.UndeleteProjectRequest]) (*connect.Response[v1pb.Project], error) {
	project, err := s.getProjectMessage(ctx, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	// Idempotent: if already active, return the project
	if !project.Deleted {
		return connect.NewResponse(convertToProject(project)), nil
	}

	if err := s.store.UpdateProjects(ctx, &store.UpdateProjectMessage{
		ResourceID: project.ResourceID,
		Workspace:  project.Workspace,
		Delete:     &undeletePatch,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	project, err = s.store.GetProject(ctx, &store.FindProjectMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &project.ResourceID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(convertToProject(project)), nil
}

// BatchDeleteProjects deletes multiple projects in batch.
func (s *ProjectService) BatchDeleteProjects(ctx context.Context, request *connect.Request[v1pb.BatchDeleteProjectsRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(request.Msg.Names) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("names cannot be empty"))
	}

	// Handle purge (hard delete)
	if request.Msg.Purge {
		var projectsToPurge []*store.ProjectMessage
		for _, name := range request.Msg.Names {
			project, err := s.getProjectMessage(ctx, name)
			if err != nil {
				return nil, err
			}
			if common.IsDefaultProject(project.Workspace, project.ResourceID) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("default project cannot be purged"))
			}
			projectsToPurge = append(projectsToPurge, project)
		}

		// Soft-delete projects that aren't already deleted
		var projectsToSoftDelete []*store.UpdateProjectMessage
		for _, project := range projectsToPurge {
			if !project.Deleted {
				projectsToSoftDelete = append(projectsToSoftDelete, &store.UpdateProjectMessage{
					ResourceID: project.ResourceID,
					Workspace:  project.Workspace,
					Delete:     &deletePatch,
				})
			}
		}
		if len(projectsToSoftDelete) > 0 {
			if err := s.store.UpdateProjects(ctx, projectsToSoftDelete...); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		// Permanently delete all projects (moves databases to default project)
		for _, project := range projectsToPurge {
			if err := s.store.DeleteProject(ctx, project.Workspace, project.ResourceID); err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to purge project %q", project.Title))
			}
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Regular soft delete (archive) flow
	// Phase 1: Load all projects and check permissions
	var projects []*store.ProjectMessage
	for _, name := range request.Msg.Names {
		project, err := s.getProjectMessage(ctx, name)
		if err != nil {
			return nil, err
		}
		if common.IsDefaultProject(project.Workspace, project.ResourceID) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("default project cannot be deleted"))
		}
		// Idempotent: skip already deleted projects
		if project.Deleted {
			continue
		}
		projects = append(projects, project)
	}

	// Phase 2: Mark all projects as deleted (soft delete/archive)
	// No need to check for databases or issues - they remain in the archived project
	var updatePatches []*store.UpdateProjectMessage
	for _, project := range projects {
		updatePatches = append(updatePatches, &store.UpdateProjectMessage{
			ResourceID: project.ResourceID,
			Workspace:  project.Workspace,
			Delete:     &deletePatch,
		})
	}

	if len(updatePatches) > 0 {
		if err := s.store.UpdateProjects(ctx, updatePatches...); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}


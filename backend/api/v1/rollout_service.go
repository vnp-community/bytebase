package v1

import (
	"context"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/webhook"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
)

// RolloutService represents a service for managing rollout.
type RolloutService struct {
	v1connect.UnimplementedRolloutServiceHandler
	store          *store.Store
	dbFactory      *dbfactory.DBFactory
	bus            bus.EventBus
	webhookManager *webhook.Manager
	iamManager     *iam.Manager
}

// NewRolloutService returns a rollout service instance.
func NewRolloutService(store *store.Store, dbFactory *dbfactory.DBFactory, bus bus.EventBus, webhookManager *webhook.Manager, iamManager *iam.Manager) *RolloutService {
	return &RolloutService{
		store:          store,
		dbFactory:      dbFactory,
		bus:            bus,
		webhookManager: webhookManager,
		iamManager:     iamManager,
	}
}

// GetRollout gets a rollout.
func (s *RolloutService) GetRollout(ctx context.Context, req *connect.Request[v1pb.GetRolloutRequest]) (*connect.Response[v1pb.Rollout], error) {
	request := req.Msg
	projectID, planID, err := common.GetProjectIDPlanIDFromRolloutName(request.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get project"))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %q not found", projectID))
	}

	// getRolloutWithTasks inlined
	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout %d not found in project %s", planID, projectID))
	}
	// Check if the plan has a rollout
	if plan.Config == nil || !plan.Config.HasRollout {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout %d not found in project %s", planID, projectID))
	}

	tasks, err := s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get tasks"))
	}

	// Get environment order.
	environments, err := s.store.GetEnvironment(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get environments"))
	}
	environmentOrderMap := make(map[string]int)
	for i, env := range environments.GetEnvironments() {
		environmentOrderMap[env.Id] = i
	}

	rolloutV1, err := convertToRollout(project, plan, tasks, environmentOrderMap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to rollout"))
	}
	return connect.NewResponse(rolloutV1), nil
}

// ListRollouts lists rollouts.
func (s *RolloutService) ListRollouts(ctx context.Context, req *connect.Request[v1pb.ListRolloutsRequest]) (*connect.Response[v1pb.ListRolloutsResponse], error) {
	request := req.Msg
	projectID, err := common.GetProjectID(request.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get project"))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %q not found", projectID))
	}

	offset, err := parseLimitAndOffset(&pageSize{
		token:   request.PageToken,
		limit:   int(request.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	// Filter plans to only those with rollouts (tasks).
	hasRollout := true
	findPlan := &store.FindPlanMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ProjectID:  projectID,
		Limit:      &limitPlusOne,
		Offset:     &offset.offset,
		HasRollout: &hasRollout,
	}
	if err := buildRolloutFindWithFilter(findPlan, request.Filter); err != nil {
		return nil, err
	}
	plans, err := s.store.ListPlans(ctx, findPlan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list plans"))
	}

	var nextPageToken string
	if len(plans) == limitPlusOne {
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get next page token"))
		}
		plans = plans[:offset.limit]
	}

	// Batch load all tasks for all plans to avoid N+1 queries
	planIDs := make([]int64, len(plans))
	for i, plan := range plans {
		planIDs[i] = plan.UID
	}
	var allTasks []*store.TaskMessage
	if len(planIDs) > 0 {
		var err error
		allTasks, err = s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanIDs: &planIDs})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list tasks"))
		}
	}

	// Group tasks by plan ID
	tasksByPlanID := make(map[int64][]*store.TaskMessage)
	for _, task := range allTasks {
		tasksByPlanID[task.PlanID] = append(tasksByPlanID[task.PlanID], task)
	}

	// Get environment order once for all rollouts.
	environments, err := s.store.GetEnvironment(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get environments"))
	}
	environmentOrderMap := make(map[string]int)
	for i, env := range environments.GetEnvironments() {
		environmentOrderMap[env.Id] = i
	}

	// Convert plans and their tasks to rollouts
	rollouts := []*v1pb.Rollout{}
	for _, plan := range plans {
		tasks := tasksByPlanID[plan.UID]
		rollout, err := convertToRollout(project, plan, tasks, environmentOrderMap)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to rollout"))
		}
		rollouts = append(rollouts, rollout)
	}

	return connect.NewResponse(&v1pb.ListRolloutsResponse{
		Rollouts:      rollouts,
		NextPageToken: nextPageToken,
	}), nil
}

// buildRolloutFindWithFilter builds the filter for rollout find.
func buildRolloutFindWithFilter(planFind *store.FindPlanMessage, filter string) error {
	if filter == "" {
		return nil
	}

	filterQ, err := store.GetListRolloutFilter(filter)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	planFind.FilterQ = filterQ
	return nil
}

// CreateRollout creates a rollout from plan.
func (s *RolloutService) CreateRollout(ctx context.Context, req *connect.Request[v1pb.CreateRolloutRequest]) (*connect.Response[v1pb.Rollout], error) {
	request := req.Msg
	projectID, planID, err := common.GetProjectIDPlanID(request.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get project"))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project not found for id: %v", projectID))
	}

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, UID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan %d not found in project %s", planID, projectID))
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	// Fetch issue associated with this plan (if any)
	issue, err := s.store.GetIssue(ctx, &store.FindIssueMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ProjectIDs: []string{projectID},
		PlanUID:    &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find issue"))
	}

	// Check permission: allow if user has bb.rollouts.create permission
	hasPermission, err := s.iamManager.CheckPermission(ctx, permission.RolloutsCreate, user, common.GetWorkspaceIDFromContext(ctx), project.ResourceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check permission"))
	}

	if !hasPermission {
		// Allow data export issue creators to create rollout for their own issues
		if issue == nil || issue.Type != storepb.Issue_DATABASE_EXPORT || issue.CreatorEmail != user.Email {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied to create rollout"))
		}
	}

	tasks, err := GetPipelineCreate(ctx, s.store, plan.Config.GetSpecs(), project.ResourceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to get pipeline create"))
	}
	if isChangeDatabasePlan(plan.Config.GetSpecs()) {
		tasks, err = filterTasksByStage(tasks, request.Target)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to filter tasks with stage id"))
		}
	}

	if err := CreateRolloutAndPendingTasks(ctx, s.store, user.Email, plan, issue, project, tasks); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tasks, err = s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get tasks"))
	}

	// Get environment order.
	environments, err := s.store.GetEnvironment(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get environments"))
	}
	environmentOrderMap := make(map[string]int)
	for i, env := range environments.GetEnvironments() {
		environmentOrderMap[env.Id] = i
	}

	rolloutV1, err := convertToRollout(project, plan, tasks, environmentOrderMap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to rollout"))
	}

	// Tickle task run scheduler.
	s.bus.TickleTaskRun()

	return connect.NewResponse(rolloutV1), nil
}

// CreateRolloutAndPendingTasks creates rollout tasks and pending task runs.
func CreateRolloutAndPendingTasks(
	ctx context.Context,
	s *store.Store,
	userEmail string,
	plan *store.PlanMessage,
	issue *store.IssueMessage,
	project *store.ProjectMessage,
	tasks []*store.TaskMessage,
) error {
	var err error
	if tasks == nil {
		tasks, err = GetPipelineCreate(ctx, s, plan.Config.GetSpecs(), project.ResourceID)
		if err != nil {
			return errors.Wrap(err, "failed to get pipeline create for rollout creation")
		}
	}

	// Create rollout tasks
	tasks, err = s.CreateTasks(ctx, project.ResourceID, plan.UID, tasks)
	if err != nil {
		return errors.Wrap(err, "failed to create rollout tasks")
	}

	// Update plan to set hasRollout to true
	config := proto.CloneOf(plan.Config)
	config.HasRollout = true
	_, err = s.UpdatePlan(ctx, &store.UpdatePlanMessage{
		UID:       plan.UID,
		ProjectID: project.ResourceID,
		Config:    config,
	})
	if err != nil {
		return errors.Wrap(err, "failed to update plan hasRollout")
	}

	// Update issue status to DONE when rollout is created
	if issue != nil {
		newStatus := storepb.Issue_DONE
		updatedIssue, err := s.UpdateIssue(ctx, issue.ProjectID, issue.UID, &store.UpdateIssueMessage{Status: &newStatus})
		if err != nil {
			return errors.Wrapf(err, "failed to update issue %q's status", issue.Title)
		}

		if _, err := s.CreateIssueComments(ctx, userEmail, &store.IssueCommentMessage{
			ProjectID: issue.ProjectID,
			IssueUID:  issue.UID,
			Payload: &storepb.IssueCommentPayload{
				Event: &storepb.IssueCommentPayload_IssueUpdate_{
					IssueUpdate: &storepb.IssueCommentPayload_IssueUpdate{
						FromStatus: &issue.Status,
						ToStatus:   &updatedIssue.Status,
					},
				},
			},
		}); err != nil {
			return errors.Wrapf(err, "failed to create issue comment after changing the issue status")
		}
	}

	// Check if we should auto-rollout
	envPolicies := make(map[string]*storepb.RolloutPolicy)
	for _, task := range tasks {
		if task.Environment == "" {
			continue
		}

		policy, ok := envPolicies[task.Environment]
		if !ok {
			var err error
			policy, err = s.GetRolloutPolicy(ctx, project.Workspace, task.Environment)
			if err != nil {
				return errors.Wrapf(err, "failed to get rollout policy for environment %s", task.Environment)
			}
			envPolicies[task.Environment] = policy
		}

		if policy.Automatic {
			create := &store.TaskRunMessage{
				TaskUID:   task.ID,
				ProjectID: task.ProjectID,
			}

			// System-generated task run for auto-rollout
			if err := s.CreatePendingTaskRuns(ctx, "", create); err != nil {
				return errors.Wrapf(err, "failed to create pending task runs for task %d", task.ID)
			}
		}
	}
	return nil
}

func isChangeDatabasePlan(specs []*storepb.PlanConfig_Spec) bool {
	for _, spec := range specs {
		if spec.GetChangeDatabaseConfig() != nil {
			return true
		}
	}
	return false
}

// GetPipelineCreate gets a pipeline create message from a plan.
func GetPipelineCreate(ctx context.Context, s *store.Store, specs []*storepb.PlanConfig_Spec, projectID string) ([]*store.TaskMessage, error) {
	// Step 1 - transform database group specs.
	transformedSpecs, err := applyDatabaseGroupSpecTransformations(ctx, s, specs, projectID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to apply database group spec transformations")
	}

	// Step 2 - convert all task creates.
	var taskCreates []*store.TaskMessage
	for _, spec := range transformedSpecs {
		tcs, err := getTaskCreatesFromSpec(ctx, s, spec)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get task creates from spec")
		}
		taskCreates = append(taskCreates, tcs...)
	}

	return taskCreates, nil
}

// filterTasksByStage filters tasks using targetEnvironmentID.
func filterTasksByStage(tasks []*store.TaskMessage, targetEnvironment *string) ([]*store.TaskMessage, error) {
	if targetEnvironment == nil {
		return tasks, nil
	}
	if *targetEnvironment == "" {
		return nil, nil
	}
	targetEnvironmentID, err := common.GetEnvironmentID(*targetEnvironment)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get environment id from %q", *targetEnvironment)
	}

	// Filter tasks to only include those in allowed environments
	filteredTasks := []*store.TaskMessage{}
	for _, task := range tasks {
		if task.Environment == targetEnvironmentID {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks, nil
}

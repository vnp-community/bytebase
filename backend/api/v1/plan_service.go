package v1

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/runner/approval"

	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
)

// PlanService represents a service for managing plan.
type PlanService struct {
	v1connect.UnimplementedPlanServiceHandler
	store          *store.Store
	bus            bus.EventBus
	iamManager     *iam.Manager
	webhookManager *webhook.Manager
	licenseService *enterprise.LicenseService
}

// NewPlanService returns a plan service instance.
func NewPlanService(store *store.Store, bus bus.EventBus, iamManager *iam.Manager, webhookManager *webhook.Manager, licenseService *enterprise.LicenseService) *PlanService {
	return &PlanService{
		store:          store,
		bus:            bus,
		iamManager:     iamManager,
		webhookManager: webhookManager,
		licenseService: licenseService,
	}
}

// GetPlan gets a plan.
func (s *PlanService) GetPlan(ctx context.Context, request *connect.Request[v1pb.GetPlanRequest]) (*connect.Response[v1pb.Plan], error) {
	req := request.Msg
	projectID, planID, err := common.GetProjectIDPlanID(req.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		UID:       &planID,
		ProjectID: projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan %d not found in project %s", planID, projectID))
	}
	convertedPlan, err := convertToPlan(ctx, s.store, plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to plan"))
	}
	return connect.NewResponse(convertedPlan), nil
}

// ListPlans lists plans.
func (s *PlanService) ListPlans(ctx context.Context, request *connect.Request[v1pb.ListPlansRequest]) (*connect.Response[v1pb.ListPlansResponse], error) {
	req := request.Msg
	projectID, err := common.GetProjectID(req.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	offset, err := parseLimitAndOffset(&pageSize{
		token:   req.PageToken,
		limit:   int(req.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	find := &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		Limit:     &limitPlusOne,
		Offset:    &offset.offset,
		ProjectID: projectID,
	}

	if req.Filter != "" {
		filterQ, err := store.GetListPlanFilter(req.Filter)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		find.FilterQ = filterQ
	}

	plans, err := s.store.ListPlans(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list plans"))
	}

	var nextPageToken string
	// has more pages
	if len(plans) == limitPlusOne {
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get next page token"))
		}
		plans = plans[:offset.limit]
	}

	convertedPlans, err := convertToPlans(ctx, s.store, plans)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to plans"))
	}

	return connect.NewResponse(&v1pb.ListPlansResponse{
		Plans:         convertedPlans,
		NextPageToken: nextPageToken,
	}), nil
}

func getProjectIDsSearchFilter(ctx context.Context, user *store.UserMessage, permission permission.Permission, iamManager *iam.Manager, stores *store.Store) (*[]string, error) {
	workspaceID := common.GetWorkspaceIDFromContext(ctx)
	projects, err := stores.ListProjects(ctx, &store.FindProjectMessage{Workspace: workspaceID})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list projects")
	}

	ok, err := iamManager.CheckPermission(ctx, permission, user, workspaceID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check permission %q", permission)
	}
	if ok {
		projectIDs := make([]string, 0, len(projects))
		for _, project := range projects {
			projectIDs = append(projectIDs, project.ResourceID)
		}
		return &projectIDs, nil
	}

	var projectIDs []string
	for _, project := range projects {
		ok, err := iamManager.CheckPermission(ctx, permission, user, workspaceID, project.ResourceID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check permission %q", permission)
		}
		if ok {
			projectIDs = append(projectIDs, project.ResourceID)
		}
	}
	return &projectIDs, nil
}

// CreatePlan creates a new plan.
func (s *PlanService) CreatePlan(ctx context.Context, request *connect.Request[v1pb.CreatePlanRequest]) (*connect.Response[v1pb.Plan], error) {
	req := request.Msg
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}
	projectID, err := common.GetProjectID(req.Parent)
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

	trimmedTitle := strings.TrimSpace(req.Plan.Title)
	if project.Setting.EnforceIssueTitle && trimmedTitle == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("project %q requires a manual plan title (enforce_issue_title is enabled)", req.Parent))
	}

	// Validate plan specs
	databaseGroup, err := validateSpecs(ctx, s.store, projectID, req.Plan.Specs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to validate plan specs"))
	}

	planMessage := &store.PlanMessage{
		ProjectID:   projectID,
		Name:        trimmedTitle,
		Description: req.Plan.Description,
		Config:      convertPlan(req.Plan),
	}

	plan, err := s.store.CreatePlan(ctx, planMessage, user.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create plan"))
	}

	planCheckRun, err := getPlanCheckRunFromPlan(ctx, s.store, project, plan, databaseGroup)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan check run for plan"))
	}
	if planCheckRun != nil {
		if err := s.store.CreatePlanCheckRun(ctx, planCheckRun); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create plan check run"))
		}
	}
	// Tickle plan check scheduler.
	s.bus.TicklePlanCheck()

	convertedPlan, err := convertToPlan(ctx, s.store, plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to plan"))
	}
	return connect.NewResponse(convertedPlan), nil
}

// UpdatePlan updates a plan.
func (s *PlanService) UpdatePlan(ctx context.Context, request *connect.Request[v1pb.UpdatePlanRequest]) (*connect.Response[v1pb.Plan], error) {
	req := request.Msg
	if req.UpdateMask == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("update_mask must be set"))
	}
	if len(req.UpdateMask.Paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("update_mask must not be empty"))
	}
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}
	projectID, planID, err := common.GetProjectIDPlanID(req.Plan.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &projectID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get project %q, err: %v", projectID, err))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %q not found", projectID))
	}
	oldPlan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, UID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get plan %q: %v", req.Plan.Name, err))
	}
	if oldPlan == nil {
		if req.AllowMissing {
			ok, err := s.iamManager.CheckPermission(ctx, permission.PlansCreate, user, common.GetWorkspaceIDFromContext(ctx), projectID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to check permission"))
			}
			if !ok {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("user does not have permission %q", permission.PlansCreate))
			}
			return s.CreatePlan(ctx, connect.NewRequest(&v1pb.CreatePlanRequest{
				Parent: common.FormatProject(projectID),
				Plan:   req.Plan,
			}))
		}
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan %q not found", req.Plan.Name))
	}

	if storePlanConfigHasRelease(oldPlan.Config) && slices.Contains(req.UpdateMask.Paths, "specs") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("disallowed to update the plan specs because the plan is created from a release"))
	}

	// Disallow updating CREATE_DATABASE plan specs
	if storePlanConfigHasCreateDatabase(oldPlan.Config) && slices.Contains(req.UpdateMask.Paths, "specs") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("disallowed to update the plan specs for CREATE_DATABASE plans"))
	}

	ok, err = func() (bool, error) {
		if oldPlan.Creator == user.Email {
			return true, nil
		}
		return s.iamManager.CheckPermission(ctx, permission.PlansUpdate, user, common.GetWorkspaceIDFromContext(ctx), oldPlan.ProjectID)
	}()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check permission"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("permission denied to update plan"))
	}

	planUpdate := &store.UpdatePlanMessage{
		UID:       oldPlan.UID,
		ProjectID: oldPlan.ProjectID,
	}

	var planCheckRunsTrigger bool
	var databaseGroup *v1pb.DatabaseGroup

	for _, path := range req.UpdateMask.Paths {
		switch path {
		case "title":
			trimmed := strings.TrimSpace(req.Plan.Title)
			if project.Setting.EnforceIssueTitle && trimmed == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("project %q requires a manual plan title (enforce_issue_title is enabled)", common.FormatProject(project.ResourceID)))
			}
			planUpdate.Name = &trimmed
		case "description":
			planUpdate.Description = new(req.Plan.Description)
		case "state":
			planUpdate.Deleted = new(req.Plan.State == v1pb.State_DELETED)
		case "specs":
			// Block all spec changes if plan has a rollout (pipeline).
			// Block all spec changes if plan has a rollout (pipeline).
			if oldPlan.Config != nil && oldPlan.Config.GetHasRollout() {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("cannot update specs for plan that has a rollout"))
			}

			// Validate the new specs.
			dg, err := validateSpecs(ctx, s.store, oldPlan.ProjectID, req.Plan.Specs)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to validate plan specs"))
			}
			databaseGroup = dg

			// Convert and store new specs.
			allSpecs := convertPlanSpecs(req.GetPlan().GetSpecs())
			config := proto.CloneOf(oldPlan.Config)
			config.Specs = allSpecs
			planUpdate.Config = config

			// Trigger plan check runs.
			planCheckRunsTrigger = true

			// Evict approvals if issue exists to request re-approval.
			issue, err := s.store.GetIssue(ctx, &store.FindIssueMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectIDs: []string{projectID}, PlanUID: &oldPlan.UID})
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get issue: %v", err))
			}
			if issue != nil {
				// Reset approval finding status
				updatedIssue, err := s.store.UpdateIssue(ctx, issue.ProjectID, issue.UID, &store.UpdateIssueMessage{
					PayloadUpsert: &storepb.Issue{
						Approval: &storepb.IssuePayloadApproval{
							ApprovalFindingDone: false,
						},
					},
				})
				if err != nil {
					slog.Error("failed to reset approval finding status after plan update", log.BBError(err))
				}

				// DATABASE_CHANGE: Don't trigger ApprovalCheckChan here - plan update creates new plan check run,
				// which will trigger approval finding on completion
				// DATABASE_EXPORT: Re-run approval finding synchronously (no plan checks for export data)
				if updatedIssue.Type == storepb.Issue_DATABASE_EXPORT {
					if err := approval.FindAndApplyApprovalTemplate(ctx, s.store, s.webhookManager, s.licenseService, updatedIssue); err != nil {
						slog.Error("failed to find approval template after plan update",
							slog.String("project", updatedIssue.ProjectID), slog.Int64("issue_uid", updatedIssue.UID),
							slog.String("issue_title", updatedIssue.Title),
							log.BBError(err))
						// Continue anyway - non-fatal error
					}
				}
			}
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid update_mask path %q", path))
		}
	}

	updatedPlan, err := s.store.UpdatePlan(ctx, planUpdate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to update plan %q: %v", req.Plan.Name, err))
	}

	if planCheckRunsTrigger {
		planCheckRun, err := getPlanCheckRunFromPlan(ctx, s.store, project, updatedPlan, databaseGroup)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan check run for plan"))
		}
		if planCheckRun != nil {
			if err := s.store.CreatePlanCheckRun(ctx, planCheckRun); err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create plan check run"))
			}
		}
		// Tickle plan check scheduler.
		s.bus.TicklePlanCheck()
	}

	convertedPlan, err := convertToPlan(ctx, s.store, updatedPlan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to plan"))
	}
	return connect.NewResponse(convertedPlan), nil
}

// GetPlanCheckRun gets the plan check run for the plan.
func (s *PlanService) GetPlanCheckRun(ctx context.Context, request *connect.Request[v1pb.GetPlanCheckRunRequest]) (*connect.Response[v1pb.PlanCheckRun], error) {
	req := request.Msg
	projectID, planUID, err := common.GetProjectIDPlanIDFromPlanCheckRun(req.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	planCheckRun, err := s.store.GetPlanCheckRun(ctx, projectID, planUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan check run"))
	}
	if planCheckRun == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan check run not found for plan %d", planUID))
	}

	converted := convertToPlanCheckRun(projectID, planUID, planCheckRun)
	return connect.NewResponse(converted), nil
}

// RunPlanChecks runs plan checks for a plan.
func (s *PlanService) RunPlanChecks(ctx context.Context, request *connect.Request[v1pb.RunPlanChecksRequest]) (*connect.Response[v1pb.RunPlanChecksResponse], error) {
	req := request.Msg
	projectID, planID, err := common.GetProjectIDPlanID(req.Name)
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
	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		UID:       &planID,
		ProjectID: projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan not found"))
	}
	if storePlanConfigHasRelease(plan.Config) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("cannot run plan checks because plan %q has release", plan.Name))
	}
	// Once a rollout exists the plan is frozen; re-running checks produces the
	// same result and is misleading. Match the frontend gate in
	// PlanCheckSection.vue / ChecksSection.vue / IssueDetailChecks.tsx.
	if plan.Config.GetHasRollout() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.Errorf("cannot run plan checks because plan %q already has a rollout", plan.Name))
	}
	var databaseGroup *v1pb.DatabaseGroup
	for _, spec := range plan.Config.GetSpecs() {
		if c, ok := spec.Config.(*storepb.PlanConfig_Spec_ChangeDatabaseConfig); ok {
			if len(c.ChangeDatabaseConfig.Targets) == 1 {
				if _, _, err := common.GetProjectIDDatabaseGroupID(c.ChangeDatabaseConfig.Targets[0]); err == nil {
					dg, err := getDatabaseGroupByName(ctx, s.store, c.ChangeDatabaseConfig.Targets[0], v1pb.DatabaseGroupView_DATABASE_GROUP_VIEW_FULL)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database group %q: %v", c.ChangeDatabaseConfig.Targets[0], err))
					}
					if dg == nil {
						return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("database group %q not found", c.ChangeDatabaseConfig.Targets[0]))
					}
					databaseGroup = dg
					break
				}
			}
		}
	}
	planCheckRun, err := getPlanCheckRunFromPlan(ctx, s.store, project, plan, databaseGroup)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan check run for plan"))
	}
	if planCheckRun != nil {
		if err := s.store.CreatePlanCheckRun(ctx, planCheckRun); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create plan check run"))
		}
	}

	// Tickle plan check scheduler.
	s.bus.TicklePlanCheck()

	return connect.NewResponse(&v1pb.RunPlanChecksResponse{}), nil
}

// CancelPlanCheckRun cancels the plan check run for a plan.
func (s *PlanService) CancelPlanCheckRun(ctx context.Context, request *connect.Request[v1pb.CancelPlanCheckRunRequest]) (*connect.Response[v1pb.CancelPlanCheckRunResponse], error) {
	req := request.Msg
	projectID, planUID, err := common.GetProjectIDPlanIDFromPlanCheckRun(req.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find project"))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %v not found", projectID))
	}

	planCheckRun, err := s.store.GetPlanCheckRun(ctx, projectID, planUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan check run"))
	}
	if planCheckRun == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan check run not found for plan %d", planUID))
	}

	if planCheckRun.Status != store.PlanCheckRunStatusRunning && planCheckRun.Status != store.PlanCheckRunStatusAvailable {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("plan check run is not running or available"))
	}

	// Cancel in-flight plan check run if running.
	s.bus.CancelPlanCheck(bus.PlanCheckRunRef{ProjectID: projectID, UID: planCheckRun.UID})

	// Broadcast cancel signal to all replicas for HA.
	if err := s.store.SendSignal(ctx, storepb.Signal_CANCEL_PLAN_CHECK_RUN, projectID, planCheckRun.UID); err != nil {
		slog.Warn("failed to send cancel signal", log.BBError(err))
	}

	// Update the status to canceled.
	if err := s.store.BatchCancelPlanCheckRuns(ctx, projectID, []int64{planCheckRun.UID}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to cancel plan check run"))
	}

	return connect.NewResponse(&v1pb.CancelPlanCheckRunResponse{}), nil
}


package v1

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/bus"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	parserbase "github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/utils"
)

// BatchRunTasks runs tasks in batch.
func (s *RolloutService) BatchRunTasks(ctx context.Context, req *connect.Request[v1pb.BatchRunTasksRequest]) (*connect.Response[v1pb.BatchRunTasksResponse], error) {
	request := req.Msg
	if len(request.Tasks) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("The tasks in request cannot be empty"))
	}
	projectID, planID, _, err := common.GetProjectIDPlanIDMaybeStageID(request.Parent)
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

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find plan for rollout"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout (plan) %v not found", planID))
	}

	// Reset notification state so user gets fresh feedback on retry
	if err := s.store.ResetPlanWebhookDelivery(ctx, projectID, planID); err != nil {
		slog.Error("failed to reset plan webhook delivery", log.BBError(err))
		// Don't fail the request - notification is non-critical
	}

	issueN, err := s.store.GetIssue(ctx, &store.FindIssueMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ProjectIDs: []string{projectID},
		PlanUID:    &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find issue"))
	}

	// Parse requested task IDs and group by their environment
	taskEnvironments := map[string][]int64{}
	taskIDsToRunMap := map[int64]bool{}
	for _, task := range request.Tasks {
		_, _, stageID, taskID, err := common.GetProjectIDPlanIDStageIDTaskID(task)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		environment := formatEnvironmentFromStageID(stageID)
		taskEnvironments[environment] = append(taskEnvironments[environment], taskID)
		taskIDsToRunMap[taskID] = true
	}
	if len(taskEnvironments) > 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tasks should be in the same environment"))
	}

	// Get the environment for the tasks to run
	var environmentToRun string
	for env := range taskEnvironments {
		environmentToRun = env
		break
	}

	// Get all tasks in the same environment
	stageToRunTasks, err := s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanID: &planID, Environment: &environmentToRun})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list tasks"))
	}
	if len(stageToRunTasks) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("No tasks to run in the stage"))
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	ok, err = s.canUserRunEnvironmentTasks(ctx, user, project, issueN, environmentToRun, plan.Creator)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check if the user can run tasks"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("Not allowed to run tasks"))
	}

	// Check if issue approval is required according to the project settings
	if project.Setting.RequireIssueApproval && issueN != nil {
		approved, err := utils.CheckIssueApproved(issueN)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check if the issue is approved"))
		}
		if !approved {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot run the tasks because issue approval is required but the issue is not approved"))
		}
	}

	var taskRunCreates []*store.TaskRunMessage
	for _, task := range stageToRunTasks {
		if !taskIDsToRunMap[task.ID] {
			continue
		}

		create := &store.TaskRunMessage{
			TaskUID:   task.ID,
			ProjectID: projectID,
		}
		if request.GetRunTime() != nil {
			t := request.GetRunTime().AsTime()
			create.RunAt = &t
		}
		if request.GetSkipPriorBackup() {
			create.PayloadProto = &storepb.TaskRunPayload{
				SkipPriorBackup: true,
			}
		}
		taskRunCreates = append(taskRunCreates, create)
	}
	slices.SortFunc(taskRunCreates, func(a, b *store.TaskRunMessage) int {
		switch {
		case a.TaskUID < b.TaskUID:
			return -1
		case a.TaskUID > b.TaskUID:
			return 1
		default:
			return 0
		}
	})

	if err := s.store.CreatePendingTaskRuns(ctx, user.Email, taskRunCreates...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to create pending task runs, error %v", err))
	}

	// Tickle task run scheduler.
	s.bus.TickleTaskRun()

	return connect.NewResponse(&v1pb.BatchRunTasksResponse{}), nil
}

// BatchSkipTasks skips tasks in batch.
func (s *RolloutService) BatchSkipTasks(ctx context.Context, req *connect.Request[v1pb.BatchSkipTasksRequest]) (*connect.Response[v1pb.BatchSkipTasksResponse], error) {
	request := req.Msg
	projectID, planID, _, err := common.GetProjectIDPlanIDMaybeStageID(request.Parent)
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

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find plan for rollout"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout (plan) %v not found", planID))
	}

	issueN, err := s.store.GetIssue(ctx, &store.FindIssueMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ProjectIDs: []string{projectID},
		PlanUID:    &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find issue"))
	}

	tasks, err := s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list tasks"))
	}

	taskByID := make(map[int64]*store.TaskMessage)
	for _, task := range tasks {
		taskByID[task.ID] = task
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}
	var taskUIDs []int64
	environmentSet := map[string]struct{}{}
	for _, task := range request.Tasks {
		_, _, _, taskID, err := common.GetProjectIDPlanIDStageIDTaskID(task)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		taskMsg, ok := taskByID[taskID]
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("task %v not found in the rollout", taskID))
		}
		taskUIDs = append(taskUIDs, taskID)
		environmentSet[taskMsg.Environment] = struct{}{}
	}

	for environment := range environmentSet {
		ok, err = s.canUserRunEnvironmentTasks(ctx, user, project, issueN, environment, plan.Creator)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check if the user can skip tasks"))
		}
		if !ok {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("not allowed to skip tasks in environment %q", environment))
		}
	}

	if err := s.store.BatchSkipTasks(ctx, projectID, taskUIDs, request.Reason); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to skip tasks"))
	}

	// Signal to check if plan is complete and successful (may send PIPELINE_COMPLETED)
	s.bus.RequestPlanCompletionCheck(bus.PlanRef{ProjectID: projectID, PlanID: planID})

	return connect.NewResponse(&v1pb.BatchSkipTasksResponse{}), nil
}

// BatchCancelTaskRuns cancels a list of task runs.
func (s *RolloutService) BatchCancelTaskRuns(ctx context.Context, req *connect.Request[v1pb.BatchCancelTaskRunsRequest]) (*connect.Response[v1pb.BatchCancelTaskRunsResponse], error) {
	request := req.Msg
	if len(request.TaskRuns) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task runs cannot be empty"))
	}

	projectID, planID, stageID, _, err := common.GetProjectIDPlanIDStageIDMaybeTaskID(request.Parent)
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

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find plan for rollout"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout (plan) %v not found", planID))
	}

	issueN, err := s.store.GetIssue(ctx, &store.FindIssueMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ProjectIDs: []string{projectID},
		PlanUID:    &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find issue"))
	}

	for _, taskRun := range request.TaskRuns {
		_, _, taskRunStageID, _, _, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(taskRun)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if taskRunStageID != stageID {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task run %v is not in the specified stage %v", taskRun, stageID))
		}
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	environment := formatEnvironmentFromStageID(stageID)
	ok, err = s.canUserCancelEnvironmentTaskRun(ctx, user, project, issueN, environment, plan.Creator)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check if the user can run tasks"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("Not allowed to cancel tasks"))
	}

	var taskRunIDs []int64
	for _, taskRun := range request.TaskRuns {
		_, _, _, _, taskRunID, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(taskRun)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		taskRunIDs = append(taskRunIDs, taskRunID)
	}

	taskRuns, err := s.store.ListTaskRuns(ctx, &store.FindTaskRunMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UIDs:      &taskRunIDs,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list task runs"))
	}

	for _, taskRun := range taskRuns {
		switch taskRun.Status {
		case storepb.TaskRun_PENDING:
		case storepb.TaskRun_AVAILABLE:
		case storepb.TaskRun_RUNNING:
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("taskRun %v is not pending, available or running", taskRun.ID))
		}
	}

	for _, taskRun := range taskRuns {
		if taskRun.Status == storepb.TaskRun_RUNNING {
			s.bus.CancelTaskRun(bus.TaskRunRef{ProjectID: projectID, ID: taskRun.ID})
			// Broadcast cancel signal to all replicas for HA.
			if err := s.store.SendSignal(ctx, storepb.Signal_CANCEL_TASK_RUN, projectID, taskRun.ID); err != nil {
				slog.Warn("failed to send cancel signal", log.BBError(err))
			}
		}
	}

	if err := s.store.BatchCancelTaskRuns(ctx, projectID, taskRunIDs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to batch patch task run status to canceled"))
	}

	return connect.NewResponse(&v1pb.BatchCancelTaskRunsResponse{}), nil
}

func (s *RolloutService) PreviewTaskRunRollback(ctx context.Context, req *connect.Request[v1pb.PreviewTaskRunRollbackRequest]) (*connect.Response[v1pb.PreviewTaskRunRollbackResponse], error) {
	request := req.Msg
	projectID, planID, _, taskUID, taskRunUID, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(request.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to get task run uid"))
	}

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan %d not found in project %s", planID, projectID))
	}

	taskRuns, err := s.store.ListTaskRuns(ctx, &store.FindTaskRunMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &taskRunUID,
		PlanUID:   &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list task runs"))
	}
	if len(taskRuns) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("task run %v not found", taskRunUID))
	}
	if len(taskRuns) > 1 {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("found multiple task runs with the same uid %v", taskRunUID))
	}

	taskRun := taskRuns[0]

	if taskRun.Status != storepb.TaskRun_DONE {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task run %v is not done", taskRun.ID))
	}

	if taskRun.ResultProto == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task run %v has no result", taskRun.ID))
	}

	if !taskRun.ResultProto.HasPriorBackup {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task run %v has no rollback", taskRun.ID))
	}

	// Get backup detail from task run logs.
	logs, err := s.store.ListTaskRunLogs(ctx, projectID, taskRunUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list task run logs"))
	}
	var backupDetail *storepb.PriorBackupDetail
	for _, log := range logs {
		if log.Payload.Type == storepb.TaskRunLog_PRIOR_BACKUP_END && log.Payload.PriorBackupEnd != nil {
			backupDetail = log.Payload.PriorBackupEnd.PriorBackupDetail
			break
		}
	}
	if backupDetail == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task run %v has no backup detail in logs", taskRun.ID))
	}

	tasks, err := s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, ID: &taskUID, PlanID: &planID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get task"))
	}
	if len(tasks) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("task %d not found in plan %d", taskUID, planID))
	}
	task := tasks[0]

	instance, err := s.store.GetInstance(ctx, &store.FindInstanceMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &task.InstanceID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get instance"))
	}

	sheetSha256 := task.Payload.GetSheetSha256()
	if sheetSha256 == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("task %v has no sheet", task.ID))
	}
	sheet, err := s.store.GetSheetFull(ctx, sheetSha256)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get sheet statements"))
	}
	statements := sheet.Statement

	var results []string
	for _, item := range backupDetail.Items {
		restore, err := parserbase.GenerateRestoreSQL(ctx, instance.Metadata.GetEngine(), parserbase.RestoreContext{
			InstanceID:              instance.ResourceID,
			GetDatabaseMetadataFunc: BuildGetDatabaseMetadataFunc(s.store),
			ListDatabaseNamesFunc:   BuildListDatabaseNamesFunc(s.store),
			IsCaseSensitive:         store.IsObjectCaseSensitive(instance),
		}, statements, item)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to generate restore sql"))
		}
		results = append(results, restore)
	}

	return connect.NewResponse(&v1pb.PreviewTaskRunRollbackResponse{
		Statement: strings.Join(results, "\n"),
	}), nil
}

// canUserRunEnvironmentTasks returns if a user can run the tasks in an environment.
func (s *RolloutService) canUserRunEnvironmentTasks(ctx context.Context, user *store.UserMessage, project *store.ProjectMessage, issue *store.IssueMessage, environment string, _ string) (bool, error) {
	// For data export issues, only the creator can run tasks.
	if issue != nil && issue.Type == storepb.Issue_DATABASE_EXPORT {
		return issue.CreatorEmail == user.Email, nil
	}

	// Users with bb.taskRuns.create can always create task runs.
	ok, err := s.iamManager.CheckPermission(ctx, permission.TaskRunsCreate, user, common.GetWorkspaceIDFromContext(ctx), project.ResourceID)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check workspace role")
	}
	if ok {
		return true, nil
	}

	p, err := GetValidRolloutPolicyForEnvironment(ctx, s.store, common.GetWorkspaceIDFromContext(ctx), environment)
	if err != nil {
		return false, err
	}

	policy, err := s.store.GetProjectIamPolicy(ctx, common.GetWorkspaceIDFromContext(ctx), project.ResourceID)
	if err != nil {
		return false, common.Wrapf(err, common.Internal, "failed to get project %s IAM policy", project.ResourceID)
	}
	workspacePolicy, err := s.store.GetWorkspaceIamPolicy(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return false, common.Wrapf(err, common.Internal, "failed to get workspace IAM policy")
	}
	roles := utils.GetUserFormattedRolesMap(ctx, s.store, common.GetWorkspaceIDFromContext(ctx), user, policy.Policy)
	workspaceRoles := utils.GetUserFormattedRolesMap(ctx, s.store, common.GetWorkspaceIDFromContext(ctx), user, workspacePolicy.Policy)
	for k := range workspaceRoles {
		roles[k] = true
	}

	for _, role := range p.Roles {
		if roles[role] {
			return true, nil
		}
	}

	return false, nil
}

func (s *RolloutService) canUserCancelEnvironmentTaskRun(ctx context.Context, user *store.UserMessage, project *store.ProjectMessage, issue *store.IssueMessage, environment string, creator string) (bool, error) {
	return s.canUserRunEnvironmentTasks(ctx, user, project, issue, environment, creator)
}

// GetValidRolloutPolicyForEnvironment gets the rollout policy for an environment.
func GetValidRolloutPolicyForEnvironment(ctx context.Context, stores *store.Store, workspaceID string, environment string) (*storepb.RolloutPolicy, error) {
	policy, err := stores.GetRolloutPolicy(ctx, workspaceID, environment)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get rollout policy for environment %s", environment)
	}
	return policy, nil
}

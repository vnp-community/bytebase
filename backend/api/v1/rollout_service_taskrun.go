package v1

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgtype"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bytebase/bytebase/backend/common"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/db"
	"github.com/bytebase/bytebase/backend/store"
)

// ListTaskRuns lists rollout task runs.
func (s *RolloutService) ListTaskRuns(ctx context.Context, req *connect.Request[v1pb.ListTaskRunsRequest]) (*connect.Response[v1pb.ListTaskRunsResponse], error) {
	request := req.Msg
	projectID, planID, maybeStageID, maybeTaskID, err := common.GetProjectIDPlanIDMaybeStageIDMaybeTaskID(request.Parent)
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
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get rollout"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout %d not found in project %s", planID, projectID))
	}
	taskRuns, err := s.store.ListTaskRuns(ctx, &store.FindTaskRunMessage{
		Workspace:   common.GetWorkspaceIDFromContext(ctx),
		ProjectID:   projectID,
		PlanUID:     &planID,
		Environment: maybeStageID,
		TaskUID:     maybeTaskID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list task runs"))
	}

	taskRunsV1, err := convertToTaskRuns(ctx, s.store, taskRuns)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to task runs"))
	}
	return connect.NewResponse(&v1pb.ListTaskRunsResponse{
		TaskRuns: taskRunsV1,
	}), nil
}

// GetTaskRun gets a task run.
func (s *RolloutService) GetTaskRun(ctx context.Context, req *connect.Request[v1pb.GetTaskRunRequest]) (*connect.Response[v1pb.TaskRun], error) {
	request := req.Msg
	projectID, planID, _, _, taskRunUID, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(request.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get rollout"))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout %d not found in project %s", planID, projectID))
	}

	taskRun, err := s.store.GetTaskRunV1(ctx, &store.FindTaskRunMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &taskRunUID,
		PlanUID:   &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get task run"))
	}
	if taskRun == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("task run %d not found in rollout %d", taskRunUID, planID))
	}

	taskRunV1, err := convertToTaskRun(ctx, s.store, taskRun)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to task run"))
	}
	return connect.NewResponse(taskRunV1), nil
}

func (s *RolloutService) GetTaskRunLog(ctx context.Context, req *connect.Request[v1pb.GetTaskRunLogRequest]) (*connect.Response[v1pb.TaskRunLog], error) {
	request := req.Msg
	projectID, planID, _, _, taskRunUID, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(request.Parent)
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

	taskRun, err := s.store.GetTaskRunV1(ctx, &store.FindTaskRunMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &taskRunUID,
		PlanUID:   &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get task run"))
	}
	if taskRun == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("task run %d not found in plan %d", taskRunUID, planID))
	}

	logs, err := s.store.ListTaskRunLogs(ctx, projectID, taskRunUID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to list task run logs"))
	}
	return connect.NewResponse(convertToTaskRunLog(request.Parent, logs)), nil
}

func (s *RolloutService) GetTaskRunSession(ctx context.Context, req *connect.Request[v1pb.GetTaskRunSessionRequest]) (*connect.Response[v1pb.TaskRunSession], error) {
	request := req.Msg
	projectID, planID, _, taskUID, taskRunUID, err := common.GetProjectIDPlanIDStageIDTaskIDTaskRunID(request.Parent)
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

	driver, err := s.dbFactory.GetAdminDatabaseDriver(ctx, instance, nil, db.ConnectionContext{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get driver"))
	}
	defer driver.Close(ctx)

	appName := fmt.Sprintf("bytebase-taskrun-%d", taskRunUID)
	session, err := getSession(ctx, instance.Metadata.GetEngine(), driver.GetDB(), appName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get session"))
	}

	session.Name = request.Parent + "/session"

	return connect.NewResponse(session), nil
}

func getSession(ctx context.Context, engine storepb.Engine, db *sql.DB, appName string) (*v1pb.TaskRunSession, error) {
	switch engine {
	case storepb.Engine_POSTGRES, storepb.Engine_COCKROACHDB:
		query := `
			WITH target_session AS (
				SELECT pid FROM pg_catalog.pg_stat_activity WHERE application_name = $1 LIMIT 1
			)
			SELECT
				a.pid,
				pg_blocking_pids(a.pid) AS blocked_by_pids,
				a.query,
				a.state,
				a.wait_event_type,
				a.wait_event,
				a.datname,
				a.usename,
				a.application_name,
				a.client_addr,
				a.client_port,
				a.backend_start,
				a.xact_start,
				a.query_start
			FROM
				pg_catalog.pg_stat_activity a
			WHERE a.application_name = $1
			   OR (SELECT pid FROM target_session) = ANY(pg_blocking_pids(a.pid))
			   OR a.pid = ANY(pg_blocking_pids((SELECT pid FROM target_session)))
			ORDER BY a.pid
		`
		rows, err := db.QueryContext(ctx, query, appName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query rows")
		}
		defer rows.Close()

		ss := &v1pb.TaskRunSession_Postgres{}
		for rows.Next() {
			var s v1pb.TaskRunSession_Postgres_Session

			var blockedByPids pgtype.TextArray

			var bs time.Time
			var xs, qs *time.Time
			if err := rows.Scan(
				&s.Pid,
				&blockedByPids,
				&s.Query,
				&s.State,
				&s.WaitEventType,
				&s.WaitEvent,
				&s.Datname,
				&s.Usename,
				&s.ApplicationName,
				&s.ClientAddr,
				&s.ClientPort,
				&bs,
				&xs,
				&qs,
			); err != nil {
				return nil, errors.Wrapf(err, "failed to scan")
			}

			if err := blockedByPids.AssignTo(&s.BlockedByPids); err != nil {
				return nil, errors.Wrapf(err, "failed to assign blocking pids")
			}

			s.BackendStart = timestamppb.New(bs)
			if xs != nil {
				s.XactStart = timestamppb.New(*xs)
			}
			if qs != nil {
				s.QueryStart = timestamppb.New(*qs)
			}

			if s.ApplicationName == appName {
				ss.Session = &s
			} else if ss.Session != nil {
				// For blocking/blocked sessions, we need to check if they're related to our main session
				if slices.Contains(s.BlockedByPids, ss.Session.Pid) {
					ss.BlockedSessions = append(ss.BlockedSessions, &s)
				} else if slices.Contains(ss.Session.BlockedByPids, s.Pid) {
					ss.BlockingSessions = append(ss.BlockingSessions, &s)
				}
			}
		}

		if err := rows.Err(); err != nil {
			return nil, errors.Wrapf(err, "failed to scan rows")
		}

		return &v1pb.TaskRunSession{
			Session: &v1pb.TaskRunSession_Postgres_{
				Postgres: ss,
			},
		}, nil
	default:
		return nil, errors.Errorf("session monitoring is only supported for PostgreSQL and CockroachDB")
	}
}

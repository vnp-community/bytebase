package v1

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/common/permission"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	parserbase "github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/utils"
)

type accessCheckFunc func(context.Context, *store.InstanceMessage, *store.DatabaseMessage, *store.UserMessage, []*parserbase.QuerySpan, bool /* isExplain */) error

// preCheckAccess finds and returns the best matching active access grant for the query.
func (s *SQLService) preCheckAccess(ctx context.Context, request *v1pb.QueryRequest, database *store.DatabaseMessage) *store.AccessGrantMessage {
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &database.ProjectID,
	})
	if err != nil {
		slog.Warn("failed to find project", slog.String("project_id", database.ProjectID), log.BBError(err))
		return nil
	}
	if project == nil {
		slog.Warn("project not found", slog.String("project_id", database.ProjectID))
		return nil
	}
	if !project.Setting.AllowJustInTimeAccess {
		slog.Debug("JIT is not enabled in the project", slog.String("project_id", database.ProjectID))
		return nil
	}

	user, ok := GetUserFromContext(ctx)
	if !ok || user == nil {
		return nil
	}

	databaseFullName := common.FormatDatabase(database.InstanceID, database.DatabaseName)
	now := time.Now().UTC().Format(time.RFC3339)

	filter := fmt.Sprintf(
		`status == "ACTIVE" && target == %q && expire_time > %q && query == %q`,
		databaseFullName,
		now,
		strings.TrimSpace(request.Statement),
	)
	filterQ, err := store.GetListAccessGrantFilter(filter)
	if err != nil {
		slog.Warn("failed to build access grant filter", log.BBError(err))
		return nil
	}

	grants, err := s.store.ListAccessGrants(ctx, &store.FindAccessGrantMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: &database.ProjectID,
		Creator:   &user.Email,
		FilterQ:   filterQ,
	})
	if err != nil {
		slog.Warn("failed to list access grants", log.BBError(err))
		return nil
	}

	if len(grants) == 0 {
		return nil
	}
	// Pick the best grant (prefer unmask=true).
	for _, grant := range grants {
		if grant.Payload != nil && grant.Payload.Unmask {
			return grant
		}
	}
	return grants[0]
}

// accessCheck check the access for the database. Do not support cross-project resources.
func (s *SQLService) accessCheck(
	ctx context.Context,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	user *store.UserMessage,
	spans []*parserbase.QuerySpan,
	isExplain bool,
) error {
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &database.ProjectID})
	if err != nil {
		return err
	}
	if project == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.Errorf("project %q not found", database.ProjectID))
	}

	workspacePolicy, err := s.store.GetWorkspaceIamPolicy(ctx, common.GetWorkspaceIDFromContext(ctx))
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get workspace iam policy"))
	}

	projectPolicy, err := s.store.GetProjectIamPolicy(ctx, common.GetWorkspaceIDFromContext(ctx), project.ResourceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New(err.Error()))
	}

	checkDatabaseAccess := func(perm permission.Permission) error {
		databaseFullName := common.FormatDatabase(instance.ResourceID, database.DatabaseName)
		attributes := map[string]any{
			common.CELAttributeRequestTime:      time.Now(),
			common.CELAttributeResourceDatabase: databaseFullName,
		}
		env := ""
		if database.EffectiveEnvironmentID != nil {
			env = *database.EffectiveEnvironmentID
		}
		attributes[common.CELAttributeResourceEnvironmentID] = env

		ok, err := s.hasDatabaseAccessRights(
			ctx,
			user,
			perm,
			attributes,
			workspacePolicy.Policy,
			projectPolicy.Policy,
		)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.Errorf("failed to check access control for database: %q, error %v", databaseFullName, err))
		}
		if !ok {
			return &queryError{
				err: connect.NewError(
					connect.CodePermissionDenied,
					errors.Errorf("permission denied to access resources: %v", databaseFullName),
				),
				resources:  []string{databaseFullName},
				permission: string(perm),
			}
		}
		return nil
	}

	// When spans is empty, it's an EXPLAIN query where GetQuerySpan is skipped.
	if len(spans) == 0 {
		perm := permission.SQLSelect
		if isExplain {
			perm = permission.SQLExplain
		}
		return checkDatabaseAccess(perm)
	}

	for _, span := range spans {
		var perm permission.Permission
		// New query ACL experience.
		if common.EngineSupportQueryNewACL(instance.Metadata.GetEngine()) {
			switch span.Type {
			case parserbase.QueryTypeUnknown:
				return connect.NewError(connect.CodePermissionDenied, errors.New("disallowed query type"))
			case parserbase.DDL:
				perm = permission.SQLDdl
			case parserbase.DML:
				perm = permission.SQLDml
			case parserbase.Explain:
				perm = permission.SQLExplain
			case parserbase.SelectInfoSchema:
				perm = permission.SQLInfo
			case parserbase.Select:
				perm = permission.SQLSelect
			default:
			}
		} else if span.Type == parserbase.Select {
			perm = permission.SQLSelect
		}

		if perm == "" {
			// always fallback to bb.sql.select
			perm = permission.SQLSelect
		}

		// For non-SELECT queries or SELECT queries with no source columns,
		// check at database level and skip column-level checks
		if span.Type != parserbase.Select || len(span.SourceColumns) == 0 {
			if err := checkDatabaseAccess(perm); err != nil {
				return err
			}
			continue
		}

		var deniedResources []string
		for column := range span.SourceColumns {
			attributes := map[string]any{
				common.CELAttributeRequestTime:       time.Now(),
				common.CELAttributeResourceDatabase:  common.FormatDatabase(instance.ResourceID, column.Database),
				common.CELAttributeResourceSchemaName: column.Schema,
				common.CELAttributeResourceTableName:  column.Table,
			}
			ok, err := s.hasDatabaseAccessRights(
				ctx,
				user,
				perm,
				attributes,
				workspacePolicy.Policy,
				projectPolicy.Policy,
			)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Errorf("failed to check access control for database: %q, error %v", column.Database, err))
			}
			if !ok {
				resource, ok := attributes[common.CELAttributeResourceDatabase].(string)
				if !ok {
					resource = ""
				}
				if schema, ok := attributes[common.CELAttributeResourceSchemaName]; ok && schema != "" {
					resource = fmt.Sprintf("%s/schemas/%s", resource, schema)
				}
				if table, ok := attributes[common.CELAttributeResourceTableName]; ok && table != "" {
					resource = fmt.Sprintf("%s/tables/%s", resource, table)
				}
				deniedResources = append(deniedResources, resource)
			}
		}
		if len(deniedResources) > 0 {
			return &queryError{
				err: connect.NewError(
					connect.CodePermissionDenied,
					errors.Errorf("permission denied to access resources: %v", deniedResources),
				),
				resources:  deniedResources,
				permission: string(perm),
			}
		}
	}

	return nil
}

func (s *SQLService) prepareRelatedMessage(ctx context.Context, requestName string) (*store.UserMessage, *store.InstanceMessage, *store.DatabaseMessage, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, errors.New(err.Error()))
	}

	instanceID, databaseName, err := common.GetInstanceDatabaseID(requestName)
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to parse %q", requestName))
	}
	database, err := s.store.GetDatabase(ctx, &store.FindDatabaseMessage{
		Workspace:    common.GetWorkspaceIDFromContext(ctx),
		InstanceID:   &instanceID,
		DatabaseName: &databaseName,
	})
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database"))
	}
	if database == nil {
		return nil, nil, nil, connect.NewError(connect.CodeNotFound, errors.Errorf("database %q not found", requestName))
	}

	instance, err := s.store.GetInstance(ctx, &store.FindInstanceMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &database.InstanceID,
	})
	if err != nil {
		return nil, nil, nil, connect.NewError(connect.CodeInternal, errors.New(err.Error()))
	}
	if instance == nil {
		return nil, nil, nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q not found", database.InstanceID))
	}

	return user, instance, database, nil
}

func (s *SQLService) hasDatabaseAccessRights(
	ctx context.Context,
	user *store.UserMessage,
	perm permission.Permission,
	attributes map[string]any,
	iamPolicies ...*storepb.IamPolicy,
) (bool, error) {
	bindings := utils.GetUserIAMPolicyBindings(ctx, s.store, common.GetWorkspaceIDFromContext(ctx), user, iamPolicies...)
	for _, binding := range bindings {
		permissions, err := s.iamManager.GetPermissions(ctx, common.GetWorkspaceIDFromContext(ctx), binding.Role)
		if err != nil {
			return false, errors.Wrapf(err, "failed to get permissions")
		}
		if !permissions[perm] {
			continue
		}

		expression := binding.Condition.GetExpression()
		// resource.environment_id only applies to DDL/DML permissions.
		if perm != permission.SQLDdl && perm != permission.SQLDml {
			expression = stripEnvironmentCondition(expression)
		}

		ok, err := evaluateQueryExportPolicyCondition(expression, attributes)
		if err != nil {
			slog.Error("failed to evaluate condition", log.BBError(err), slog.String("condition", expression))
			continue
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

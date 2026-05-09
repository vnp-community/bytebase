package v1

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	celast "github.com/google/cel-go/common/ast"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
)

// DatabaseService implements the database service.
type DatabaseService struct {
	v1connect.UnimplementedDatabaseServiceHandler
	store          store.DataStore
	schemaSyncer   *schemasync.Syncer
	profile        *config.Profile
	iamManager     *iam.Manager
	licenseService *enterprise.LicenseService
}

// NewDatabaseService creates a new DatabaseService.
func NewDatabaseService(store store.DataStore, schemaSyncer *schemasync.Syncer, profile *config.Profile, iamManager *iam.Manager, licenseService *enterprise.LicenseService) *DatabaseService {
	return &DatabaseService{
		store:          store,
		schemaSyncer:   schemaSyncer,
		profile:        profile,
		iamManager:     iamManager,
		licenseService: licenseService,
	}
}

// GetDatabase gets a database.
func (s *DatabaseService) GetDatabase(ctx context.Context, req *connect.Request[v1pb.GetDatabaseRequest]) (*connect.Response[v1pb.Database], error) {
	instanceID, databaseName, err := common.GetInstanceDatabaseID(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to parse %q", req.Msg.Name))
	}
	databaseMessage, err := s.store.GetDatabase(ctx, &store.FindDatabaseMessage{
		Workspace:    common.GetWorkspaceIDFromContext(ctx),
		InstanceID:   &instanceID,
		DatabaseName: &databaseName,
		ShowDeleted:  true,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database"))
	}
	if databaseMessage == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("database %q not found", req.Msg.Name))
	}
	database, err := s.convertToDatabase(ctx, databaseMessage)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert database"))
	}
	return connect.NewResponse(database), nil
}

func (s *DatabaseService) BatchGetDatabases(ctx context.Context, req *connect.Request[v1pb.BatchGetDatabasesRequest]) (*connect.Response[v1pb.BatchGetDatabasesResponse], error) {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}

	// Parse parent to extract project ID filter if specified.
	var projectIDFilter *string
	if strings.HasPrefix(req.Msg.Parent, common.ProjectNamePrefix) {
		projectID, err := common.GetProjectID(req.Msg.Parent)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid parent %q", req.Msg.Parent))
		}
		projectIDFilter = new(projectID)
	}
	// For instances/{instance} or "-" (wildcard), no project filter is applied.
	databases := make([]*v1pb.Database, 0, len(req.Msg.Names))
	for _, name := range req.Msg.Names {
		instanceID, databaseName, err := common.GetInstanceDatabaseID(name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to parse %q", name))
		}
		databaseMessage, err := s.store.GetDatabase(ctx, &store.FindDatabaseMessage{
			Workspace:    common.GetWorkspaceIDFromContext(ctx),
			InstanceID:   &instanceID,
			DatabaseName: &databaseName,
			ShowDeleted:  true,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database"))
		}
		if databaseMessage == nil {
			// Ignore deleted database.
			continue
		}
		// If parent specifies a project, validate database belongs to that project.
		if projectIDFilter != nil && databaseMessage.ProjectID != *projectIDFilter {
			// Ignore database not in the specified project.
			continue
		}
		ok, err := s.iamManager.CheckPermission(ctx, permission.DatabasesGet, user, common.GetWorkspaceIDFromContext(ctx), databaseMessage.ProjectID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to check permission with error: %v", err.Error()))
		}
		if !ok {
			// Ignore no permission database.
			continue
		}
		database, err := s.convertToDatabase(ctx, databaseMessage)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert database"))
		}
		databases = append(databases, database)
	}
	return connect.NewResponse(&v1pb.BatchGetDatabasesResponse{Databases: databases}), nil
}

func getVariableAndValueFromExpr(expr celast.Expr) (string, any) {
	var variable string
	var value any
	for _, arg := range expr.AsCall().Args() {
		switch arg.Kind() {
		case celast.IdentKind:
			variable = arg.AsIdent()
		case celast.SelectKind:
			// Handle member selection like "labels.environment"
			sel := arg.AsSelect()
			if sel.Operand().Kind() == celast.IdentKind {
				variable = fmt.Sprintf("%s.%s", sel.Operand().AsIdent(), sel.FieldName())
			}
		case celast.LiteralKind:
			value = arg.AsLiteral().Value()
		case celast.ListKind:
			list := []any{}
			for _, e := range arg.AsList().Elements() {
				if e.Kind() == celast.LiteralKind {
					list = append(list, e.AsLiteral().Value())
				}
			}
			value = list
		default:
		}
	}
	return variable, value
}

func getSubConditionFromExpr(expr celast.Expr, getFilter func(expr celast.Expr) (string, error), join string) (string, error) {
	var args []string
	for _, arg := range expr.AsCall().Args() {
		s, err := getFilter(arg)
		if err != nil {
			return "", err
		}
		args = append(args, "("+s+")")
	}
	return strings.Join(args, fmt.Sprintf(" %s ", join)), nil
}

// ListDatabases lists all databases.
func (s *DatabaseService) ListDatabases(ctx context.Context, req *connect.Request[v1pb.ListDatabasesRequest]) (*connect.Response[v1pb.ListDatabasesResponse], error) {
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

	find := &store.FindDatabaseMessage{
		Workspace:   common.GetWorkspaceIDFromContext(ctx),
		Limit:       &limitPlusOne,
		Offset:      &offset.offset,
		ShowDeleted: req.Msg.ShowDeleted,
	}

	orderByKeys, err := store.GetDatabaseOrders(req.Msg.OrderBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.OrderByKeys = orderByKeys

	filterQ, err := store.GetListDatabaseFilter(common.GetWorkspaceIDFromContext(ctx), req.Msg.Filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.FilterQ = filterQ

	switch {
	case strings.HasPrefix(req.Msg.Parent, common.ProjectNamePrefix):
		p, err := common.GetProjectID(req.Msg.Parent)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid parent %q", req.Msg.Parent))
		}
		ok, err := s.iamManager.CheckPermission(ctx, permission.ProjectsGet, user, common.GetWorkspaceIDFromContext(ctx), p)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to check permission with error: %v", err.Error()))
		}
		if !ok {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("user does not have permission %q in %q", permission.ProjectsGet, req.Msg.Parent))
		}
		find.ProjectID = &p
	case strings.HasPrefix(req.Msg.Parent, common.WorkspacePrefix):
		ok, err := s.iamManager.CheckPermission(ctx, permission.DatabasesList, user, common.GetWorkspaceIDFromContext(ctx))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to check permission with error: %v", err.Error()))
		}
		if !ok {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("user does not have permission %q", permission.DatabasesList))
		}
	case strings.HasPrefix(req.Msg.Parent, common.InstanceNamePrefix):
		ok, err := s.iamManager.CheckPermission(ctx, permission.InstancesGet, user, common.GetWorkspaceIDFromContext(ctx))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to check permission with error: %v", err.Error()))
		}
		if !ok {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("user does not have permission %q", permission.InstancesGet))
		}

		instanceID, err := common.GetInstanceID(req.Msg.Parent)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid parent %q", req.Msg.Parent))
		}
		find.InstanceID = &instanceID
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid parent %q", req.Msg.Parent))
	}

	databaseMessages, err := s.store.ListDatabases(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("%v", err.Error()))
	}

	nextPageToken := ""
	if len(databaseMessages) == limitPlusOne {
		databaseMessages = databaseMessages[:offset.limit]
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to marshal next page token"))
		}
	}

	response := &v1pb.ListDatabasesResponse{
		NextPageToken: nextPageToken,
	}
	for _, databaseMessage := range databaseMessages {
		database, err := s.convertToDatabase(ctx, databaseMessage)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert database"))
		}
		response.Databases = append(response.Databases, database)
	}
	return connect.NewResponse(response), nil
}

// UpdateDatabase updates a database.
func (s *DatabaseService) UpdateDatabase(ctx context.Context, req *connect.Request[v1pb.UpdateDatabaseRequest]) (*connect.Response[v1pb.Database], error) {
	if req.Msg.Database == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("database must be set"))
	}
	if len(req.Msg.GetUpdateMask().GetPaths()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("update_mask must be set"))
	}

	// Use the helper function to get the database
	instanceID, databaseName, err := common.GetInstanceDatabaseID(req.Msg.Database.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to parse %q", req.Msg.Database.Name))
	}
	databaseMessage, err := s.store.GetDatabase(ctx, &store.FindDatabaseMessage{
		Workspace:    common.GetWorkspaceIDFromContext(ctx),
		InstanceID:   &instanceID,
		DatabaseName: &databaseName,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database"))
	}
	if databaseMessage == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("database %q not found", req.Msg.Database.Name))
	}
	if databaseMessage.Deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("database %q was deleted", req.Msg.Database.Name))
	}

	var project *store.ProjectMessage
	patch := &store.UpdateDatabaseMessage{
		InstanceID:   databaseMessage.InstanceID,
		DatabaseName: databaseMessage.DatabaseName,
	}
	for _, path := range req.Msg.UpdateMask.Paths {
		switch path {
		case "project":
			projectID, err := common.GetProjectID(req.Msg.Database.Project)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
			}
			project, err = s.store.GetProject(ctx, &store.FindProjectMessage{
				Workspace:   common.GetWorkspaceIDFromContext(ctx),
				ResourceID:  &projectID,
				ShowDeleted: true,
			})
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Errorf("%v", err.Error()))
			}
			if project == nil {
				return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project %q not found", projectID))
			}
			if project.Deleted {
				return nil, connect.NewError(connect.CodeFailedPrecondition, errors.Errorf("project %q is deleted", projectID))
			}
			patch.ProjectID = &project.ResourceID
		case "labels":
			patch.MetadataUpdates = append(patch.MetadataUpdates, func(dm *storepb.DatabaseMetadata) {
				dm.Labels = req.Msg.Database.Labels
			})
		case "environment":
			if req.Msg.Database.Environment != nil && *req.Msg.Database.Environment != "" {
				environmentID, err := common.GetEnvironmentID(*req.Msg.Database.Environment)
				if err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
				}
				environment, err := s.store.GetEnvironmentByID(ctx, common.GetWorkspaceIDFromContext(ctx), environmentID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, errors.Errorf("%v", err.Error()))
				}
				if environment == nil {
					return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("environment %q not found", environmentID))
				}
				patch.EnvironmentID = &environmentID
			} else {
				patch.EnvironmentID = new("")
			}
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unsupported update mask path %q", path))
		}
	}

	updatedDatabase, err := s.store.UpdateDatabase(ctx, patch)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("%v", err.Error()))
	}

	database, err := s.convertToDatabase(ctx, updatedDatabase)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert database"))
	}
	return connect.NewResponse(database), nil
}


package v1

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/schema"
	parserbase "github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/store/model"
	"github.com/bytebase/bytebase/backend/utils"
)

func (s *SQLService) createQueryHistory(database *store.DatabaseMessage, queryType store.QueryHistoryType, statement string, userEmail string, duration time.Duration, queryErr error) {
	qh := &store.QueryHistoryMessage{
		Creator:   userEmail,
		Project:   database.ProjectID,
		Database:  common.FormatDatabase(database.InstanceID, database.DatabaseName),
		Statement: statement,
		Type:      queryType,
		Payload: &storepb.QueryHistoryPayload{
			Error:    nil,
			Duration: durationpb.New(duration),
		},
	}
	if queryErr != nil {
		qh.Payload.Error = new(queryErr.Error())
	}

	historyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.store.CreateQueryHistory(historyCtx, qh); err != nil {
		queryErr := ""
		if v := qh.Payload.Error; v != nil {
			queryErr = *v
		}
		slog.Error(
			"failed to create query history",
			log.BBError(err),
			slog.String("instance", database.InstanceID),
			slog.String("database", database.DatabaseName),
			slog.String("project", database.ProjectID),
			slog.String("query_error", queryErr),
		)
	}
}

// SearchQueryHistories lists query histories.
func (s *SQLService) SearchQueryHistories(ctx context.Context, req *connect.Request[v1pb.SearchQueryHistoriesRequest]) (*connect.Response[v1pb.SearchQueryHistoriesResponse], error) {
	request := req.Msg
	offset, err := parseLimitAndOffset(&pageSize{
		token:   request.PageToken,
		limit:   int(request.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}

	find := &store.FindQueryHistoryMessage{
		Creator: &user.Email,
		Limit:   &limitPlusOne,
		Offset:  &offset.offset,
	}
	filterQ, err := store.GetListQueryHistoryFilter(request.Filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	find.FilterQ = filterQ

	historyList, err := s.store.ListQueryHistories(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to list history: %v", err.Error()))
	}

	nextPageToken := ""
	if len(historyList) == limitPlusOne {
		historyList = historyList[:offset.limit]
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to marshal next page token"))
		}
	}

	resp := &v1pb.SearchQueryHistoriesResponse{
		NextPageToken: nextPageToken,
	}
	for _, history := range historyList {
		queryHistory, err := s.convertToV1QueryHistory(ctx, history)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert log entity"))
		}
		if queryHistory == nil {
			continue
		}
		resp.QueryHistories = append(resp.QueryHistories, queryHistory)
	}

	return connect.NewResponse(resp), nil
}

func BuildGetLinkedDatabaseMetadataFunc(storeInstance *store.Store, engine storepb.Engine) parserbase.GetLinkedDatabaseMetadataFunc {
	if engine != storepb.Engine_ORACLE {
		return nil
	}
	return func(ctx context.Context, instanceID string, linkedDatabaseName string, schemaName string) (string, string, *model.DatabaseMetadata, error) {
		databases, err := storeInstance.ListDatabases(ctx, &store.FindDatabaseMessage{
			Workspace:  common.GetWorkspaceIDFromContext(ctx),
			InstanceID: &instanceID,
		})
		if err != nil {
			return "", "", nil, err
		}
		var linkedMeta *storepb.LinkedDatabaseMetadata
		for _, database := range databases {
			meta, err := storeInstance.GetDBSchema(ctx, &store.FindDBSchemaMessage{
				Workspace:    common.GetWorkspaceIDFromContext(ctx),
				InstanceID:   database.InstanceID,
				DatabaseName: database.DatabaseName,
			})
			if err != nil {
				return "", "", nil, err
			}
			if linkedMeta = meta.GetLinkedDatabase(linkedDatabaseName); linkedMeta != nil {
				break
			}
		}
		if linkedMeta == nil {
			return "", "", nil, nil
		}
		var linkedDatabase *store.DatabaseMessage
		databaseName := linkedMeta.GetUsername()
		if schemaName != "" {
			databaseName = schemaName
		}
		databaseList, err := storeInstance.ListDatabases(ctx, &store.FindDatabaseMessage{
			Workspace:    common.GetWorkspaceIDFromContext(ctx),
			DatabaseName: &databaseName,
			Engine:       &engine,
		})
		if err != nil {
			return "", "", nil, err
		}
		for _, database := range databaseList {
			instance, err := storeInstance.GetInstance(ctx, &store.FindInstanceMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &database.InstanceID})
			if err != nil {
				return "", "", nil, err
			}
			if instance != nil {
				for _, dataSource := range instance.Metadata.DataSources {
					if strings.Contains(linkedMeta.GetHost(), dataSource.GetHost()) {
						linkedDatabase = database
						break
					}
				}
				if linkedDatabase != nil {
					break
				}
			}
		}
		if linkedDatabase == nil {
			return "", "", nil, nil
		}
		linkedDatabaseMetadata, err := storeInstance.GetDBSchema(ctx, &store.FindDBSchemaMessage{
			Workspace:    common.GetWorkspaceIDFromContext(ctx),
			InstanceID:   linkedDatabase.InstanceID,
			DatabaseName: linkedDatabase.DatabaseName,
		})
		if err != nil {
			return "", "", nil, err
		}
		if linkedDatabaseMetadata == nil {
			return "", "", nil, nil
		}
		return linkedDatabase.InstanceID, linkedDatabaseName, linkedDatabaseMetadata, nil
	}
}

func BuildGetDatabaseMetadataFunc(storeInstance *store.Store) parserbase.GetDatabaseMetadataFunc {
	return func(ctx context.Context, instanceID, databaseName string) (string, *model.DatabaseMetadata, error) {
		databaseMetadata, err := storeInstance.GetDBSchema(ctx, &store.FindDBSchemaMessage{
			Workspace:    common.GetWorkspaceIDFromContext(ctx),
			InstanceID:   instanceID,
			DatabaseName: databaseName,
		})
		if err != nil {
			return "", nil, err
		}
		if databaseMetadata == nil {
			return "", nil, nil
		}
		return databaseName, databaseMetadata, nil
	}
}

func BuildListDatabaseNamesFunc(storeInstance *store.Store) parserbase.ListDatabaseNamesFunc {
	return func(ctx context.Context, instanceID string) ([]string, error) {
		databases, err := storeInstance.ListDatabases(ctx, &store.FindDatabaseMessage{
			Workspace:  common.GetWorkspaceIDFromContext(ctx),
			InstanceID: &instanceID,
		})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(databases))
		for _, database := range databases {
			names = append(names, database.DatabaseName)
		}
		return names, nil
	}
}

func resolveDataSourceID(instance *store.InstanceMessage, dataSourceID string) (string, error) {
	if dataSourceID != "" {
		return dataSourceID, nil
	}

	var adminDataSourceID string
	var readOnlyDataSourceID string
	readOnlyCount := 0
	for _, dataSource := range instance.Metadata.GetDataSources() {
		switch dataSource.GetType() {
		case storepb.DataSourceType_ADMIN:
			adminDataSourceID = dataSource.GetId()
		case storepb.DataSourceType_READ_ONLY:
			readOnlyCount++
			readOnlyDataSourceID = dataSource.GetId()
		default:
		}
	}

	switch {
	case readOnlyCount == 1:
		return readOnlyDataSourceID, nil
	case readOnlyCount > 1:
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("instance has multiple read-only data sources, please specify data_source_id explicitly"))
	case adminDataSourceID != "":
		return adminDataSourceID, nil
	default:
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("instance has no admin data source"))
	}
}

func checkAndGetDataSourceQueriable(
	ctx context.Context,
	storeInstance *store.Store,
	licenseService *enterprise.LicenseService,
	database *store.DatabaseMessage,
	dataSourceID string,
) (*storepb.DataSource, error) {
	if dataSourceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("data source id is required"))
	}

	instance, err := storeInstance.GetInstance(ctx, &store.FindInstanceMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), ResourceID: &database.InstanceID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get instance %v with error: %v", database.InstanceID, err.Error()))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q not found", database.InstanceID))
	}
	dataSource := func() *storepb.DataSource {
		for _, ds := range instance.Metadata.GetDataSources() {
			if ds.GetId() == dataSourceID {
				return ds
			}
		}
		return nil
	}()
	if dataSource == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("data source %q not found", dataSourceID))
	}

	if dataSource.GetType() != storepb.DataSourceType_ADMIN {
		if err := licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_INSTANCE_READ_ONLY_CONNECTION, instance); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New(err.Error()))
		}
		return dataSource, nil
	}

	//nolint:nilerr // feature not enabled — license check "error" means feature unavailable, not a failure; return admin data source as-is.
	if err := licenseService.IsFeatureEnabled(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_QUERY_POLICY); err != nil {
		return dataSource, nil
	}

	queryDataPolicy, err := storeInstance.GetEffectiveQueryDataPolicy(ctx, common.GetWorkspaceIDFromContext(ctx), common.FormatProject(database.ProjectID))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get query data policy with error: %v", err.Error()))
	}

	if queryDataPolicy.AllowAdminDataSource {
		return dataSource, nil
	}

	ds := utils.DataSourceFromInstanceWithType(instance, storepb.DataSourceType_READ_ONLY)
	if ds != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("data source %q is not queryable", dataSourceID))
	}
	return dataSource, nil
}

func (*SQLService) DiffMetadata(_ context.Context, req *connect.Request[v1pb.DiffMetadataRequest]) (*connect.Response[v1pb.DiffMetadataResponse], error) {
	request := req.Msg
	switch request.Engine {
	case v1pb.Engine_MYSQL, v1pb.Engine_POSTGRES, v1pb.Engine_TIDB, v1pb.Engine_ORACLE, v1pb.Engine_MSSQL:
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unsupported engine: %v", request.Engine))
	}
	if request.SourceMetadata == nil || request.TargetMetadata == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("source_metadata and target_metadata are required"))
	}
	storeSourceMetadata := convertV1DatabaseMetadata(request.SourceMetadata)
	storeTargetMetadata := convertV1DatabaseMetadata(request.TargetMetadata)

	isObjectCaseSensitive := true
	sourceDBSchema := model.NewDatabaseMetadata(storeSourceMetadata, nil, nil, storepb.Engine(request.Engine), isObjectCaseSensitive)
	targetDBSchema := model.NewDatabaseMetadata(storeTargetMetadata, nil, nil, storepb.Engine(request.Engine), isObjectCaseSensitive)

	migrationSQL, err := schema.DiffMigration(storepb.Engine(request.Engine), sourceDBSchema, targetDBSchema)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to compute diff between source and target schemas"))
	}

	return connect.NewResponse(&v1pb.DiffMetadataResponse{
		Diff: migrationSQL,
	}), nil
}

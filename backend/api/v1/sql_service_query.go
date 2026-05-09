package v1

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"regexp"
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
	"github.com/bytebase/bytebase/backend/plugin/advisor/code"
	"github.com/bytebase/bytebase/backend/plugin/db"
	parserbase "github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/store/model"
)

// Query executes a SQL query.
func (s *SQLService) Query(ctx context.Context, req *connect.Request[v1pb.QueryRequest]) (*connect.Response[v1pb.QueryResponse], error) {
	request := req.Msg
	// Prepare related message.
	user, instance, database, err := s.prepareRelatedMessage(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	accessGrant := s.preCheckAccess(ctx, request, database)

	statement := request.Statement
	// In Redshift datashare, Rewrite query used for parser.
	if database.Metadata.GetDatashare() {
		statement = strings.ReplaceAll(statement, fmt.Sprintf("%s.", database.DatabaseName), "")
	}

	// Validate the request.
	// New query ACL experience.
	if !request.Explain && !common.EngineSupportQueryNewACL(instance.Metadata.GetEngine()) {
		if err := validateQueryRequest(instance, statement); err != nil {
			return nil, err
		}
	}

	resolvedDataSourceID, err := resolveDataSourceID(instance, request.DataSourceId)
	if err != nil {
		return nil, err
	}

	dataSource, err := checkAndGetDataSourceQueriable(ctx, s.store, s.licenseService, database, resolvedDataSourceID)
	if err != nil {
		return nil, err
	}
	driver, err := s.dbFactory.GetDataSourceDriver(ctx, instance, dataSource, db.ConnectionContext{
		DatabaseName: database.DatabaseName,
		DataShare:    database.Metadata.GetDatashare(),
		ReadOnly:     dataSource.GetType() == storepb.DataSourceType_READ_ONLY,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database driver: %v", err))
	}
	defer driver.Close(ctx)

	sqlDB := driver.GetDB()
	var conn *sql.Conn
	if sqlDB != nil {
		conn, err = sqlDB.Conn(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database connection: %v", err))
		}
		defer conn.Close()
	}

	startTime := time.Now()
	queryRestriction := getEffectiveQueryDataPolicy(
		ctx,
		s.store,
		s.licenseService,
		request.Limit,
		database.ProjectID,
	)
	queryContext := db.QueryContext{
		Explain:              request.Explain,
		Limit:                int(queryRestriction.MaximumResultRows),
		OperatorEmail:        user.Email,
		Option:               request.QueryOption,
		Container:            request.GetContainer(),
		MaximumSQLResultSize: queryRestriction.MaximumResultSize,
		SkipMasking:          accessGrant != nil && accessGrant.Payload.Unmask,
	}
	if request.Schema != nil {
		queryContext.Schema = *request.Schema
	}
	if queryRestriction.MaxQueryTimeoutInSeconds > 0 {
		queryContext.Timeout = &durationpb.Duration{Seconds: queryRestriction.MaxQueryTimeoutInSeconds}
	}

	var optionalAccessCheck accessCheckFunc
	if accessGrant == nil {
		optionalAccessCheck = s.accessCheck
	}
	results, _, duration, queryErr := queryRetryStopOnError(
		ctx,
		s.store,
		user,
		instance,
		database,
		driver,
		conn,
		statement,
		queryContext,
		s.licenseService,
		optionalAccessCheck,
		s.schemaSyncer,
	)
	slog.Debug("query finished",
		log.BBError(queryErr),
		slog.Duration("duration", time.Since(startTime)),
		slog.String("instance", instance.ResourceID),
		slog.String("database", database.DatabaseName),
	)

	// Update activity.
	s.createQueryHistory(database, store.QueryHistoryTypeQuery, statement, user.Email, duration, queryErr)

	if queryErr != nil {
		if len(results) == 0 {
			if _, ok := queryErr.(*connect.Error); ok {
				return nil, queryErr
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New(queryErr.Error()))
		}
		// populate the detailed_error field of the last query result
		var qe *queryError
		var pe *parserbase.SyntaxError
		if errors.As(queryErr, &qe) {
			if len(qe.resources) > 0 {
				results[len(results)-1].DetailedError = &v1pb.QueryResult_PermissionDenied{
					PermissionDenied: &v1pb.PermissionDeniedDetail{
						Resources: qe.resources,
						RequiredPermissions: []string{
							qe.permission,
						},
					},
				}
			} else if qe.commandType != v1pb.QueryResult_CommandError_TYPE_UNSPECIFIED {
				results[len(results)-1].DetailedError = &v1pb.QueryResult_CommandError_{
					CommandError: &v1pb.QueryResult_CommandError{
						CommandType: qe.commandType,
					},
				}
			}
		} else if errors.As(queryErr, &pe) {
			results[len(results)-1].DetailedError = &v1pb.QueryResult_SyntaxError_{
				SyntaxError: &v1pb.QueryResult_SyntaxError{
					StartPosition: convertToPosition(pe.Position),
				},
			}
		}
	}

	slog.Debug("request finished",
		slog.Duration("duration", time.Since(startTime)),
		slog.String("instance", instance.ResourceID),
		slog.String("database", database.DatabaseName),
	)

	response := &v1pb.QueryResponse{
		Results: results,
	}

	return connect.NewResponse(response), nil
}

func getEffectiveQueryDataPolicy(
	ctx context.Context,
	stores *store.Store,
	licenseService *enterprise.LicenseService,
	limit int32,
	projectID string,
) *store.EffectiveQueryDataPolicy {
	value := &store.EffectiveQueryDataPolicy{
		MaximumResultSize: common.DefaultMaximumSQLResultSize,
		MaximumResultRows: math.MaxInt32,
	}
	if err := licenseService.IsFeatureEnabled(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_QUERY_POLICY); err == nil {
		policy, err := stores.GetEffectiveQueryDataPolicy(ctx, common.GetWorkspaceIDFromContext(ctx), common.FormatProject(projectID))
		if err != nil {
			slog.Error("failed to get the query data policy", log.BBError(err))
			return value
		}
		value = policy
	}
	if limit > 0 {
		value.MaximumResultRows = min(limit, value.MaximumResultRows)
	}
	if value.MaximumResultRows == math.MaxInt32 {
		value.MaximumResultRows = 0
	}
	return value
}

func extractSourceTable(comment string) (string, string, string, error) {
	pattern := `\((\w+),\s*(\w+)(?:,\s*(\w+))?\)`
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(comment)

	if len(matches) == 3 || (len(matches) == 4 && matches[3] == "") {
		databaseName := matches[1]
		tableName := matches[2]
		return databaseName, "", tableName, nil
	} else if len(matches) == 4 {
		databaseName := matches[1]
		schemaName := matches[2]
		tableName := matches[3]
		return databaseName, schemaName, tableName, nil
	}

	return "", "", "", errors.Errorf("failed to extract source table from comment: %s", comment)
}

func getSchemaMetadata(engine storepb.Engine, dbMetadata *model.DatabaseMetadata) *model.SchemaMetadata {
	switch engine {
	case storepb.Engine_POSTGRES:
		return dbMetadata.GetSchemaMetadata(common.BackupDatabaseNameOfEngine(storepb.Engine_POSTGRES))
	case storepb.Engine_MSSQL:
		return dbMetadata.GetSchemaMetadata("dbo")
	default:
		return dbMetadata.GetSchemaMetadata("")
	}
}

func replaceBackupTableWithSource(ctx context.Context, stores *store.Store, instance *store.InstanceMessage, database *store.DatabaseMessage, spans []*parserbase.QuerySpan) error {
	switch instance.Metadata.GetEngine() {
	case storepb.Engine_POSTGRES:
		// Don't need to check the database name for postgres here.
		// We backup the table to the same database with bbdataarchive schema for Postgres.
	case storepb.Engine_ORACLE:
		if database.DatabaseName != common.BackupDatabaseNameOfEngine(storepb.Engine_ORACLE) {
			return nil
		}
	default:
		if database.DatabaseName != common.BackupDatabaseNameOfEngine(instance.Metadata.GetEngine()) {
			return nil
		}
	}
	dbMetadata, err := stores.GetDBSchema(ctx, &store.FindDBSchemaMessage{
		Workspace:    common.GetWorkspaceIDFromContext(ctx),
		InstanceID:   database.InstanceID,
		DatabaseName: database.DatabaseName,
	})
	if err != nil {
		return err
	}
	schema := getSchemaMetadata(instance.Metadata.GetEngine(), dbMetadata)
	if schema == nil {
		return nil
	}

	for _, span := range spans {
		span.SourceColumns = generateNewSourceColumnSet(instance.Metadata.GetEngine(), span.SourceColumns, schema)
		for _, result := range span.Results {
			result.SourceColumns = generateNewSourceColumnSet(instance.Metadata.GetEngine(), result.SourceColumns, schema)
		}
	}
	return nil
}

func generateNewSourceColumnSet(engine storepb.Engine, origin parserbase.SourceColumnSet, schema *model.SchemaMetadata) parserbase.SourceColumnSet {
	result := make(parserbase.SourceColumnSet)
	for column := range origin {
		if isBackupTable(engine, column) {
			tableSchema := schema.GetTable(column.Table)
			if tableSchema == nil {
				result[column] = true
				continue
			}
			sourceDatabase, sourceSchema, sourceTable, err := extractSourceTable(tableSchema.GetTableComment())
			if err != nil {
				slog.Debug("failed to extract source table", log.BBError(err))
				result[column] = true
				continue
			}
			newColumn := generateNewColumn(engine, column, sourceDatabase, sourceSchema, sourceTable)
			result[newColumn] = true
		} else {
			result[column] = true
		}
	}
	return result
}

func generateNewColumn(engine storepb.Engine, column parserbase.ColumnResource, database, schema, table string) parserbase.ColumnResource {
	switch engine {
	case storepb.Engine_POSTGRES:
		return parserbase.ColumnResource{
			Server:   column.Server,
			Database: column.Database,
			Schema:   database,
			Table:    table,
			Column:   column.Column,
		}
	default:
		return parserbase.ColumnResource{
			Server:   column.Server,
			Database: database,
			Schema:   schema,
			Table:    table,
			Column:   column.Column,
		}
	}
}

func isBackupTable(engine storepb.Engine, column parserbase.ColumnResource) bool {
	switch engine {
	case storepb.Engine_POSTGRES:
		return column.Schema == common.BackupDatabaseNameOfEngine(storepb.Engine_POSTGRES)
	case storepb.Engine_ORACLE:
		return column.Database == common.BackupDatabaseNameOfEngine(storepb.Engine_ORACLE)
	default:
		return column.Database == common.BackupDatabaseNameOfEngine(engine)
	}
}

func queryRetry(
	ctx context.Context,
	stores *store.Store,
	user *store.UserMessage,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	driver db.Driver,
	conn *sql.Conn,
	statements []parserbase.Statement,
	originalStatement string,
	queryContext db.QueryContext,
	licenseService *enterprise.LicenseService,
	optionalAccessCheck accessCheckFunc,
	schemaSyncer *schemasync.Syncer,
) ([]*v1pb.QueryResult, []*parserbase.QuerySpan, time.Duration, error) {
	var spans []*parserbase.QuerySpan
	var sensitivePredicateColumns [][]parserbase.ColumnResource
	var err error
	if !queryContext.Explain {
		spans, err = parserbase.GetQuerySpan(
			ctx,
			parserbase.GetQuerySpanContext{
				InstanceID:                    instance.ResourceID,
				GetDatabaseMetadataFunc:       BuildGetDatabaseMetadataFunc(stores),
				ListDatabaseNamesFunc:         BuildListDatabaseNamesFunc(stores),
				GetLinkedDatabaseMetadataFunc: BuildGetLinkedDatabaseMetadataFunc(stores, instance.Metadata.GetEngine()),
			},
			instance.Metadata.GetEngine(),
			statements,
			database.DatabaseName,
			queryContext.Schema,
			!store.IsObjectCaseSensitive(instance),
		)
		if err != nil {
			return nil, nil, time.Duration(0), err
		}
		if err := replaceBackupTableWithSource(ctx, stores, instance, database, spans); err != nil {
			slog.Debug("failed to replace backup table with source", log.BBError(err))
		}
		if optionalAccessCheck != nil {
			if err := optionalAccessCheck(ctx, instance, database, user, spans, queryContext.Explain); err != nil {
				return nil, nil, time.Duration(0), err
			}
			slog.Debug("optional access check", slog.String("instance", instance.ResourceID), slog.String("database", database.DatabaseName))
		}
		if !queryContext.SkipMasking && licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_DATA_MASKING, instance) == nil {
			masker := NewQueryResultMasker(stores)
			sensitivePredicateColumns, err = masker.ExtractSensitivePredicateColumns(ctx, spans, instance, user)
			if err != nil {
				return nil, nil, time.Duration(0), connect.NewError(connect.CodeInternal, errors.New(err.Error()))
			}
			slog.Debug("extract sensitive predicate columns", slog.String("instance", instance.ResourceID), slog.String("database", database.DatabaseName))
		}
	}

	maskingEnabled := !queryContext.Explain && !queryContext.SkipMasking &&
		licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_DATA_MASKING, instance) == nil

	if maskingEnabled {
		if err := preExecuteMaskingCheck(ctx, stores, instance.Metadata.GetEngine(), database, spans); err != nil {
			return nil, nil, time.Duration(0), err
		}
	}

	slog.Debug("start execute with timeout", slog.String("instance", instance.ResourceID), slog.String("database", database.DatabaseName), slog.String("statement", originalStatement))
	results, duration, queryErr := executeWithTimeout(
		ctx,
		driver,
		conn,
		originalStatement,
		queryContext,
	)
	if queryErr != nil {
		return nil, nil, duration, queryErr
	}
	slog.Debug("execute success", slog.String("instance", instance.ResourceID), slog.String("statement", originalStatement), slog.Duration("duration", duration))
	if queryContext.Explain {
		return results, nil, duration, nil
	}

	syncDatabaseMap := make(map[string]bool)
	for i, r := range results {
		if r.Error != "" {
			continue
		}
		if i < len(spans) && spans[i].NotFoundError != nil {
			for k := range spans[i].SourceColumns {
				slog.Debug("database metadata need to sync", slog.String("instance", instance.ResourceID), slog.String("database", k.Database), slog.String("schema", k.Schema), slog.String("table", k.Table), slog.String("column", k.Column))
				syncDatabaseMap[k.Database] = true
			}
		}
	}

	// Sync database metadata.
	for accessDatabaseName := range syncDatabaseMap {
		slog.Debug("sync database metadata", slog.String("instance", instance.ResourceID), slog.String("database", accessDatabaseName))
		d, err := stores.GetDatabase(ctx, &store.FindDatabaseMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), InstanceID: &instance.ResourceID, DatabaseName: &accessDatabaseName})
		if err != nil {
			return nil, nil, duration, err
		}
		if d == nil {
			slog.Debug("skip metadata sync: database not tracked",
				slog.String("instance", instance.ResourceID),
				slog.String("database", accessDatabaseName))
			continue
		}
		if err := schemaSyncer.SyncDatabaseSchema(ctx, d); err != nil {
			return nil, nil, duration, errors.Wrapf(err, "failed to sync database schema for database %q", accessDatabaseName)
		}
	}

	// Retry getting query span.
	if len(syncDatabaseMap) > 0 {
		slog.Debug("retry query after sync metadata", slog.String("instance", instance.ResourceID), slog.String("database", database.DatabaseName))
		spans, err = parserbase.GetQuerySpan(
			ctx,
			parserbase.GetQuerySpanContext{
				InstanceID:                    instance.ResourceID,
				GetDatabaseMetadataFunc:       BuildGetDatabaseMetadataFunc(stores),
				ListDatabaseNamesFunc:         BuildListDatabaseNamesFunc(stores),
				GetLinkedDatabaseMetadataFunc: BuildGetLinkedDatabaseMetadataFunc(stores, instance.Metadata.GetEngine()),
			},
			instance.Metadata.GetEngine(),
			statements,
			database.DatabaseName,
			queryContext.Schema,
			!store.IsObjectCaseSensitive(instance),
		)
		if err != nil {
			return nil, nil, time.Duration(0), err
		}
		if err := replaceBackupTableWithSource(ctx, stores, instance, database, spans); err != nil {
			slog.Debug("failed to replace backup table with source", log.BBError(err))
		}
		if !queryContext.SkipMasking && licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_DATA_MASKING, instance) == nil {
			masker := NewQueryResultMasker(stores)
			sensitivePredicateColumns, err = masker.ExtractSensitivePredicateColumns(ctx, spans, instance, user)
			if err != nil {
				return nil, nil, time.Duration(0), connect.NewError(connect.CodeInternal, errors.New(err.Error()))
			}
		}
	}
	for i, result := range results {
		if i < len(spans) && result.Error == "" {
			if spans[i].FunctionNotSupportedError != nil {
				return nil, nil, duration, connect.NewError(connect.CodeInternal, errors.Errorf("failed to mask data: %v", spans[i].FunctionNotSupportedError))
			}
			if spans[i].NotFoundError != nil {
				return nil, nil, duration, connect.NewError(connect.CodeInternal, errors.Errorf("failed to mask data: %v", spans[i].NotFoundError))
			}
		}
	}

	if maskingEnabled {
		slog.Debug("mask query results", slog.String("instance", instance.ResourceID), slog.String("database", database.DatabaseName))
		if dm := getDocumentMasker(instance.Metadata.GetEngine()); dm != nil {
			semanticTypeToMaskerMap, err := buildSemanticTypeToMaskerMap(ctx, stores)
			if err != nil {
				return nil, nil, duration, connect.NewError(connect.CodeInternal, errors.New(err.Error()))
			}
			if err := dm.maskResults(ctx, stores, database, spans, results, semanticTypeToMaskerMap, queryContext); err != nil {
				return nil, nil, duration, err
			}
		} else {
			masker := NewQueryResultMasker(stores)
			if err := masker.MaskResults(ctx, spans, results, instance, user); err != nil {
				return nil, nil, duration, connect.NewError(connect.CodeInternal, errors.New(err.Error()))
			}

			for i, result := range results {
				if i >= len(sensitivePredicateColumns) {
					continue
				}
				if len(sensitivePredicateColumns[i]) == 0 {
					continue
				}
				result.Error = getSensitivePredicateColumnErrorMessages(sensitivePredicateColumns[i])
				result.Rows = nil
				result.RowsCount = 0
			}
		}
	}
	return results, spans, duration, nil
}

func getSensitivePredicateColumnErrorMessages(sensitiveColumns []parserbase.ColumnResource) string {
	var buf bytes.Buffer
	_, _ = buf.WriteString("Using sensitive columns in WHERE clause is not allowed: ")
	for j, column := range sensitiveColumns {
		if j > 0 {
			_, _ = buf.WriteString(", ")
		}
		_, _ = buf.WriteString(column.String())
	}
	return buf.String()
}

// queryRetryStopOnError runs the query and stops on encountering errors.
func queryRetryStopOnError(
	ctx context.Context,
	stores *store.Store,
	user *store.UserMessage,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	driver db.Driver,
	conn *sql.Conn,
	statement string,
	queryContext db.QueryContext,
	licenseService *enterprise.LicenseService,
	optionalAccessCheck accessCheckFunc,
	schemaSyncer *schemasync.Syncer,
) ([]*v1pb.QueryResult, []*parserbase.QuerySpan, time.Duration, error) {
	if instance.Metadata.GetEngine() == storepb.Engine_MSSQL {
		statements, err := parserbase.SplitMultiSQL(instance.Metadata.GetEngine(), statement)
		if err != nil {
			return nil, nil, 0, err
		}
		return queryRetry(ctx, stores, user, instance, database, driver, conn, statements, statement, queryContext, licenseService, optionalAccessCheck, schemaSyncer)
	}

	statements, err := parserbase.SplitMultiSQL(instance.Metadata.GetEngine(), statement)
	if err != nil {
		return queryRetry(ctx, stores, user, instance, database, driver, conn, []parserbase.Statement{{Text: statement}}, statement, queryContext, licenseService, optionalAccessCheck, schemaSyncer)
	}

	var allResults []*v1pb.QueryResult
	var allSpans []*parserbase.QuerySpan
	var totalDuration time.Duration

	for _, stmt := range statements {
		if stmt.Empty {
			continue
		}

		results, spans, duration, err := queryRetry(ctx, stores, user, instance, database, driver, conn, []parserbase.Statement{stmt}, stmt.Text, queryContext, licenseService, optionalAccessCheck, schemaSyncer)
		totalDuration += duration

		if err != nil {
			allResults = append(allResults, &v1pb.QueryResult{
				Error:     err.Error(),
				Statement: stmt.Text,
			})
			allSpans = append(allSpans, nil)
			return allResults, allSpans, totalDuration, err
		}

		allResults = append(allResults, results...)
		allSpans = append(allSpans, spans...)

		for _, result := range results {
			if result.Error != "" {
				return allResults, allSpans, totalDuration, nil
			}
		}
	}

	return allResults, allSpans, totalDuration, nil
}

func executeWithTimeout(
	ctx context.Context,
	driver db.Driver,
	conn *sql.Conn,
	statement string,
	queryContext db.QueryContext,
) ([]*v1pb.QueryResult, time.Duration, error) {
	queryCtx := ctx
	var timeout time.Duration
	if queryContext.Timeout != nil {
		timeout = queryContext.Timeout.AsDuration()
		slog.Debug("create query context with timeout", slog.Duration("timeout", timeout))
		newCtx, cancelCtx := context.WithTimeout(ctx, timeout)
		defer cancelCtx()
		queryCtx = newCtx
	}

	start := time.Now()
	result, err := driver.QueryConn(queryCtx, conn, statement, queryContext)
	select {
	case <-queryCtx.Done():
		return nil, time.Since(start), errors.Errorf("timeout reached: %v", timeout)
	default:
	}
	sanitizeResults(result)
	return result, time.Since(start), err
}

func sanitizeResults(results []*v1pb.QueryResult) {
	for _, result := range results {
		for _, row := range result.GetRows() {
			for _, value := range row.GetValues() {
				if value != nil {
					if value, ok := value.Kind.(*v1pb.RowValue_StringValue); ok {
						value.StringValue = common.SanitizeUTF8String(value.StringValue)
					}
				}
			}
		}
	}
}

func validateQueryRequest(instance *store.InstanceMessage, statement string) error {
	ok, _, err := parserbase.ValidateSQLForEditor(instance.Metadata.GetEngine(), statement)
	if err != nil {
		if syntaxErr, ok := err.(*parserbase.SyntaxError); ok {
			err := connect.NewError(connect.CodeInvalidArgument, syntaxErr)
			if detail, detailErr := connect.NewErrorDetail(&v1pb.PlanCheckRun_Result{
				Code:    int32(code.StatementSyntaxError),
				Content: syntaxErr.Message,
				Title:   "Syntax error",
				Status:  v1pb.Advice_ERROR,
				Report: &v1pb.PlanCheckRun_Result_SqlReviewReport_{
					SqlReviewReport: &v1pb.PlanCheckRun_Result_SqlReviewReport{
						StartPosition: convertToPosition(syntaxErr.Position),
					},
				},
			}); detailErr == nil {
				err.AddDetail(detail)
			}
			return err
		}
		return err
	}
	if !ok {
		return &queryError{
			err:         connect.NewError(connect.CodeInvalidArgument, errors.New("Support read-only command statements only")),
			commandType: v1pb.QueryResult_CommandError_NON_READ_ONLY,
		}
	}
	return nil
}

var envConditionRe = regexp.MustCompile(`resource\.environment_id\s+in\s+\[[^\]]*\]`)

// stripEnvironmentCondition removes the "resource.environment_id in [...]" clause
// from a CEL expression string.
func stripEnvironmentCondition(expression string) string {
	result := envConditionRe.ReplaceAllString(expression, "")
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "&&")
	result = strings.TrimSuffix(result, "&&")
	return strings.TrimSpace(result)
}

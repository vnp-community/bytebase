package v1

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/alexmullins/zip"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/export"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/db"
	parserbase "github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
)

// Export exports the SQL query result.
func (s *SQLService) Export(ctx context.Context, req *connect.Request[v1pb.ExportRequest]) (*connect.Response[v1pb.ExportResponse], error) {
	request := req.Msg
	// Prehandle export from issue.
	if strings.HasPrefix(request.Name, common.ProjectNamePrefix) {
		response, err := s.doExportFromIssue(ctx, request.Name)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(response), nil
	}

	// Prepare related message.
	user, instance, database, err := s.prepareRelatedMessage(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	statement := request.Statement
	// In Redshift datashare, Rewrite query used for parser.
	if database.Metadata.GetDatashare() {
		statement = strings.ReplaceAll(statement, fmt.Sprintf("%s.", database.DatabaseName), "")
	}

	// Validate the request.
	// New query ACL experience.
	if instance.Metadata.GetEngine() != storepb.Engine_MYSQL {
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
	bytes, duration, exportErr := doExport(ctx, s.store, s.dbFactory, s.licenseService, request, user, instance, database, s.accessCheck, s.schemaSyncer, dataSource)

	s.createQueryHistory(database, store.QueryHistoryTypeExport, statement, user.Email, duration, exportErr)

	if exportErr != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New(exportErr.Error()))
	}

	return connect.NewResponse(&v1pb.ExportResponse{
		Content: bytes,
	}), nil
}

func (s *SQLService) doExportFromIssue(ctx context.Context, requestName string) (*v1pb.ExportResponse, error) {
	// Try to parse as rollout name first (more specific), then fallback to stage name
	var planID int64
	var projectID string
	var err error
	projectID, planID, err = common.GetProjectIDPlanIDFromRolloutName(requestName)
	if err != nil {
		// If rollout parsing fails, try parsing as stage name
		projectID, planID, _, err = common.GetProjectIDPlanIDMaybeStageID(requestName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to parse request name as rollout or stage: %v", err))
		}
	}

	plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		ProjectID: projectID,
		UID:       &planID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get rollout: %v", err))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("rollout %d not found in project %s", planID, projectID))
	}

	tasks, err := s.store.ListTasks(ctx, &store.TaskFind{Workspace: common.GetWorkspaceIDFromContext(ctx), ProjectID: projectID, PlanID: &plan.UID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get tasks: %v", err))
	}
	if len(tasks) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("rollout %d has no task", plan.UID))
	}

	// Get password from the plan spec
	// For export data plans, there is always exactly one spec
	var passwordStr string
	if len(plan.Config.Specs) > 0 {
		if exportConfig := plan.Config.Specs[0].GetExportDataConfig(); exportConfig != nil && exportConfig.Password != nil {
			passwordStr = *exportConfig.Password
		}
	}

	pendingEncrypts := []*encryptContent{}

	for _, task := range tasks {
		// Skip tasks that are marked as skipped (they don't have archives)
		if task.Payload.GetSkipped() {
			continue
		}

		taskRuns, err := s.store.ListTaskRuns(ctx, &store.FindTaskRunMessage{
			Workspace: common.GetWorkspaceIDFromContext(ctx),
			ProjectID: projectID,
			TaskUID:   &task.ID,
			Status:    &[]storepb.TaskRun_Status{storepb.TaskRun_DONE},
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get task run: %v", err))
		}
		if len(taskRuns) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("rollout %v has no task run", requestName))
		}
		taskRun := taskRuns[0]
		exportArchiveID := taskRun.ResultProto.ExportArchiveId
		if exportArchiveID == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("issue %v has no export archive", requestName))
		}
		exportArchive, err := s.store.GetExportArchive(ctx, common.GetWorkspaceIDFromContext(ctx), exportArchiveID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get export archive: %v", err))
		}
		if exportArchive == nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("export not found or expired, please request a new export"))
		}

		// The exportArchive.Bytes should be a zip without password. We will read it and append all files into the pendingEncrypts,
		// then create a new file zip for them.
		zipReader, err := zip.NewReader(bytes.NewReader(exportArchive.Bytes), int64(len(exportArchive.Bytes)))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to read export archive: %v", err))
		}

		for _, file := range zipReader.File {
			rc, err := file.Open()
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to open file %s in archive: %v", file.Name, err))
			}

			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to read file %s: %v", file.Name, err))
			}

			pendingEncrypts = append(pendingEncrypts, &encryptContent{
				Content: content,
				Name:    file.Name,
			})
		}
	}

	encryptedBytes, err := doEncrypt(pendingEncrypts, passwordStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to encrypt data: %v", err))
	}

	return &v1pb.ExportResponse{
		Content: encryptedBytes,
	}, nil
}

// doExport performs SQL Editor exports with masking applied.
// This is used for ad-hoc exports where users have the EXPORTER role.
// For approved DATABASE_EXPORT tasks, see data_export_executor.go which exports without masking.
func doExport(
	ctx context.Context,
	stores *store.Store,
	dbFactory *dbfactory.DBFactory,
	licenseService *enterprise.LicenseService,
	request *v1pb.ExportRequest,
	user *store.UserMessage,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	optionalAccessCheck accessCheckFunc,
	schemaSyncer *schemasync.Syncer,
	dataSource *storepb.DataSource,
) ([]byte, time.Duration, error) {
	if dataSource == nil {
		return nil, 0, connect.NewError(connect.CodeNotFound, errors.Errorf("cannot found valid data source"))
	}
	driver, err := dbFactory.GetDataSourceDriver(ctx, instance, dataSource, db.ConnectionContext{
		DatabaseName: database.DatabaseName,
		DataShare:    database.Metadata.GetDatashare(),
		ReadOnly:     true,
	})
	if err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database driver: %v", err))
	}
	defer driver.Close(ctx)

	sqlDB := driver.GetDB()
	var conn *sql.Conn
	if sqlDB != nil {
		conn, err = sqlDB.Conn(ctx)
		if err != nil {
			return nil, 0, err
		}
		defer conn.Close()
	}
	queryRestriction := getEffectiveQueryDataPolicy(
		ctx,
		stores,
		licenseService,
		request.Limit,
		database.ProjectID,
	)
	queryContext := db.QueryContext{
		Limit:                int(queryRestriction.MaximumResultRows),
		OperatorEmail:        user.Email,
		MaximumSQLResultSize: queryRestriction.MaximumResultSize,
	}
	if queryRestriction.MaxQueryTimeoutInSeconds > 0 {
		queryContext.Timeout = &durationpb.Duration{Seconds: queryRestriction.MaxQueryTimeoutInSeconds}
	}
	if request.Schema != nil {
		queryContext.Schema = *request.Schema
	}

	// Split the statement for span analysis
	statements, err := parserbase.SplitMultiSQL(instance.Metadata.GetEngine(), request.Statement)
	if err != nil {
		// Fall back to single statement for engines without splitter support
		statements = []parserbase.Statement{{Text: request.Statement}}
	}

	results, spans, duration, queryErr := queryRetry(
		ctx,
		stores,
		user,
		instance,
		database,
		driver,
		conn,
		statements,
		request.Statement,
		queryContext,
		licenseService,
		optionalAccessCheck,
		schemaSyncer,
	)
	if queryErr != nil {
		return nil, duration, queryErr
	}

	if licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_DATA_MASKING, instance) == nil {
		masker := NewQueryResultMasker(stores)
		if err := masker.MaskResults(ctx, spans, results, instance, user); err != nil {
			return nil, duration, err
		}
	}

	var buf bytes.Buffer
	zipw := zip.NewWriter(&buf)

	exportCount := 0
	for i, result := range results {
		if result.GetError() != "" {
			return nil, duration, errors.Errorf("failed to exec the SQL with error: %v", result.GetError())
		}

		if err := exportResultToZip(ctx, zipw, stores, instance, database, result, request, i+1); err != nil {
			return nil, duration, errors.Errorf("failed to export result to zip with error: %v", result.GetError())
		}

		exportCount++
		// Help GC by clearing the result data we've already processed
		result.Rows = nil
	}

	if exportCount == 0 {
		return nil, duration, errors.Errorf("empty export data for database %s", database.DatabaseName)
	}

	if err := zipw.Close(); err != nil {
		return nil, duration, errors.Wrap(err, "failed to close zip writer")
	}

	return buf.Bytes(), duration, nil
}

// exportResultToZip exports a single query result to the ZIP archive.
// It writes both the SQL statement and the formatted result data.
func exportResultToZip(
	ctx context.Context,
	zipw *zip.Writer,
	stores *store.Store,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	result *v1pb.QueryResult,
	request *v1pb.ExportRequest,
	statementNumber int,
) error {
	baseFilename := fmt.Sprintf("%s/%s/statement-%d", database.InstanceID, database.DatabaseName, statementNumber)

	// Write statement file
	statementFilename := fmt.Sprintf("%s.sql", baseFilename)
	if err := export.WriteZipEntry(zipw, statementFilename, []byte(result.Statement), request.GetPassword()); err != nil {
		return errors.Wrap(err, "failed to write statement")
	}

	// Write result file by streaming directly to ZIP
	resultExt := strings.ToLower(request.Format.String())
	resultFilename := fmt.Sprintf("%s.result.%s", baseFilename, resultExt)
	if err := formatExportToZip(ctx, zipw, resultFilename, stores, instance, database, result, request); err != nil {
		return errors.Wrap(err, "failed to write formatted result")
	}

	return nil
}

// formatExportToZip formats query results and writes them directly to a ZIP entry.
// This function streams the formatted data to minimize memory usage.
func formatExportToZip(
	ctx context.Context,
	zipw *zip.Writer,
	filename string,
	stores *store.Store,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	result *v1pb.QueryResult,
	request *v1pb.ExportRequest,
) error {
	writer, err := export.CreateZipWriter(zipw, filename, request.GetPassword())
	if err != nil {
		return err
	}

	switch request.Format {
	case v1pb.ExportFormat_CSV:
		return export.CSVToWriter(writer, result)
	case v1pb.ExportFormat_JSON:
		return export.JSONToWriter(writer, result)
	case v1pb.ExportFormat_SQL:
		return exportSQLWithContext(ctx, writer, stores, instance, database, result, request)
	case v1pb.ExportFormat_XLSX:
		return export.XLSXToWriter(writer, result)
	default:
		return errors.Errorf("unsupported export format: %s", request.Format.String())
	}
}

// exportSQLWithContext exports SQL INSERT statements with proper context.
func exportSQLWithContext(
	ctx context.Context,
	w io.Writer,
	stores *store.Store,
	instance *store.InstanceMessage,
	database *store.DatabaseMessage,
	result *v1pb.QueryResult,
	request *v1pb.ExportRequest,
) error {
	resourceList, err := export.GetResources(
		ctx,
		stores,
		instance.Metadata.GetEngine(),
		database.DatabaseName,
		request.Statement,
		instance,
		BuildGetDatabaseMetadataFunc(stores),
		BuildListDatabaseNamesFunc(stores),
		BuildGetLinkedDatabaseMetadataFunc(stores, instance.Metadata.GetEngine()),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to extract resource list")
	}
	statementPrefix, err := export.SQLStatementPrefix(instance.Metadata.GetEngine(), resourceList, result.ColumnNames)
	if err != nil {
		return err
	}
	return export.SQLToWriter(w, instance.Metadata.GetEngine(), statementPrefix, result)
}

type encryptContent struct {
	Name    string
	Content []byte
}

func doEncrypt(exports []*encryptContent, password string) ([]byte, error) {
	var b bytes.Buffer
	fzip := io.Writer(&b)

	zipw := zip.NewWriter(fzip)
	defer zipw.Close()

	for _, exportContent := range exports {
		if err := export.WriteZipEntry(zipw, exportContent.Name, exportContent.Content, password); err != nil {
			return nil, err
		}
	}

	if err := zipw.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close zip writer")
	}

	return b.Bytes(), nil
}
